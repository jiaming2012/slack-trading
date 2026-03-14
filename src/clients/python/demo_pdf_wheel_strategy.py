#!/usr/bin/env python3
"""
Demo: PDF-Guided Wheel Strategy via Backtester Playground

Same flow as demo_wheel_strategy.py, but uses empirical probability
distributions (PDFs) to guide put strike selection and Kelly-based sizing.

Steps:
  1. Build (or load) the PDF from historical data
  2. Create a simulator playground with 15-min + daily repos
  3. Run the PDF-guided wheel strategy loop
  4. Print metrics

Usage:
    python demo_pdf_wheel_strategy.py [--symbol AAPL] [--start 2025-01-01] \
        [--end 2025-06-30] [--balance 100000] [--twirp-host http://127.0.0.1:5051] \
        [--pdf-path aapl_pdf.json]

Requires the Go trading server to be running on the specified twirp host.
"""

import argparse
import os
import sys
from loguru import logger

from backtester_playground_client_grpc import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    RepositorySource,
)
from pdf_types import PDFDocument
from pdf_wheel_strategy import PDFWheelStrategy, run_pdf_wheel_strategy


def main():
    parser = argparse.ArgumentParser(
        description="Demo PDF-guided wheel strategy in the backtester playground"
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
        help="Path to pre-built PDF JSON. If not provided, must be built first.",
    )
    parser.add_argument(
        "--kelly-fraction", type=float, default=0.5,
        help="Kelly fraction multiplier (default: 0.5 = half-Kelly)",
    )
    args = parser.parse_args()

    symbol = args.symbol
    start_date = args.start
    end_date = args.end
    balance = args.balance
    twirp_host = args.twirp_host
    pdf_path = args.pdf_path
    kelly_frac = args.kelly_fraction

    # Configure logger
    logger.remove()
    logger.add(
        sys.stderr,
        level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    logger.info("=" * 60)
    logger.info("PDF-Guided Wheel Strategy Demo")
    logger.info("=" * 60)
    logger.info(f"Symbol:         {symbol}")
    logger.info(f"Period:         {start_date} -> {end_date}")
    logger.info(f"Balance:        ${balance:,.2f}")
    logger.info(f"Kelly fraction: {kelly_frac}")
    logger.info(f"Server:         {twirp_host}")
    logger.info(f"PDF path:       {pdf_path or '(will build from scratch)'}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Load or build the PDF
    # ------------------------------------------------------------------
    if pdf_path and os.path.exists(pdf_path):
        logger.info(f"Loading PDF from {pdf_path} ...")
        pdf = PDFDocument.load(pdf_path)
        logger.info(
            f"PDF loaded: {len(pdf.signals)} signals, "
            f"scan range {pdf.scan_start} to {pdf.scan_end}"
        )
    else:
        logger.error(
            "No PDF file provided or file not found. "
            "Please build a PDF first using pdf_builder.py and pass --pdf-path."
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
        run_pdf_wheel_strategy(
            playground, symbol, logger, twirp_host,
            pdf=pdf,
            peak_equity=balance,
            kelly_fraction_mult=kelly_frac,
        )
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
