#!/usr/bin/env python3
"""
Build a PDF from real Polygon data fetched through the backtester playground.

Creates a playground with LTF + HTF repos, fetches all candles, feeds them
to PDFBuilder, and saves the result as JSON.

Supports two strategy presets:
  - wheel:           15-min LTF + daily HTF (default, for pdf_wheel_strategy)
  - mean_reversion:  5-min LTF + 1-hour HTF (for mean_reversion_strategy)

Custom timeframes can also be specified directly via --ltf and --htf flags.

Usage:
    # Wheel strategy (default):
    python build_pdf_from_polygon.py --symbol AAPL --start 2024-06-01 --end 2025-06-01

    # Mean-reversion strategy:
    python build_pdf_from_polygon.py --strategy mean_reversion \
        --symbol AAPL --start 2024-06-01 --end 2025-06-01

    # Custom timeframes:
    python build_pdf_from_polygon.py --ltf 5m --htf 1h \
        --symbol AAPL --start 2024-06-01 --end 2025-06-01

Requires the Go trading server to be running.
"""

import argparse
import sys
from datetime import datetime
from typing import Dict, Tuple
from zoneinfo import ZoneInfo

from loguru import logger

from engine.client import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    Repository,
    RepositorySource,
)
from lib.pdf_builder import PDFBuilder


# ------------------------------------------------------------------ #
# Strategy presets
# ------------------------------------------------------------------ #

# (ltf_multiplier, ltf_unit, htf_multiplier, htf_unit, htf_timeframe, default_horizons)
STRATEGY_PRESETS: Dict[str, Tuple] = {
    "wheel": (
        15, "minute",    # LTF: 15-min
        1, "day",        # HTF: daily
        "daily",         # htf_timeframe for PDFBuilder
        {"1h": 4, "4h": 16, "1d": 26, "2d": 52},
    ),
    "mean_reversion": (
        5, "minute",     # LTF: 5-min
        1, "hour",       # HTF: 1-hour
        "ltf",           # htf_timeframe for PDFBuilder
        {"1h": 12, "4h": 48, "1d": 78},
    ),
}


def _parse_timeframe(spec: str) -> Tuple[int, str, int]:
    """
    Parse a timeframe spec like '5m', '15m', '1h', '1d' into
    (multiplier, unit_name, period_seconds).
    """
    spec = spec.strip().lower()
    unit_map = {
        "m": ("minute", 60),
        "h": ("hour", 3600),
        "d": ("day", 86400),
        "w": ("week", 604800),
    }
    for suffix, (unit_name, unit_secs) in unit_map.items():
        if spec.endswith(suffix):
            multiplier = int(spec[: -len(suffix)])
            return multiplier, unit_name, multiplier * unit_secs
    raise ValueError(
        f"Invalid timeframe spec: {spec!r}. "
        f"Use format like '5m', '15m', '1h', '1d'."
    )


def bar_to_dict(bar) -> dict:
    """Convert a protobuf Bar to a plain dict for PDFBuilder."""
    return {
        "open": bar.open,
        "high": bar.high,
        "low": bar.low,
        "close": bar.close,
        "datetime": bar.datetime,
        "superD_50_3": bar.superD_50_3,
        "stochrsi_k_14_14_3_3": bar.stochrsi_k_14_14_3_3,
        "stochrsi_cross_above_20": bar.stochrsi_cross_above_20,
        "stochrsi_cross_below_80": bar.stochrsi_cross_below_80,
        "sma_50": bar.sma_50,
        "sma_100": bar.sma_100,
        "sma_200": bar.sma_200,
    }


def main():
    parser = argparse.ArgumentParser(
        description="Build PDF from Polygon data",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Strategy presets:\n"
            "  wheel            15-min LTF + daily HTF (default)\n"
            "  mean_reversion   5-min LTF + 1-hour HTF\n"
            "\n"
            "Custom timeframes override the strategy preset:\n"
            "  --ltf 5m --htf 1h\n"
            "  --ltf 15m --htf 1d\n"
        ),
    )
    parser.add_argument("--symbol", default="AAPL", help="Stock symbol (default: AAPL)")
    parser.add_argument("--start", default="2024-06-01", help="Start date YYYY-MM-DD")
    parser.add_argument("--end", default="2025-06-01", help="End date YYYY-MM-DD")
    parser.add_argument(
        "--strategy", default="wheel",
        choices=list(STRATEGY_PRESETS.keys()),
        help="Strategy preset (default: wheel)",
    )
    parser.add_argument("--ltf", default=None, help="Custom LTF timeframe, e.g. '5m', '15m' (overrides --strategy)")
    parser.add_argument("--htf", default=None, help="Custom HTF timeframe, e.g. '1h', '1d' (overrides --strategy)")
    parser.add_argument(
        "--return-model", default="empirical",
        choices=["empirical", "bayesian_nig"],
        help="Statistical model for return distributions (default: empirical)",
    )
    parser.add_argument("--twirp-host", default="http://127.0.0.1:5051", help="Twirp server URL")
    parser.add_argument("--output", default="", help="Output JSON path (default: <symbol>_pdf.json)")
    args = parser.parse_args()

    symbol = args.symbol
    start_date = args.start
    end_date = args.end
    twirp_host = args.twirp_host

    # ------------------------------------------------------------------
    # Resolve timeframes from strategy preset or custom flags
    # ------------------------------------------------------------------
    preset = STRATEGY_PRESETS[args.strategy]
    ltf_mult, ltf_unit, htf_mult, htf_unit, htf_timeframe, horizons = preset

    if args.ltf:
        ltf_mult, ltf_unit, _ = _parse_timeframe(args.ltf)
    if args.htf:
        htf_mult, htf_unit, _ = _parse_timeframe(args.htf)
        # When using custom HTF, use "ltf" timeframe unless it's daily
        htf_timeframe = "daily" if htf_unit == "day" else "ltf"

    from utils import get_timespan_unit
    ltf_period = ltf_mult * get_timespan_unit(ltf_unit)
    htf_period = htf_mult * get_timespan_unit(htf_unit)

    output_path = args.output or f"{symbol.lower()}_pdf.json"

    # ------------------------------------------------------------------
    # Logger
    # ------------------------------------------------------------------
    logger.remove()
    logger.add(
        sys.stderr, level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    ltf_label = f"{ltf_mult}-{ltf_unit}"
    htf_label = f"{htf_mult}-{htf_unit}"

    logger.info("=" * 60)
    logger.info(f"Building PDF for {symbol}")
    logger.info(f"Strategy:  {args.strategy}")
    logger.info(f"LTF:       {ltf_label} ({ltf_period}s)")
    logger.info(f"HTF:       {htf_label} ({htf_period}s)")
    logger.info(f"Model:     {args.return_model}")
    logger.info(f"Period:    {start_date} -> {end_date}")
    logger.info(f"Server:    {twirp_host}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Create playground with LTF + HTF repos
    # ------------------------------------------------------------------
    indicators = [
        "supertrend", "stochrsi", "atr", "doji", "hammer",
        "sma_50", "sma_100", "sma_200",
        "stochrsi_cross_above_20", "stochrsi_cross_below_80",
    ]

    repos = [
        Repository(
            symbol=symbol,
            timespan_multiplier=ltf_mult,
            timespan_unit=ltf_unit,
            indicators=indicators,
            history_in_days=365,
        ),
        Repository(
            symbol=symbol,
            timespan_multiplier=htf_mult,
            timespan_unit=htf_unit,
            indicators=indicators,
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

    logger.info("Creating playground ...")
    playground = BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=twirp_host,
    )
    logger.info(f"Playground created — id: {playground.id}")

    # ------------------------------------------------------------------
    # 2. Fetch all candles from repos
    # ------------------------------------------------------------------
    ts_start = datetime.fromisoformat(start_date).replace(tzinfo=ZoneInfo("America/New_York"))
    ts_end = datetime.fromisoformat(end_date).replace(tzinfo=ZoneInfo("America/New_York"))

    logger.info(f"Fetching {ltf_label} candles ...")
    ltf_bars_pb = playground.fetch_candles_v3(symbol, ltf_period, ts_start, ts_end)
    logger.info(f"  Got {len(ltf_bars_pb)} {ltf_label} bars")

    logger.info(f"Fetching {htf_label} candles ...")
    htf_bars_pb = playground.fetch_candles_v3(symbol, htf_period, ts_start, ts_end)
    logger.info(f"  Got {len(htf_bars_pb)} {htf_label} bars")

    # Convert protobuf bars to dicts
    ltf_bars = [bar_to_dict(b) for b in ltf_bars_pb]
    htf_bars = [bar_to_dict(b) for b in htf_bars_pb]

    # ------------------------------------------------------------------
    # 3. Build PDF
    # ------------------------------------------------------------------
    logger.info("Building PDF ...")
    builder = PDFBuilder(
        symbol=symbol,
        ltf_bars=ltf_bars,
        daily_bars=htf_bars,
        ltf_period_seconds=ltf_period,
        horizons=horizons,
        htf_timeframe=htf_timeframe,
        return_model=args.return_model,
    )
    pdf = builder.build()

    # ------------------------------------------------------------------
    # 4. Summary + save
    # ------------------------------------------------------------------
    sufficient = pdf.get_sufficient_signals()
    logger.info(f"Total compound signals: {len(pdf.signals)}")
    logger.info(f"Sufficient samples:     {len(sufficient)}")

    for key, spdf in sorted(pdf.signals.items(), key=lambda x: x[1].sample_size, reverse=True)[:10]:
        tag = " *" if spdf.sufficient_samples else ""
        logger.info(f"  {key:50s}  n={spdf.sample_size:>5}{tag}")

    if len(pdf.signals) > 10:
        logger.info(f"  ... and {len(pdf.signals) - 10} more")

    pdf.save(output_path)
    logger.info(f"PDF saved to {output_path}")


if __name__ == "__main__":
    main()
