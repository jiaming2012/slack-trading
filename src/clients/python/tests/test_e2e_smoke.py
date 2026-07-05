#!/usr/bin/env python3
"""
End-to-end Simulation smoke test (e2e-sim-smoke-test).

Answers one question fast and unattended: "does the platform still boot
and complete a real Simulation loop with a real fill?"

This module assumes a Twirp server is already reachable (the orchestration
script `run_e2e_smoke_test.sh` boots and tears it down). It:

  1. Creates a Simulation-mode Playground via ``CreatePolygonPlaygroundRequest``
     for AAPL over a short, fixed 2025 date window known to succeed against
     the Feed (per the CLAUDE.md gotcha that AAPL 2025 dates work).
  2. Drives a bounded tick loop (max-tick count + wall-clock timeout) via the
     minimal ``SmokeStrategy`` driver, which places one deterministic market
     buy order early in the loop.
  3. Asserts that order transitions to ``filled`` before the loop's terminal
     condition is reached.
  4. Tears down (removes the Playground from the server) on both the pass
     path and every failure path.

Pytest's own process exit code (0 pass / nonzero fail) is the pass/fail
signal consumed by the orchestration script.

Configuration (all via environment variables, with sensible defaults):
    E2E_SMOKE_TWIRP_HOST  Twirp server URL          (default http://127.0.0.1:5051)
    E2E_SMOKE_SYMBOL      underlying stock symbol   (default AAPL)
    E2E_SMOKE_START_DATE  playground start date     (default 2025-01-06)
    E2E_SMOKE_STOP_DATE   playground stop date      (default 2025-01-10)
    E2E_SMOKE_BALANCE     initial account balance   (default 100000)
    E2E_SMOKE_QUANTITY    shares to buy             (default 10)
    E2E_SMOKE_MAX_TICKS   max tick-loop iterations  (default 60)
    E2E_SMOKE_TIMEOUT_S   wall-clock timeout (secs) (default 120)
"""

import os
import sys
import time
from pathlib import Path

import pytest
from loguru import logger

# conftest.py already inserts the python client root onto sys.path, but keep
# this belt-and-suspenders insert so the module also runs standalone.
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from engine.client import (  # noqa: E402
    BacktesterPlaygroundClient,
    CreatePolygonPlaygroundRequest,
    PlaygroundEnvironment,
    RepositorySource,
)
from rpc.playground_pb2 import Repository  # noqa: E402
from tests.e2e_smoke_strategy import SmokeStrategy  # noqa: E402


def _cfg(name: str, default: str) -> str:
    return os.environ.get(name, default)


TWIRP_HOST = _cfg("E2E_SMOKE_TWIRP_HOST", "http://127.0.0.1:5051")
SYMBOL = _cfg("E2E_SMOKE_SYMBOL", "AAPL")
START_DATE = _cfg("E2E_SMOKE_START_DATE", "2025-01-06")
STOP_DATE = _cfg("E2E_SMOKE_STOP_DATE", "2025-01-10")
BALANCE = float(_cfg("E2E_SMOKE_BALANCE", "100000"))
QUANTITY = float(_cfg("E2E_SMOKE_QUANTITY", "10"))
MAX_TICKS = int(_cfg("E2E_SMOKE_MAX_TICKS", "60"))
TIMEOUT_S = float(_cfg("E2E_SMOKE_TIMEOUT_S", "120"))

# Simulator hourly candles; one order placed on the first tick.
TICK_SECONDS = 3600
PLACE_AT_TICK = 1


def _create_playground() -> BacktesterPlaygroundClient:
    logger.remove()
    logger.add(sys.stderr, level="INFO")

    repo = Repository(
        symbol=SYMBOL,
        timespan_multiplier=1,
        timespan_unit="hour",
        indicators=[],
        history_in_days=1,
    )
    req = CreatePolygonPlaygroundRequest(
        balance=BALANCE,
        start_date=START_DATE,
        stop_date=STOP_DATE,
        repositories=[repo],
        environment=PlaygroundEnvironment.SIMULATOR.value,
    )
    return BacktesterPlaygroundClient(
        req,
        live_account_type=None,
        source=RepositorySource.POLYGON,
        logger=logger,
        twirp_host=TWIRP_HOST,
    )


def test_e2e_sim_smoke():
    """Boot-adjacent smoke: create sim playground, force a fill, tear down."""
    # --- Playground creation (fail immediately on error / empty id) ---
    playground = _create_playground()
    assert playground.id, "playground creation returned an empty Playground ID"
    assert playground.account is not None, "playground has no initial account state"
    assert playground.account.balance == BALANCE, (
        f"initial balance {playground.account.balance} != requested {BALANCE}"
    )
    logger.info(f"[smoke] created Simulation playground {playground.id} "
                f"balance={playground.account.balance}")

    strategy = SmokeStrategy(playground, symbol=SYMBOL, quantity=QUANTITY,
                             place_at_tick=PLACE_AT_TICK)

    filled = False
    tick_index = 0
    deadline = time.monotonic() + TIMEOUT_S
    hit_timeout = False

    try:
        # --- Bounded tick loop -------------------------------------------
        while not playground.is_backtest_complete():
            if time.monotonic() > deadline:
                hit_timeout = True
                break
            if tick_index >= MAX_TICKS:
                break

            playground.tick(TICK_SECONDS)
            tick_index += 1

            placed = strategy.on_tick(tick_index)
            if placed is not None:
                logger.info(f"[smoke] placed order id={placed} at tick {tick_index}")

            if strategy.is_order_filled():
                filled = True
                logger.info(f"[smoke] order id={strategy.order_id} FILLED at tick {tick_index}")
                break

        # --- Terminal-condition assertions -------------------------------
        assert not hit_timeout, (
            f"smoke test exceeded wall-clock timeout of {TIMEOUT_S}s at tick "
            f"{tick_index} without the order filling (order status="
            f"{strategy.order_status()})"
        )
        assert strategy.order_id is not None, (
            f"tick loop ended (tick {tick_index}) before the deterministic "
            f"order was ever placed (place_at_tick={PLACE_AT_TICK}, "
            f"max_ticks={MAX_TICKS})"
        )
        assert filled, (
            f"order id={strategy.order_id} never reached 'filled' status before "
            f"the tick loop's terminal condition (tick {tick_index}/{MAX_TICKS}, "
            f"backtest_complete={playground.is_backtest_complete()}); "
            f"last status={strategy.order_status()}"
        )
    finally:
        # --- Teardown on BOTH pass and every failure path ----------------
        try:
            playground.remove_from_server()
            logger.info(f"[smoke] removed playground {playground.id} from server")
        except Exception as exc:  # pragma: no cover - teardown best-effort
            logger.warning(f"[smoke] teardown: failed to remove playground "
                           f"{playground.id}: {exc}")


if __name__ == "__main__":
    # Allow direct invocation: `python -m tests.test_e2e_smoke`.
    raise SystemExit(pytest.main([__file__, "-v", "-s"]))
