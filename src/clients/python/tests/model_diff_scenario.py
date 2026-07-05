#!/usr/bin/env python3
"""
Model-diff scenario runner (MIG-03 / model-migration-diff-test).

Runs the fixed, deterministic backtester scenario described by
``tests/baselines/model-diff-scenario.json`` against a live Go trading
server and prints (or writes) a canonical, byte-for-byte-stable
serialization of the resulting orders, fills, tick-by-tick equity, and
final account state.

This is the harness behind `task test:model-diff` (verification gate G4
of the reconcile-models-packages change): the same scenario is run before
and after the models/eventmodels package merge, and the two canonical
outputs are diffed byte-for-byte. Any behavioral drift introduced by the
merge shows up as a diff.

Determinism notes:
  - Simulator environment replays recorded historical (Polygon) candle
    data -- no wall-clock or network variance in the compared output.
  - Order IDs and trade IDs are assigned from a per-playground counter
    seeded fresh for every new playground, so a fixed call sequence
    against a fresh playground always reproduces the same IDs.
  - Order `create_date` / trade `create_date` are stamped from the
    playground's simulated clock (the candle timestamp), not wall time.
  - All floats in the output are rounded and rendered with fixed
    precision (6 decimal places) so float formatting can never be the
    source of a diff.

Usage:
    python -m tests.model_diff_scenario [--scenario PATH] [--twirp-host URL] [--output PATH]

Requires the Go trading server to be running (see `task app:dev`).
"""

import argparse
import json
import sys
from pathlib import Path

from loguru import logger
from google.protobuf.json_format import MessageToDict

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from engine.client import (  # noqa: E402
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    RepositorySource,
)
from rpc.playground_pb2 import Repository  # noqa: E402
from engine.types import OrderSide  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[4]
DEFAULT_SCENARIO_PATH = REPO_ROOT / "src/clients/python/tests/baselines/model-diff-scenario.json"
DEFAULT_REFERENCE_PATH = REPO_ROOT / "src/clients/python/tests/baselines/model-diff-reference.txt"

# Safety cap: refuse to run away if is_backtest_complete() never trips.
MAX_TICKS = 200

_ORDER_SIDES = {
    "buy": OrderSide.BUY,
    "sell": OrderSide.SELL,
    "sell_short": OrderSide.SELL_SHORT,
    "buy_to_cover": OrderSide.BUY_TO_COVER,
    "buy_to_open": OrderSide.BUY_TO_OPEN,
    "sell_to_close": OrderSide.SELL_TO_CLOSE,
    "sell_to_open": OrderSide.SELL_TO_OPEN,
    "buy_to_close": OrderSide.BUY_TO_CLOSE,
}


def _round_floats(obj):
    """Recursively render every float as a fixed-precision string.

    This makes the serialized output immune to float-repr differences
    across Python/library versions -- the design.md requirement for
    "fixed float formatting". Non-float leaves pass through unchanged.
    """
    if isinstance(obj, float):
        return format(round(obj, 6), ".6f")
    if isinstance(obj, dict):
        return {k: _round_floats(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [_round_floats(v) for v in obj]
    return obj


def _order_to_dict(order) -> dict:
    d = MessageToDict(order, preserving_proto_field_name=True)
    return _round_floats(d)


def load_scenario(path: Path) -> dict:
    with open(path) as f:
        return json.load(f)


def run_scenario(scenario: dict, twirp_host: str) -> dict:
    """Run the fixed scenario against a live server; return the raw (pre-serialization) result."""
    logger.remove()
    logger.add(sys.stderr, level="INFO")

    repo_cfg = scenario["repository"]
    repo = Repository(
        symbol=repo_cfg["symbol"],
        timespan_multiplier=repo_cfg["timespan_multiplier"],
        timespan_unit=repo_cfg["timespan_unit"],
        indicators=repo_cfg.get("indicators", []),
        history_in_days=repo_cfg.get("history_in_days", 1),
    )

    req = CreatePolygonPlaygroundRequest(
        balance=scenario["balance"],
        start_date=scenario["start_date"],
        stop_date=scenario["end_date"],
        repositories=[repo],
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )

    playground = BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=twirp_host,
    )

    tick_seconds = scenario["tick_seconds"]
    orders_by_tick = {}
    for spec in scenario["orders"]:
        orders_by_tick.setdefault(spec["trigger_tick_index"], []).append(spec)

    order_ids_by_tag = {}
    tick_index = 0
    equity_series = [
        {
            "tick_index": tick_index,
            "timestamp": str(playground.timestamp),
            "balance": playground.account.balance,
            "equity": playground.account.equity,
        }
    ]

    while not playground.is_backtest_complete():
        playground.tick(tick_seconds)
        tick_index += 1
        equity_series.append(
            {
                "tick_index": tick_index,
                "timestamp": str(playground.timestamp),
                "balance": playground.account.balance,
                "equity": playground.account.equity,
            }
        )

        for spec in orders_by_tick.get(tick_index, []):
            close_order_id = None
            if spec.get("closes"):
                close_order_id = order_ids_by_tag[spec["closes"]]

            resp = playground.place_order(
                symbol=scenario["symbol"],
                quantity=spec["quantity"],
                side=_ORDER_SIDES[spec["side"]],
                asset_class=spec["asset_class"],
                tag=spec["tag"],
                client_request_id=spec["client_request_id"],
                close_order_id=close_order_id,
            )
            order_ids_by_tag[spec["tag"]] = resp.id

        if tick_index > MAX_TICKS:
            raise RuntimeError(
                f"model-diff scenario exceeded MAX_TICKS={MAX_TICKS} without completing -- "
                "refusing to run away."
            )

    orders = playground.fetch_orders()

    final_account = {
        "timestamp": str(playground.timestamp),
        "balance": playground.account.balance,
        "equity": playground.account.equity,
        "realized_pnl": playground.get_realized_profit(),
        "positions": {
            symbol: {
                "quantity": pos.quantity,
                "cost_basis": pos.cost_basis,
                "pl": pos.pl,
                "maintenance_margin": pos.maintenance_margin,
                "current_price": pos.current_price,
            }
            for symbol, pos in sorted(playground.account.positions.items())
        },
    }

    return {
        "scenario": scenario,
        "ticks": equity_series,
        "final_account": final_account,
        "orders": [_order_to_dict(o) for o in orders],
    }


def serialize(result: dict) -> str:
    """Canonical, byte-for-byte-stable text form: sorted keys, fixed float formatting."""
    canonical = _round_floats(result)
    return json.dumps(canonical, indent=2, sort_keys=True) + "\n"


def main():
    parser = argparse.ArgumentParser(description="Run the fixed model-diff backtester scenario (MIG-03).")
    parser.add_argument("--scenario", type=Path, default=DEFAULT_SCENARIO_PATH, help="Path to scenario JSON")
    parser.add_argument("--twirp-host", type=str, default="http://127.0.0.1:5051", help="Twirp server URL")
    parser.add_argument("--output", type=Path, default=None, help="Write canonical output here (default: stdout)")
    args = parser.parse_args()

    scenario = load_scenario(args.scenario)
    result = run_scenario(scenario, args.twirp_host)
    text = serialize(result)

    if args.output:
        args.output.write_text(text)
        print(f"Wrote canonical model-diff output to {args.output}", file=sys.stderr)
    else:
        print(text)


if __name__ == "__main__":
    main()
