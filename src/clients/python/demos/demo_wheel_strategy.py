#!/usr/bin/env python3
"""
Demo: Wheel Strategy via Backtester Playground

The Wheel Strategy alternates between two phases:
  Phase 1 — Sell cash-secured OTM puts (no stock held).
             If assigned, receive 100 shares → move to Phase 2.
  Phase 2 — Sell covered calls (stock held).
             If called away, lose stock → move back to Phase 1.

Creates a simulator playground for the chosen symbol and runs the full
wheel cycle over the specified date range.

Usage:
    python demo_wheel_strategy.py [--symbol AAPL] [--start 2024-06-01] \
        [--end 2024-12-31] [--balance 100000] [--twirp-host http://127.0.0.1:5051]

Requires the Go trading server to be running on the specified twirp host.
"""

import argparse
import sys
from datetime import datetime
from loguru import logger

from engine.client import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    RepositorySource,
)
from engine.trading_engine import run_strategy
from deprecated.wheel import WheelStrategy, generate_signal_stats


def main():
    parser = argparse.ArgumentParser(
        description="Demo wheel strategy in the backtester playground"
    )
    parser.add_argument(
        "--symbol", type=str, default="AAPL", help="Underlying stock symbol (default: AAPL)"
    )
    parser.add_argument(
        "--start", type=str, default="2024-06-01", help="Simulation start date YYYY-MM-DD"
    )
    parser.add_argument(
        "--end", type=str, default="2024-12-31", help="Simulation end date YYYY-MM-DD"
    )
    parser.add_argument(
        "--balance", type=float, default=100_000, help="Starting account balance (default: 100000)"
    )
    parser.add_argument(
        "--twirp-host", type=str, default="http://127.0.0.1:5051", help="Twirp server URL"
    )
    args = parser.parse_args()

    symbol = args.symbol
    start_date = args.start
    end_date = args.end
    balance = args.balance
    twirp_host = args.twirp_host

    # Configure logger
    logger.remove()
    logger.add(
        sys.stderr,
        level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    logger.info("=" * 60)
    logger.info("Wheel Strategy Demo")
    logger.info("=" * 60)
    logger.info(f"Symbol:     {symbol}")
    logger.info(f"Period:     {start_date} -> {end_date}")
    logger.info(f"Balance:    ${balance:,.2f}")
    logger.info(f"Server:     {twirp_host}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Build repositories (1-hour + 1-day candles with indicators)
    # ------------------------------------------------------------------
    repos = WheelStrategy.get_repositories(
        symbol,
        datetime.fromisoformat(start_date),
        datetime.fromisoformat(end_date),
    )

    req = CreatePolygonPlaygroundRequest(
        balance=balance,
        start_date=start_date,
        stop_date=end_date,
        repositories=repos,
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )

    # ------------------------------------------------------------------
    # 2. Create a simulator playground
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
    # 3. Run the wheel strategy
    # ------------------------------------------------------------------
    logger.info("Running wheel strategy ...")
    try:
        generate_signal_stats(playground, symbol)
        playground.stats.generate_model()
        strategy = WheelStrategy(playground, symbol, logger, max_open_count=3)
        run_strategy(strategy, playground, logger)
    except Exception:
        logger.exception("Strategy encountered an error")
        sys.exit(1)

    # ------------------------------------------------------------------
    # 4. Print results
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

    # Clean up
    try:
        logger.info("Playground simulation complete.")
        logger.info(
            f"Playground id: {playground.id}"
            " (you may want to remove it manually from the server)"
        )
    except Exception:
        logger.warning("Could not remove playground from server (non-fatal).")


if __name__ == "__main__":
    main()
