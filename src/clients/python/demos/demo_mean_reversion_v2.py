#!/usr/bin/env python3
"""
Demo: PDF Mean-Reversion Strategy V2 (datasource variant) via Backtester Playground

Creates a simulator playground with 5-min LTF + 1-hour HTF candles, then
runs the V2 mean-reversion strategy (using datasource-based signal detection)
that buys dips at sigma-deviation bands when the HTF signals bullish.

This is identical to demo_mean_reversion.py except it uses
MeanReversionStrategyV2 (from the TradeSignal framework migration).

Supports optional periodic PDF retraining during the simulation.

Usage:
    # Static PDF (simulator):
    python demo_mean_reversion_v2.py \
        --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
        --balance 100000 --pdf-path aapl_5m_1h_pdf.json

    # Weekly retraining (simulator):
    python demo_mean_reversion_v2.py \
        --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
        --balance 100000 --retrain-interval weekly --model bayesian_nig

    # Live paper money (Tradier sandbox):
    python demo_mean_reversion_v2.py \
        --live --symbol AAPL --balance 100000 \
        --pdf-path aapl_5m_1h_pdf.json

    # Live real money (Tradier margin -- requires confirmation):
    python demo_mean_reversion_v2.py \
        --live margin --symbol AAPL --balance 100000 \
        --pdf-path aapl_5m_1h_pdf.json

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
from engine.types import LiveAccountType
from tools.build_pdf_from_polygon import bar_to_dict
from engine.trading_engine import run_strategy
from strategies.mean_reversion_v2 import MeanReversionStrategyV2
from lib.pdf_builder import PDFBuilder
from lib.pdf_types import PDFDocument


# ------------------------------------------------------------------ #
# Retrain helpers
# ------------------------------------------------------------------ #

_NY = ZoneInfo("America/New_York")

# Mean-reversion strategy constants (5-min LTF + 1-hour HTF)
_LTF_PERIOD = 300       # 5-min in seconds
_HTF_PERIOD = 3600      # 1-hour in seconds
_HORIZONS = {"1h": 12, "4h": 48, "1d": 78}
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
    """Check if an HTF (1-hour) candle represents a retrain boundary.

    For weekly: triggers on the last 1-hour bar of Friday (16:00 ET close).
    For monthly: triggers on the last trading hour of the month's last day.
    """
    if not hasattr(candle, "period") or candle.period != _HTF_PERIOD:
        return False

    bar_dt = candle.bar.datetime
    if isinstance(bar_dt, str):
        bar_dt = datetime.fromisoformat(bar_dt.replace("Z", "+00:00"))
    if bar_dt.tzinfo is None:
        bar_dt = bar_dt.replace(tzinfo=_NY)

    if interval == "weekly":
        # Friday's closing hour (15:00-16:00 ET = hour 15)
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
    """Create a callback for ``run_strategy(on_tick=...)``."""
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
        description="Demo PDF mean-reversion V2 strategy (datasource variant) in the backtester playground",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Retrain mode:\n"
            "  When --retrain-interval is set, the PDF is rebuilt periodically\n"
            "  during the simulation. No --pdf-path is needed.\n"
            "\n"
            "Examples:\n"
            "  # Static PDF:\n"
            "  python demo_mean_reversion_v2.py --pdf-path aapl_5m_1h_pdf.json\n"
            "\n"
            "  # Weekly retrain:\n"
            "  python demo_mean_reversion_v2.py --retrain-interval weekly\n"
        ),
    )
    parser.add_argument("--symbol", type=str, default="AAPL", help="Stock symbol (default: AAPL)")
    parser.add_argument("--start", type=str, default="2025-06-01", help="Simulation start date YYYY-MM-DD")
    parser.add_argument("--end", type=str, default="2026-02-28", help="Simulation end date YYYY-MM-DD")
    parser.add_argument("--balance", type=float, default=100_000, help="Starting account balance (default: 100000)")
    parser.add_argument("--pdf-path", type=str, default="", help="Path to pre-built PDF JSON (used when --retrain-interval is not set)")
    parser.add_argument("--max-loss-pct", type=float, default=0.02, help="Max loss per group as %% of equity (default: 0.02)")
    parser.add_argument("--stop-percentile", type=float, default=0.95, help="HTF stop percentile (default: 0.95)")
    parser.add_argument("--model", type=str, default="bayesian_nig", choices=["empirical", "bayesian_nig"], help="Return model (default: bayesian_nig)")
    parser.add_argument("--total-shares", type=int, default=0, help="Total shares per group (0 = auto-size from balance/price)")
    parser.add_argument("--exit-tiers", type=int, default=3, help="Number of partial exit tiers (default: 3)")
    parser.add_argument(
        "--tier-spacing", type=str, default="even",
        choices=["even", "tight"],
        help="Exit tier spacing: 'even' (25%%,50%%,100%%) or 'tight' (70%%,85%%,100%%) (default: even)",
    )
    parser.add_argument("--htf-horizon", type=str, default="1h", help="PDF horizon key for deviation levels (default: 1h)")
    parser.add_argument(
        "--stop-widen-on-exit", type=float, default=1.0,
        help="Stop widening factor for partial exits (0.0=disabled, 1.0=doubles at 100%% exit progress, default: 1.0)",
    )
    parser.add_argument(
        "--min-expected-profit", type=float, default=0.0,
        help="Skip entries with expected profit below this threshold (default: 0.0 = no filter)",
    )
    parser.add_argument(
        "--ev-model", type=str, default="distribution",
        choices=["binary", "distribution"],
        help="EV model: 'binary' (two-outcome) or 'distribution' (forward return integration) (default: distribution)",
    )
    parser.add_argument(
        "--live", type=str, nargs="?", const="paper", default=None,
        choices=["paper", "margin"],
        help="Run in live mode: 'paper' (default) or 'margin' (real money)",
    )
    parser.add_argument("--twirp-host", type=str, default="http://127.0.0.1:5051", help="Twirp server URL")
    parser.add_argument("--client-id", type=str, default="", help="Client ID for playground reuse across restarts")

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
        "--save-to-db", action="store_true",
        help="Persist backtest results (orders, trades, equity) to Postgres on completion",
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
    logger.info("PDF Mean-Reversion Strategy V2 Demo (datasource variant)")
    logger.info("=" * 60)
    if args.live:
        live_account_type = LiveAccountType.PAPER if args.live == "paper" else LiveAccountType.MARGIN
        if args.live == "margin":
            logger.warning("*** REAL MONEY MODE (margin) ***")
            confirm = input("Type 'yes' to confirm trading with real money: ")
            if confirm.strip().lower() != "yes":
                logger.info("Aborted.")
                sys.exit(0)
        logger.info(f"Mode:            LIVE ({args.live})")
    else:
        live_account_type = None
        logger.info(f"Mode:            SIMULATOR")
    logger.info(f"Symbol:          {args.symbol}")
    if not args.live:
        logger.info(f"Period:          {args.start} -> {args.end}")
    logger.info(f"Balance:         ${args.balance:,.2f}")
    logger.info(f"Max loss/group:  {args.max_loss_pct * 100:.1f}%")
    logger.info(f"Stop percentile: {args.stop_percentile * 100:.0f}th")
    logger.info(f"Model:           {args.model}")
    logger.info(f"Shares/group:    {args.total_shares if args.total_shares > 0 else 'auto'}")
    logger.info(f"Exit tiers:      {args.exit_tiers}")
    logger.info(f"Tier spacing:    {args.tier_spacing}")
    logger.info(f"Stop widen:      {args.stop_widen_on_exit}")
    logger.info(f"Min EV filter:   ${args.min_expected_profit:.2f}")
    logger.info(f"EV model:        {args.ev_model}")
    logger.info(f"HTF horizon:     {args.htf_horizon}")
    if args.save_to_db:
        logger.info(f"Save to DB:      YES")
    if args.retrain_interval:
        training_start_str = args.training_start or "(1 year before --start)"
        logger.info(f"Retrain:         {args.retrain_interval}")
        logger.info(f"Training start:  {training_start_str}")
        if args.rolling_window:
            logger.info(f"Rolling window:  {args.rolling_window} days")
        else:
            logger.info(f"Window:          expanding")
    else:
        logger.info(f"PDF:             {args.pdf_path or '(none)'}")
    logger.info(f"Server:          {args.twirp_host}")
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

        # Build initial PDF from [training_start, sim_start]
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
    repos = MeanReversionStrategyV2.get_repositories(args.symbol)

    if args.live:
        env = PlaygroundEnvironment.LIVE
        req = CreatePolygonPlaygroundRequest(
            balance=args.balance,
            start_date=None,
            stop_date=None,
            repositories=repos,
            environment=env.value,
        )
    else:
        env = PlaygroundEnvironment.SIMULATOR
        req = CreatePolygonPlaygroundRequest(
            balance=args.balance,
            start_date=args.start,
            stop_date=args.end,
            repositories=repos,
            environment=env.value,
        )

    if args.client_id:
        req.client_id = args.client_id
        logger.info(f"Client ID:       {args.client_id}")

    # ------------------------------------------------------------------
    # 3. Create playground
    # ------------------------------------------------------------------
    logger.info(f"Creating {env.value} playground ...")
    playground = BacktesterPlaygroundClient(
        req,
        live_account_type=live_account_type,
        source=None if args.live else RepositorySource.POLYGON,
        logger=logger,
        twirp_host=args.twirp_host,
    )
    logger.info(f"Playground created -- id: {playground.id}")
    logger.info(f"Initial timestamp:  {playground.timestamp.isoformat()}")
    logger.info(f"Account balance:    ${playground.account.balance:,.2f}")

    # ------------------------------------------------------------------
    # 4. Run the mean-reversion V2 strategy
    # ------------------------------------------------------------------
    logger.info("Running mean-reversion V2 strategy ...")
    if args.live:
        logger.info("Press Ctrl+C to stop live trading.")
    try:
        strategy = MeanReversionStrategyV2(
            playground, args.symbol, logger,
            pdf=pdf,
            max_loss_pct=args.max_loss_pct,
            stop_percentile=args.stop_percentile,
            total_shares_per_group=args.total_shares,
            num_exit_tiers=args.exit_tiers,
            htf_horizon=args.htf_horizon,
            tier_spacing=args.tier_spacing,
            stop_widen_on_exit=args.stop_widen_on_exit,
            min_expected_profit=args.min_expected_profit,
            ev_model=args.ev_model,
        )
        run_strategy(strategy, playground, logger, on_tick=on_tick_callback)
    except KeyboardInterrupt:
        logger.info("Interrupted by user -- stopping strategy.")
        strategy = None
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

    if strategy is not None:
        logger.info("-" * 60)
        logger.info(f"Trade groups:       {len(strategy.trade_groups)}")
        logger.info(f"Total trades:       {len(playground.trade_timestamps)}")

    logger.info("=" * 60)
    logger.info(f"Playground {'stopped' if args.live else 'simulation complete'}.")
    logger.info(f"Playground id: {playground.id}")

    # ------------------------------------------------------------------
    # 6. Persist to database (opt-in via --save-to-db)
    # ------------------------------------------------------------------
    if args.save_to_db and not args.live:
        from rpc.playground_pb2 import SavePlaygroundRequest

        logger.info("Saving playground to database ...")
        request = SavePlaygroundRequest(playground_id=playground.id)
        playground.network_call_with_retry(
            'save_playground', playground.client.SavePlayground, request
        )
        logger.info(f"Playground saved -- id: {playground.id}")

        # Insert backtest_runs summary row
        from engine.persistence import save_backtest_run

        params = strategy.get_parameters() if strategy is not None else {}
        params["model"] = args.model  # return model is a demo-level param

        run_id = save_backtest_run(
            playground_id=playground.id,
            client_id=playground.client_id or f"{strategy.label}-{args.symbol}-{args.start}",
            strategy_name=strategy.label if strategy is not None else "unknown",
            parameters=params,
            starting_balance=args.balance,
            final_balance=playground.account.balance,
            start_date=args.start,
            end_date=args.end,
        )
        logger.info(f"Backtest run saved -- id: {run_id}")
    elif args.save_to_db and args.live:
        logger.warning("--save-to-db is only supported for simulator mode, skipping.")


if __name__ == "__main__":
    main()
