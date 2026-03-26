#!/usr/bin/env python3
"""
Demo: PDF-Guided Wheel Strategy via Backtester Playground

Same flow as demo_wheel_strategy.py, but uses empirical probability
distributions (PDFs) to guide put strike selection and Kelly-based sizing.

Supports optional periodic PDF retraining during the simulation: the model
is rebuilt from expanding (or rolling) historical data at a configurable
interval (weekly or monthly).

Steps:
  1. Build (or load) the PDF from historical data
  2. Create a simulator playground with 15-min + daily repos
  3. Run the PDF-guided wheel strategy loop
  4. Print metrics

Usage:
    # Static PDF (original behavior):
    python demo_pdf_wheel_strategy.py --symbol AAPL --start 2025-01-01 \
        --end 2025-06-30 --pdf-path aapl_pdf.json

    # Weekly retraining (training window defaults to 1 year before --start):
    python demo_pdf_wheel_strategy.py --symbol AAPL --start 2025-01-01 \
        --end 2025-06-30 --retrain-interval weekly

    # Weekly retraining with explicit training start + rolling window:
    python demo_pdf_wheel_strategy.py --symbol AAPL --start 2025-01-01 \
        --end 2025-06-30 --retrain-interval weekly \
        --training-start 2023-06-01 --rolling-window 365

Requires the Go trading server to be running on the specified twirp host.
"""

import argparse
import os
import sys
from datetime import datetime, timedelta
from typing import Optional
from zoneinfo import ZoneInfo

from loguru import logger

from engine.client import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    Repository,
    RepositorySource,
)
from tools.build_pdf_from_polygon import bar_to_dict
from lib.pdf_builder import PDFBuilder
from lib.pdf_types import PDFDocument
from engine.trading_engine import run_strategy
from strategies.pdf_wheel import PDFWheelStrategy
from strategies.covered_call import generate_signal_stats


# ------------------------------------------------------------------ #
# Retrain helpers
# ------------------------------------------------------------------ #

_NY = ZoneInfo("America/New_York")

# Wheel strategy constants
_LTF_PERIOD = 900      # 15-min in seconds
_HTF_PERIOD = 86400    # daily in seconds
_HORIZONS = {"1h": 4, "4h": 16, "1d": 26, "2d": 52}
_INDICATORS = [
    "supertrend", "stochrsi", "atr", "doji", "hammer",
    "sma_50", "sma_100", "sma_200",
    "stochrsi_cross_above_20", "stochrsi_cross_below_80",
]


def _fetch_all_candles(
    symbol: str,
    start_date: str,
    end_date: str,
    twirp_host: str,
) -> tuple:
    """
    Create a data-only playground and fetch all LTF + daily candles.

    Returns (ltf_bars, daily_bars) as lists of dicts.
    """
    repos = [
        Repository(
            symbol=symbol,
            timespan_multiplier=15,
            timespan_unit="minute",
            indicators=_INDICATORS,
            history_in_days=365,
        ),
        Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit="day",
            indicators=_INDICATORS,
            history_in_days=365,
        ),
    ]

    req = CreatePolygonPlaygroundRequest(
        balance=100_000,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )

    data_pg = BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=twirp_host,
    )

    ts_start = datetime.fromisoformat(start_date).replace(tzinfo=_NY)
    ts_end = datetime.fromisoformat(end_date).replace(tzinfo=_NY)

    ltf_bars_pb = data_pg.fetch_candles_v3(symbol, _LTF_PERIOD, ts_start, ts_end)
    daily_bars_pb = data_pg.fetch_candles_v3(symbol, _HTF_PERIOD, ts_start, ts_end)

    ltf_bars = [bar_to_dict(b) for b in ltf_bars_pb]
    daily_bars = [bar_to_dict(b) for b in daily_bars_pb]

    return ltf_bars, daily_bars


def _parse_bar_dt(bar: dict) -> datetime:
    """Parse a bar's datetime string into a timezone-aware datetime."""
    dt_val = bar["datetime"]
    if isinstance(dt_val, str):
        # Python 3.10 fromisoformat doesn't handle 'Z' suffix
        dt_val = datetime.fromisoformat(dt_val.replace("Z", "+00:00"))
    if dt_val.tzinfo is None:
        dt_val = dt_val.replace(tzinfo=_NY)
    return dt_val


def _slice_bars(bars: list, cutoff: datetime) -> list:
    """Return bars with datetime <= cutoff."""
    return [b for b in bars if _parse_bar_dt(b) <= cutoff]


def _slice_bars_after(bars: list, start: datetime) -> list:
    """Return bars with datetime >= start."""
    return [b for b in bars if _parse_bar_dt(b) >= start]


def _build_pdf(
    symbol: str,
    ltf_bars: list,
    daily_bars: list,
    return_model: str,
) -> PDFDocument:
    """Build a PDF from candle bar dicts."""
    builder = PDFBuilder(
        symbol=symbol,
        ltf_bars=ltf_bars,
        daily_bars=daily_bars,
        ltf_period_seconds=_LTF_PERIOD,
        horizons=_HORIZONS,
        htf_timeframe="daily",
        return_model=return_model,
    )
    return builder.build()


def _is_retrain_boundary(
    candle,
    interval: str,
    last_retrain_date: Optional[datetime],
) -> bool:
    """Check if a daily candle represents a retrain boundary."""
    if not hasattr(candle, "period") or candle.period != _HTF_PERIOD:
        return False

    bar_dt = candle.bar.datetime
    if isinstance(bar_dt, str):
        bar_dt = datetime.fromisoformat(bar_dt.replace("Z", "+00:00"))

    if interval == "weekly":
        is_boundary = bar_dt.weekday() == 4  # Friday
    elif interval == "monthly":
        # Last trading day approximation: check if next day is in a new month
        next_day = bar_dt + timedelta(days=1)
        is_boundary = next_day.month != bar_dt.month
    else:
        return False

    if last_retrain_date and bar_dt.date() <= last_retrain_date.date():
        return False

    return is_boundary


def make_retrain_callback(
    symbol: str,
    all_ltf_bars: list,
    all_daily_bars: list,
    training_start: datetime,
    return_model: str,
    interval: str,
    rolling_window_days: Optional[int],
):
    """
    Create a callback for ``run_pdf_wheel_strategy(on_tick=...)``.

    The callback checks each tick batch for a daily Friday (or month-end)
    candle. When found, it rebuilds the PDF from the expanding (or rolling)
    training window and swaps ``strategy.pdf``.
    """
    state = {"last_retrain_date": None, "retrain_count": 0}

    def _callback(strategy, tick_deltas):
        for td in tick_deltas:
            new_candles = td.new_candles if hasattr(td, "new_candles") else []
            for c in new_candles:
                if not _is_retrain_boundary(c, interval, state["last_retrain_date"]):
                    continue

                bar_dt = c.bar.datetime
                if isinstance(bar_dt, str):
                    bar_dt = datetime.fromisoformat(bar_dt.replace("Z", "+00:00"))
                if bar_dt.tzinfo is None:
                    bar_dt = bar_dt.replace(tzinfo=_NY)

                # Determine training window
                if rolling_window_days is not None:
                    window_start = bar_dt - timedelta(days=rolling_window_days)
                    ltf_slice = _slice_bars_after(
                        _slice_bars(all_ltf_bars, bar_dt), window_start,
                    )
                    daily_slice = _slice_bars_after(
                        _slice_bars(all_daily_bars, bar_dt), window_start,
                    )
                    window_label = f"rolling {rolling_window_days}d"
                else:
                    ltf_slice = _slice_bars(all_ltf_bars, bar_dt)
                    daily_slice = _slice_bars(all_daily_bars, bar_dt)
                    window_label = f"expanding from {training_start.date()}"

                if len(ltf_slice) < 100 or len(daily_slice) < 5:
                    logger.warning(
                        f"  Retrain skipped: insufficient bars"
                        f" (ltf={len(ltf_slice)}, daily={len(daily_slice)})"
                    )
                    continue

                prev_signal_count = len(strategy.pdf.signals)
                new_pdf = _build_pdf(symbol, ltf_slice, daily_slice, return_model)
                strategy.pdf = new_pdf
                state["retrain_count"] += 1
                state["last_retrain_date"] = bar_dt

                logger.info("=" * 60)
                logger.info(
                    f"PDF RETRAINED #{state['retrain_count']}"
                    f" @ {bar_dt.strftime('%Y-%m-%d')} ({window_label})"
                )
                logger.info(
                    f"  LTF bars: {len(ltf_slice):,}"
                    f"  |  Daily bars: {len(daily_slice):,}"
                )
                logger.info(
                    f"  Signals: {prev_signal_count}"
                    f" -> {len(new_pdf.signals)}"
                    f"  |  Sufficient: {len(new_pdf.get_sufficient_signals())}"
                )
                logger.info("=" * 60)

    return _callback


# ------------------------------------------------------------------ #
# Main
# ------------------------------------------------------------------ #

def main():
    parser = argparse.ArgumentParser(
        description="Demo PDF-guided wheel strategy in the backtester playground",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Retrain mode:\n"
            "  When --retrain-interval is set, the PDF is rebuilt periodically\n"
            "  during the simulation using an expanding (default) or rolling\n"
            "  window of historical data. No --pdf-path is needed.\n"
            "\n"
            "Examples:\n"
            "  # Static PDF (original):\n"
            "  python demo_pdf_wheel_strategy.py --pdf-path aapl_pdf.json\n"
            "\n"
            "  # Weekly retrain (1-year default training window):\n"
            "  python demo_pdf_wheel_strategy.py --retrain-interval weekly\n"
            "\n"
            "  # Weekly retrain with rolling 6-month window:\n"
            "  python demo_pdf_wheel_strategy.py --retrain-interval weekly \\\n"
            "      --rolling-window 180\n"
        ),
    )
    parser.add_argument(
        "--symbol", type=str, default="AAPL",
        help="Underlying stock symbol (default: AAPL)",
    )
    parser.add_argument(
        "--start", type=str, default="2025-01-01",
        help="Simulation start date YYYY-MM-DD",
    )
    parser.add_argument(
        "--end", type=str, default="2025-06-30",
        help="Simulation end date YYYY-MM-DD",
    )
    parser.add_argument(
        "--balance", type=float, default=100_000,
        help="Starting account balance (default: 100000)",
    )
    parser.add_argument(
        "--twirp-host", type=str, default="http://127.0.0.1:5051",
        help="Twirp server URL",
    )
    parser.add_argument(
        "--pdf-path", type=str, default="",
        help="Path to pre-built PDF JSON (used when --retrain-interval is not set)",
    )
    parser.add_argument(
        "--kelly-fraction", type=float, default=0.5,
        help="Kelly fraction multiplier (default: 0.5 = half-Kelly)",
    )

    # Retrain flags
    parser.add_argument(
        "--retrain-interval", type=str, default=None,
        choices=["weekly", "monthly"],
        help="Rebuild the PDF at this interval during simulation",
    )
    parser.add_argument(
        "--training-start", type=str, default=None,
        help="Start of training data window YYYY-MM-DD (default: 1 year before --start)",
    )
    parser.add_argument(
        "--rolling-window", type=int, default=None,
        help="Use a rolling window of N days instead of expanding from training-start",
    )
    parser.add_argument(
        "--return-model", type=str, default="empirical",
        choices=["empirical", "bayesian_nig"],
        help="Statistical model for PDF (default: empirical)",
    )

    args = parser.parse_args()

    symbol = args.symbol
    start_date = args.start
    end_date = args.end
    balance = args.balance
    twirp_host = args.twirp_host
    pdf_path = args.pdf_path
    kelly_frac = args.kelly_fraction
    retrain_interval = args.retrain_interval

    # Configure logger
    logger.remove()
    logger.add(
        sys.stderr,
        level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    # ------------------------------------------------------------------
    # Header
    # ------------------------------------------------------------------
    logger.info("=" * 60)
    logger.info("PDF-Guided Wheel Strategy Demo")
    logger.info("=" * 60)
    logger.info(f"Symbol:         {symbol}")
    logger.info(f"Period:         {start_date} -> {end_date}")
    logger.info(f"Balance:        ${balance:,.2f}")
    logger.info(f"Kelly fraction: {kelly_frac}")
    logger.info(f"Server:         {twirp_host}")
    if retrain_interval:
        training_start_str = args.training_start or "(1 year before --start)"
        logger.info(f"Retrain:        {retrain_interval}")
        logger.info(f"Training start: {training_start_str}")
        logger.info(f"Return model:   {args.return_model}")
        if args.rolling_window:
            logger.info(f"Rolling window: {args.rolling_window} days")
        else:
            logger.info(f"Window:         expanding")
    else:
        logger.info(f"PDF path:       {pdf_path or '(none)'}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Load or build the initial PDF
    # ------------------------------------------------------------------
    on_tick_callback = None

    if retrain_interval:
        # Compute training start date
        sim_start_dt = datetime.fromisoformat(start_date).replace(tzinfo=_NY)
        if args.training_start:
            training_start_dt = datetime.fromisoformat(args.training_start).replace(tzinfo=_NY)
        else:
            training_start_dt = sim_start_dt - timedelta(days=365)

        training_start_str = training_start_dt.strftime("%Y-%m-%d")

        logger.info(
            f"Fetching training data: {training_start_str} -> {end_date} ..."
        )
        all_ltf_bars, all_daily_bars = _fetch_all_candles(
            symbol, training_start_str, end_date, twirp_host,
        )
        logger.info(
            f"  Training data: {len(all_ltf_bars):,} LTF bars,"
            f" {len(all_daily_bars):,} daily bars"
        )

        # Build initial PDF from [training_start, sim_start]
        ltf_init = _slice_bars(all_ltf_bars, sim_start_dt)
        daily_init = _slice_bars(all_daily_bars, sim_start_dt)

        logger.info(
            f"Building initial PDF from {training_start_str}"
            f" -> {start_date}"
            f" ({len(ltf_init):,} LTF, {len(daily_init):,} daily bars) ..."
        )
        pdf = _build_pdf(symbol, ltf_init, daily_init, args.return_model)
        logger.info(
            f"Initial PDF: {len(pdf.signals)} signals,"
            f" {len(pdf.get_sufficient_signals())} sufficient"
        )

        # Create retrain callback
        on_tick_callback = make_retrain_callback(
            symbol=symbol,
            all_ltf_bars=all_ltf_bars,
            all_daily_bars=all_daily_bars,
            training_start=training_start_dt,
            return_model=args.return_model,
            interval=retrain_interval,
            rolling_window_days=args.rolling_window,
        )

    elif pdf_path and os.path.exists(pdf_path):
        logger.info(f"Loading PDF from {pdf_path} ...")
        pdf = PDFDocument.load(pdf_path)
        logger.info(
            f"PDF loaded: {len(pdf.signals)} signals, "
            f"scan range {pdf.scan_start} to {pdf.scan_end}"
        )
    else:
        logger.error(
            "No PDF provided. Either pass --pdf-path or use --retrain-interval"
            " to build the PDF dynamically."
        )
        sys.exit(1)

    # ------------------------------------------------------------------
    # 2. Build repositories (15-min + 1-day candles with indicators)
    # ------------------------------------------------------------------
    repos = PDFWheelStrategy.get_repositories(symbol)

    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )

    # ------------------------------------------------------------------
    # 3. Create a simulator playground
    # ------------------------------------------------------------------
    logger.info("Creating simulator playground ...")
    playground = BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=twirp_host,
    )
    logger.info(f"Playground created — id: {playground.id}")
    logger.info(f"Initial timestamp:  {playground.timestamp.isoformat()}")
    logger.info(f"Account balance:    ${playground.account.balance:,.2f}")

    # ------------------------------------------------------------------
    # 4. Run the PDF-guided wheel strategy
    # ------------------------------------------------------------------
    logger.info("Running PDF-guided wheel strategy ...")
    try:
        generate_signal_stats(playground, symbol)
        playground.stats.generate_model()
        strategy = PDFWheelStrategy(
            playground, symbol, logger,
            pdf=pdf,
            peak_equity=balance,
            kelly_fraction_mult=kelly_frac,
            max_open_count=5,
        )
        run_strategy(strategy, playground, logger, on_tick=on_tick_callback)
    except Exception:
        logger.exception("Strategy encountered an error")
        sys.exit(1)

    # ------------------------------------------------------------------
    # 5. Print results
    # ------------------------------------------------------------------
    logger.info("=" * 60)
    logger.info("RESULTS")
    logger.info("=" * 60)
    logger.info(f"Final timestamp:    {playground.timestamp.isoformat()}")
    logger.info(f"Final balance:      ${playground.account.balance:,.2f}")
    logger.info(f"Final equity:       ${playground.account.equity:,.2f}")

    realized_pnl = playground.get_realized_profit()
    logger.info(f"Realized P&L:       ${realized_pnl:,.2f}")
    logger.info(f"Return:             {realized_pnl / balance * 100:.2f}%")

    logger.info("-" * 60)
    logger.info("Open positions:")
    if playground.account.positions:
        for sym, pos in playground.account.positions.items():
            logger.info(
                f"  {sym:30s}  qty={pos.quantity:>6.0f}"
                f"  cost_basis=${pos.cost_basis:>10.2f}  P&L=${pos.pl:>10.2f}"
            )
    else:
        logger.info("  (none)")

    logger.info("-" * 60)
    logger.info(f"Total trades placed: {len(playground.trade_timestamps)}")
    logger.info("=" * 60)

    logger.info(f"Playground id: {playground.id}")


if __name__ == "__main__":
    main()
