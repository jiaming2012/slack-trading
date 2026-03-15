#!/usr/bin/env python3
"""
Demo: PDF Mean-Reversion Strategy via Backtester Playground

Creates a simulator playground with 5-min LTF + 1-hour HTF candles, then
runs the mean-reversion strategy that buys dips at σ-deviation bands when
the HTF signals bullish.

Usage:
    python demo_mean_reversion.py \
        --symbol AAPL --start 2025-06-01 --end 2026-02-28 \
        --balance 100000 --pdf-path aapl_5m_1h_pdf.json \
        --max-loss-pct 0.02 --stop-percentile 0.95 --model bayesian_nig

Requires the Go trading server to be running on the specified twirp host.
"""

import argparse
import sys

from loguru import logger

from backtester_playground_client_grpc import (
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    RepositorySource,
)
from mean_reversion_strategy import MeanReversionStrategy, run_mean_reversion
from pdf_types import PDFDocument


def main():
    parser = argparse.ArgumentParser(
        description="Demo PDF mean-reversion strategy in the backtester playground",
    )
    parser.add_argument("--symbol", type=str, default="AAPL", help="Stock symbol (default: AAPL)")
    parser.add_argument("--start", type=str, default="2025-06-01", help="Simulation start date YYYY-MM-DD")
    parser.add_argument("--end", type=str, default="2026-02-28", help="Simulation end date YYYY-MM-DD")
    parser.add_argument("--balance", type=float, default=100_000, help="Starting account balance (default: 100000)")
    parser.add_argument("--pdf-path", type=str, required=True, help="Path to pre-built PDF JSON file")
    parser.add_argument("--max-loss-pct", type=float, default=0.02, help="Max loss per group as %% of equity (default: 0.02)")
    parser.add_argument("--stop-percentile", type=float, default=0.95, help="HTF stop percentile (default: 0.95)")
    parser.add_argument("--model", type=str, default="bayesian_nig", choices=["empirical", "bayesian_nig"], help="Return model (default: bayesian_nig)")
    parser.add_argument("--total-shares", type=int, default=1000, help="Total shares per group (default: 1000)")
    parser.add_argument("--exit-tiers", type=int, default=3, help="Number of partial exit tiers (default: 3)")
    parser.add_argument("--htf-horizon", type=str, default="1h", help="PDF horizon key for deviation levels (default: 1h)")
    parser.add_argument("--twirp-host", type=str, default="http://127.0.0.1:5051", help="Twirp server URL")
    args = parser.parse_args()

    # Configure logger
    logger.remove()
    logger.add(
        sys.stderr,
        level="INFO",
        format="<green>{time:HH:mm:ss}</green> | <level>{level:<8}</level> | {message}",
    )

    logger.info("=" * 60)
    logger.info("PDF Mean-Reversion Strategy Demo")
    logger.info("=" * 60)
    logger.info(f"Symbol:          {args.symbol}")
    logger.info(f"Period:          {args.start} -> {args.end}")
    logger.info(f"Balance:         ${args.balance:,.2f}")
    logger.info(f"PDF:             {args.pdf_path}")
    logger.info(f"Max loss/group:  {args.max_loss_pct * 100:.1f}%")
    logger.info(f"Stop percentile: {args.stop_percentile * 100:.0f}th")
    logger.info(f"Model:           {args.model}")
    logger.info(f"Shares/group:    {args.total_shares}")
    logger.info(f"Exit tiers:      {args.exit_tiers}")
    logger.info(f"HTF horizon:     {args.htf_horizon}")
    logger.info(f"Server:          {args.twirp_host}")
    logger.info("=" * 60)

    # ------------------------------------------------------------------
    # 1. Load PDF
    # ------------------------------------------------------------------
    logger.info("Loading PDF ...")
    pdf = PDFDocument.load(args.pdf_path)
    sufficient = pdf.get_sufficient_signals()
    logger.info(f"PDF loaded: {len(pdf.signals)} signals, {len(sufficient)} sufficient")

    # ------------------------------------------------------------------
    # 2. Build repositories (5-min LTF + 1-hour HTF)
    # ------------------------------------------------------------------
    repos = MeanReversionStrategy.get_repositories(args.symbol)

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
    # 4. Run the mean-reversion strategy
    # ------------------------------------------------------------------
    logger.info("Running mean-reversion strategy ...")
    try:
        strategy = run_mean_reversion(
            playground, args.symbol, logger, pdf,
            max_loss_pct=args.max_loss_pct,
            stop_percentile=args.stop_percentile,
            total_shares_per_group=args.total_shares,
            num_exit_tiers=args.exit_tiers,
            htf_horizon=args.htf_horizon,
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

    logger.info("-" * 60)
    logger.info(f"Trade groups:       {len(strategy.trade_groups)}")
    logger.info(f"Total trades:       {len(playground.trade_timestamps)}")
    logger.info("=" * 60)

    logger.info("Playground simulation complete.")
    logger.info(f"Playground id: {playground.id}")


if __name__ == "__main__":
    main()
