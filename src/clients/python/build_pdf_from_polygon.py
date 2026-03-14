#!/usr/bin/env python3
"""
Build a PDF from real Polygon data fetched through the backtester playground.

Creates a playground with 15-min + daily repos for AAPL, fetches all candles
from the repos, feeds them to PDFBuilder, and saves the result as JSON.

Usage:
    python build_pdf_from_polygon.py [--symbol AAPL] [--start 2024-06-01] \
        [--end 2025-06-01] [--twirp-host http://127.0.0.1:5051] \
        [--output aapl_pdf.json]

Requires the Go trading server to be running.
"""

import argparse
import sys
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

from loguru import logger

from backtester_playground_client_grpc import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    Repository,
    RepositorySource,
)
from pdf_builder import PDFBuilder


LTF_PERIOD = 900     # 15-min in seconds
HTF_PERIOD = 86400   # 1-day in seconds


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
    parser = argparse.ArgumentParser(description="Build PDF from Polygon data")
    parser.add_argument("--symbol", default="AAPL", help="Stock symbol (default: AAPL)")
    parser.add_argument("--start", default="2024-06-01", help="Start date YYYY-MM-DD")
    parser.add_argument("--end", default="2025-06-01", help="End date YYYY-MM-DD")
    parser.add_argument("--twirp-host", default="http://127.0.0.1:5051", help="Twirp server URL")
    parser.add_argument("--output", default="", help="Output JSON path (default: <symbol>_pdf.json)")
    args = parser.parse_args()

    symbol = args.symbol
    start_date = args.start
    end_date = args.end
    twirp_host = args.twirp_host
    output_path = args.output or f"{symbol.lower()}_pdf.json"

    logger.remove()
    logger.add(
        sys.stderr, level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    logger.info("=" * 60)
    logger.info(f"Building PDF for {symbol}")
    logger.info(f"Period: {start_date} -> {end_date}")
    logger.info(f"Server: {twirp_host}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Create playground with 15-min + daily repos
    # ------------------------------------------------------------------
    indicators = [
        "supertrend", "stochrsi", "atr", "doji", "hammer",
        "50_sma", "100_sma", "200_sma",
        "stochrsi_cross_above_20", "stochrsi_cross_below_80",
    ]

    repos = [
        Repository(
            symbol=symbol,
            timespan_multiplier=15,
            timespan_unit="minute",
            indicators=indicators,
            history_in_days=365,
        ),
        Repository(
            symbol=symbol,
            timespan_multiplier=1,
            timespan_unit="day",
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

    logger.info("Fetching 15-min candles ...")
    ltf_bars_pb = playground.fetch_candles_v3(symbol, LTF_PERIOD, ts_start, ts_end)
    logger.info(f"  Got {len(ltf_bars_pb)} 15-min bars")

    logger.info("Fetching daily candles ...")
    daily_bars_pb = playground.fetch_candles_v3(symbol, HTF_PERIOD, ts_start, ts_end)
    logger.info(f"  Got {len(daily_bars_pb)} daily bars")

    # Convert protobuf bars to dicts
    ltf_bars = [bar_to_dict(b) for b in ltf_bars_pb]
    daily_bars = [bar_to_dict(b) for b in daily_bars_pb]

    # ------------------------------------------------------------------
    # 3. Build PDF
    # ------------------------------------------------------------------
    logger.info("Building PDF ...")
    builder = PDFBuilder(
        symbol=symbol,
        ltf_bars=ltf_bars,
        daily_bars=daily_bars,
        ltf_period_seconds=LTF_PERIOD,
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
