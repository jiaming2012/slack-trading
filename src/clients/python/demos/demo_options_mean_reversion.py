#!/usr/bin/env python3
"""
Demo: Options Mean-Reversion Strategy via Backtester Playground

Creates a simulator playground with 5-min LTF + 1-hour HTF candles, then
runs the options mean-reversion strategy that buys calls on dips (bullish)
and puts on rallies (bearish).

Supports optional periodic PDF retraining during the simulation.

Usage:
    # Static PDF:
    python demo_options_mean_reversion.py \
        --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
        --balance 100000 --pdf-path aapl_5m_1h_pdf.json

    # Weekly retraining:
    python demo_options_mean_reversion.py \
        --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
        --balance 100000 --retrain-interval weekly --model bayesian_nig

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
from engine.trading_engine import run_strategy
from strategies.options_mean_reversion import OptionsMeanReversionStrategy
from lib.pdf_builder import PDFBuilder
from lib.pdf_types import PDFDocument


# ------------------------------------------------------------------ #
# Retrain helpers (shared with demo_mean_reversion.py)
# ------------------------------------------------------------------ #

_NY = ZoneInfo("America/New_York")

_LTF_PERIOD = 300       # 5-min in seconds
_HTF_PERIOD = 3600      # 1-hour in seconds
_HORIZONS = {"1h": 12, "4h": 48, "1d": 78, "1w": 390, "2w": 780}
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
    """Create a data-only playground and fetch all LTF + HTF candles."""
    repos = [
        Repository(
            symbol=symbol,
            timespan_multiplier=5,
            timespan_unit="minute",
            indicators=_INDICATORS,
            history_in_days=365,
        ),
        Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit="hour",
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
    htf_bars_pb = data_pg.fetch_candles_v3(symbol, _HTF_PERIOD, ts_start, ts_end)

    ltf_bars = [bar_to_dict(b) for b in ltf_bars_pb]
    htf_bars = [bar_to_dict(b) for b in htf_bars_pb]

    return ltf_bars, htf_bars


def _parse_bar_dt(bar: dict) -> datetime:
    """Parse a bar's datetime string into a timezone-aware datetime."""
    dt_val = bar["datetime"]
    if isinstance(dt_val, str):
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
    htf_bars: list,
    return_model: str,
) -> PDFDocument:
    """Build a PDF from candle bar dicts (5-min LTF + 1-hour HTF)."""
    builder = PDFBuilder(
        symbol=symbol,
        ltf_bars=ltf_bars,
        daily_bars=htf_bars,
        ltf_period_seconds=_LTF_PERIOD,
        horizons=_HORIZONS,
        htf_timeframe="ltf",
        return_model=return_model,
    )
    return builder.build()


def _is_retrain_boundary(
    candle,
    interval: str,
    last_retrain_date: Optional[datetime],
) -> bool:
    """Check if an HTF candle represents a retrain boundary."""
    if not hasattr(candle, "period") or candle.period != _HTF_PERIOD:
        return False

    bar_dt = candle.bar.datetime
    if isinstance(bar_dt, str):
        bar_dt = datetime.fromisoformat(bar_dt.replace("Z", "+00:00"))
    if bar_dt.tzinfo is None:
        bar_dt = bar_dt.replace(tzinfo=_NY)

    if interval == "weekly":
        is_boundary = bar_dt.weekday() == 4 and bar_dt.hour >= 15
    elif interval == "monthly":
        next_day = bar_dt + timedelta(days=1)
        is_boundary = next_day.month != bar_dt.month and bar_dt.hour >= 15
    else:
        return False

    if last_retrain_date and bar_dt.date() <= last_retrain_date.date():
        return False

    return is_boundary


def make_retrain_callback(
    symbol: str,
    all_ltf_bars: list,
    all_htf_bars: list,
    training_start: datetime,
    return_model: str,
    interval: str,
    rolling_window_days: Optional[int],
):
    """Create a callback for ``run_options_mean_reversion(on_tick=...)``."""
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

                if rolling_window_days is not None:
                    window_start = bar_dt - timedelta(days=rolling_window_days)
                    ltf_slice = _slice_bars_after(
                        _slice_bars(all_ltf_bars, bar_dt), window_start,
                    )
                    htf_slice = _slice_bars_after(
                        _slice_bars(all_htf_bars, bar_dt), window_start,
                    )
                    window_label = f"rolling {rolling_window_days}d"
                else:
                    ltf_slice = _slice_bars(all_ltf_bars, bar_dt)
                    htf_slice = _slice_bars(all_htf_bars, bar_dt)
                    window_label = f"expanding from {training_start.date()}"

                if len(ltf_slice) < 100 or len(htf_slice) < 5:
                    logger.warning(
                        f"  Retrain skipped: insufficient bars"
                        f" (ltf={len(ltf_slice)}, htf={len(htf_slice)})"
                    )
                    continue

                prev_signal_count = len(strategy.pdf.signals)
                new_pdf = _build_pdf(symbol, ltf_slice, htf_slice, return_model)
                strategy.pdf = new_pdf
                state["retrain_count"] += 1
                state["last_retrain_date"] = bar_dt

                logger.info("=" * 60)
                logger.info(
                    f"PDF RETRAINED #{state['retrain_count']}"
                    f" @ {bar_dt.strftime('%Y-%m-%d %H:%M')} ({window_label})"
                )
                logger.info(
                    f"  LTF bars: {len(ltf_slice):,}"
                    f"  |  HTF bars: {len(htf_slice):,}"
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
        description="Demo options mean-reversion strategy in the backtester playground",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Retrain mode:\n"
            "  When --retrain-interval is set, the PDF is rebuilt periodically\n"
            "  during the simulation. No --pdf-path is needed.\n"
            "\n"
            "Examples:\n"
            "  # Static PDF:\n"
            "  python demo_options_mean_reversion.py --pdf-path aapl_5m_1h_pdf.json\n"
            "\n"
            "  # Weekly retrain:\n"
            "  python demo_options_mean_reversion.py --retrain-interval weekly\n"
        ),
    )
    parser.add_argument("--symbol", type=str, default="AAPL", help="Stock symbol (default: AAPL)")
    parser.add_argument("--start", type=str, default="2025-06-01", help="Simulation start date YYYY-MM-DD")
    parser.add_argument("--end", type=str, default="2026-02-28", help="Simulation end date YYYY-MM-DD")
    parser.add_argument("--balance", type=float, default=100_000, help="Starting account balance (default: 100000)")
    parser.add_argument("--pdf-path", type=str, default="", help="Path to pre-built PDF JSON")
    parser.add_argument("--max-premium-pct", type=float, default=0.02, help="Max premium per group as %% of equity (default: 0.02)")
    parser.add_argument("--stop-percentile", type=float, default=0.95, help="Stop percentile for deviation plan (default: 0.95)")
    parser.add_argument("--model", type=str, default="bayesian_nig", choices=["empirical", "bayesian_nig"], help="Return model (default: bayesian_nig)")
    parser.add_argument("--total-contracts", type=int, default=10, help="Total contracts per group (default: 10)")
    parser.add_argument("--htf-horizon", type=str, default="1h", help="PDF horizon key (default: 1h)")
    parser.add_argument("--target-dte", type=int, default=14, help="Target days to expiration (default: 14)")
    parser.add_argument("--min-dte", type=int, default=5, help="Minimum DTE (default: 5)")
    parser.add_argument("--max-dte", type=int, default=30, help="Maximum DTE (default: 30)")
    parser.add_argument("--profit-target", type=float, default=0.50, help="Profit target as fraction of premium (default: 0.50)")
    parser.add_argument("--time-decay-exit", type=int, default=3, help="Force exit when DTE drops below this (default: 3)")
    parser.add_argument(
        "--strike-strategy", type=str, default="model",
        choices=["model", "atm", "otm"],
        help="Strike selection: 'model' (adaptive), 'atm', or 'otm' (default: model)",
    )
    parser.add_argument("--min-hold-candles", type=int, default=6, help="Min LTF candles to hold before exit (default: 6 = 30 min)")
    parser.add_argument("--tail-threshold", type=float, default=0.005, help="Tail threshold for auto sigma steps (default: 0.005, lower = more levels)")
    parser.add_argument("--min-expected-profit", type=float, default=50.0, help="Min expected profit to place entry (default: $50)")
    parser.add_argument("--long-only", action="store_true", help="Only trade bullish signals (calls on dips), skip bearish/puts")
    parser.add_argument("--twirp-host", type=str, default="http://127.0.0.1:5051", help="Twirp server URL")

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

    args = parser.parse_args()

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
    logger.info("Options Mean-Reversion Strategy Demo")
    logger.info("=" * 60)
    logger.info(f"Symbol:            {args.symbol}")
    logger.info(f"Period:            {args.start} -> {args.end}")
    logger.info(f"Balance:           ${args.balance:,.2f}")
    logger.info(f"Max premium/group: {args.max_premium_pct * 100:.1f}%")
    logger.info(f"Stop percentile:   {args.stop_percentile * 100:.0f}th")
    logger.info(f"Model:             {args.model}")
    logger.info(f"Contracts/group:   {args.total_contracts}")
    logger.info(f"Target DTE:        {args.target_dte}")
    logger.info(f"Min/Max DTE:       {args.min_dte}/{args.max_dte}")
    logger.info(f"Profit target:     {args.profit_target * 100:.0f}%")
    logger.info(f"Time decay exit:   {args.time_decay_exit} DTE")
    logger.info(f"Strike strategy:   {args.strike_strategy}")
    logger.info(f"Min hold candles:  {args.min_hold_candles} ({args.min_hold_candles * 5} min)")
    logger.info(f"Tail threshold:    {args.tail_threshold}")
    logger.info(f"Min expected profit: ${args.min_expected_profit:.2f}")
    logger.info(f"Long only:         {args.long_only}")
    logger.info(f"HTF horizon:       {args.htf_horizon}")
    if args.retrain_interval:
        training_start_str = args.training_start or "(1 year before --start)"
        logger.info(f"Retrain:           {args.retrain_interval}")
        logger.info(f"Training start:    {training_start_str}")
        if args.rolling_window:
            logger.info(f"Rolling window:    {args.rolling_window} days")
        else:
            logger.info(f"Window:            expanding")
    else:
        logger.info(f"PDF:               {args.pdf_path or '(none)'}")
    logger.info(f"Server:            {args.twirp_host}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Load or build the initial PDF
    # ------------------------------------------------------------------
    on_tick_callback = None

    if args.retrain_interval:
        sim_start_dt = datetime.fromisoformat(args.start).replace(tzinfo=_NY)
        if args.training_start:
            training_start_dt = datetime.fromisoformat(args.training_start).replace(tzinfo=_NY)
        else:
            training_start_dt = sim_start_dt - timedelta(days=365)

        training_start_str = training_start_dt.strftime("%Y-%m-%d")

        logger.info(
            f"Fetching training data: {training_start_str} -> {args.end} ..."
        )
        all_ltf_bars, all_htf_bars = _fetch_all_candles(
            args.symbol, training_start_str, args.end, args.twirp_host,
        )
        logger.info(
            f"  Training data: {len(all_ltf_bars):,} LTF bars,"
            f" {len(all_htf_bars):,} HTF bars"
        )

        ltf_init = _slice_bars(all_ltf_bars, sim_start_dt)
        htf_init = _slice_bars(all_htf_bars, sim_start_dt)

        logger.info(
            f"Building initial PDF from {training_start_str}"
            f" -> {args.start}"
            f" ({len(ltf_init):,} LTF, {len(htf_init):,} HTF bars) ..."
        )
        pdf = _build_pdf(args.symbol, ltf_init, htf_init, args.model)
        logger.info(
            f"Initial PDF: {len(pdf.signals)} signals,"
            f" {len(pdf.get_sufficient_signals())} sufficient"
        )

        on_tick_callback = make_retrain_callback(
            symbol=args.symbol,
            all_ltf_bars=all_ltf_bars,
            all_htf_bars=all_htf_bars,
            training_start=training_start_dt,
            return_model=args.model,
            interval=args.retrain_interval,
            rolling_window_days=args.rolling_window,
        )

    elif args.pdf_path and os.path.exists(args.pdf_path):
        logger.info(f"Loading PDF from {args.pdf_path} ...")
        pdf = PDFDocument.load(args.pdf_path)
        sufficient = pdf.get_sufficient_signals()
        logger.info(f"PDF loaded: {len(pdf.signals)} signals, {len(sufficient)} sufficient")

    else:
        logger.error(
            "No PDF provided. Either pass --pdf-path or use --retrain-interval"
            " to build the PDF dynamically."
        )
        sys.exit(1)

    # ------------------------------------------------------------------
    # 2. Build repositories (5-min LTF + 1-hour HTF)
    # ------------------------------------------------------------------
    repos = OptionsMeanReversionStrategy.get_repositories(args.symbol)

    req = CreatePolygonPlaygroundRequest(
        balance=args.balance,
        start_date=args.start,
        stop_date=args.end,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )

    # ------------------------------------------------------------------
    # 3. Create simulator playground
    # ------------------------------------------------------------------
    logger.info("Creating simulator playground ...")
    playground = BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=args.twirp_host,
    )
    logger.info(f"Playground created — id: {playground.id}")
    logger.info(f"Initial timestamp:  {playground.timestamp.isoformat()}")
    logger.info(f"Account balance:    ${playground.account.balance:,.2f}")

    # ------------------------------------------------------------------
    # 4. Run the options mean-reversion strategy
    # ------------------------------------------------------------------
    logger.info("Running options mean-reversion strategy ...")
    try:
        strategy = OptionsMeanReversionStrategy(
            playground, args.symbol, logger,
            pdf=pdf,
            max_premium_pct=args.max_premium_pct,
            stop_percentile=args.stop_percentile,
            total_contracts_per_group=args.total_contracts,
            htf_horizon=args.htf_horizon,
            target_dte=args.target_dte,
            min_dte=args.min_dte,
            max_dte=args.max_dte,
            profit_target_pct=args.profit_target,
            time_decay_exit_dte=args.time_decay_exit,
            strike_strategy=args.strike_strategy,
            min_hold_candles=args.min_hold_candles,
            tail_threshold=args.tail_threshold,
            min_expected_profit=args.min_expected_profit,
            long_only=args.long_only,
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
    logger.info(f"Return:             {realized_pnl / args.balance * 100:.2f}%")

    total_premium = sum(g.total_premium_spent for g in strategy.trade_groups)
    logger.info(f"Total premium paid: ${total_premium:,.2f}")

    logger.info("-" * 60)
    logger.info("Open positions:")
    if playground.account.positions:
        for sym, pos in playground.account.positions.items():
            logger.info(
                f"  {sym:40s}  qty={pos.quantity:>4.0f}"
                f"  cost_basis=${pos.cost_basis:>10.2f}  P&L=${pos.pl:>10.2f}"
            )
    else:
        logger.info("  (none)")

    logger.info("-" * 60)
    logger.info(f"Trade groups:       {len(strategy.trade_groups)}")
    logger.info(f"Total trades:       {len(playground.trade_timestamps)}")
    logger.info("=" * 60)

    logger.info("Playground simulation complete.")
    logger.info(f"Playground id: {playground.id}")


if __name__ == "__main__":
    main()
