"""
E2E integration test: BaseStrategy subclass with signal decisions,
order placement, mock fill, and Grafana dashboard metric verification.

Uses raw Twirp RPC client to create a live mock playground (avoids
Polygon API calls during market close), then drives a BaseStrategy
subclass through a manual tick loop.

Run:
    PYTHONPATH=src/clients/python RUN_E2E_PRODUCTION=1 \
    OTEL_EXPORTER_OTLP_ENDPOINT=http://159.89.226.131:4318 \
    /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest \
    src/clients/python/tests/test_strategy_e2e.py -v -s

Configure:
    TWIRP_HOST       (default: http://159.89.226.131:5051)
    GRAFANA_HOST     (default: http://159.89.226.131:3000)
    GRAFANA_USER     (default: admin)
    GRAFANA_PASSWORD (default: grodt2026)
"""
import os
import time
import uuid

import pytest
import requests
from loguru import logger

from engine.otel import setup_otel
from engine.heartbeat import StrategyHeartbeat
from engine.types import SignalDecision, OrderSide
from strategies.base_strategy import BaseStrategy
from twirp.context import Context
from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import (
    CreateLivePlaygroundRequest,
    Repository,
    PlaceOrderRequest,
    GetAccountRequest,
    MockFillOrderRequest,
    NextTickRequest,
)

TWIRP_HOST = os.getenv("TWIRP_HOST", "http://159.89.226.131:5051")
GRAFANA_HOST = os.getenv("GRAFANA_HOST", "http://159.89.226.131:3000")
GRAFANA_USER = os.getenv("GRAFANA_USER", "admin")
GRAFANA_PASSWORD = os.getenv("GRAFANA_PASSWORD", "grodt2026")


def query_prometheus(query: str) -> str:
    """Query Prometheus via Grafana proxy. Returns first result value as string."""
    url = f"{GRAFANA_HOST}/api/datasources/proxy/uid/prometheus/api/v1/query"
    resp = requests.get(url, params={"query": query}, auth=(GRAFANA_USER, GRAFANA_PASSWORD))
    resp.raise_for_status()
    data = resp.json()
    results = data.get("data", {}).get("result", [])
    if not results:
        return ""
    return str(results[0]["value"][1])


class PlaygroundProxy:
    """Minimal wrapper around Twirp RPC for strategy compatibility.

    Provides just enough interface for BaseStrategy to work:
    id, place_order, is_backtest_complete.
    """

    def __init__(self, playground_id: str, client: PlaygroundServiceClient):
        self.id = playground_id
        self._client = client
        self._complete = False

    def place_order(self, symbol, side, qty, order_type, duration, price, asset_class):
        req = PlaceOrderRequest(
            playground_id=self.id,
            symbol=symbol,
            asset_class=asset_class,
            quantity=qty,
            side=side if isinstance(side, str) else side.value,
            type=order_type,
            duration=duration,
            requested_price=price,
            client_request_id=str(uuid.uuid4()),
        )
        return self._client.PlaceOrder(ctx=Context(), request=req)

    def is_backtest_complete(self):
        return self._complete


class SimpleBuyStrategy(BaseStrategy):
    """Minimal strategy: buys on tick 1, records signal decisions on every tick."""

    def __init__(self, playground, symbol: str, max_ticks: int = 3, **kwargs):
        super().__init__(playground, symbol, logger=logger, **kwargs)
        self.tick_count = 0
        self.order_placed = False
        self.max_ticks = max_ticks

    def on_tick(self, tick_deltas) -> None:
        self.tick_count += 1

        if not self.order_placed:
            self.playground.place_order(
                symbol=self.symbol,
                side="buy",
                qty=3,
                order_type="market",
                duration="day",
                price=250.0,
                asset_class="equity",
            )
            self.order_placed = True
            self.record_decision(SignalDecision(
                signal_type="simple_buy",
                direction="long",
                decision="place",
                reason="first tick — placing test order",
                symbol=self.symbol,
                playground_id="",
            ))
            logger.info(f"Tick {self.tick_count}: BUY order placed")
        else:
            self.record_decision(SignalDecision(
                signal_type="simple_buy",
                direction="neutral",
                decision="skip",
                reason="order already placed",
                symbol=self.symbol,
                playground_id="",
            ))
            logger.info(f"Tick {self.tick_count}: skip (order already placed)")

    def get_next_tick_seconds(self) -> int:
        return 1

    def should_fetch_account(self) -> bool:
        return True

    def is_complete(self) -> bool:
        return self.tick_count >= self.max_ticks


@pytest.mark.skipif(
    os.getenv("RUN_E2E_PRODUCTION") is None,
    reason="Set RUN_E2E_PRODUCTION=1 to run production E2E tests",
)
class TestStrategyE2E:

    def test_strategy_ticks_signals_and_dashboard(self):
        """BaseStrategy subclass receives ticks, places order, fills, dashboard updates."""

        client = PlaygroundServiceClient(TWIRP_HOST, timeout=60)
        client_id = f"e2e-strategy-{uuid.uuid4().hex[:8]}"

        # --- Baseline metrics ---
        # Sum across all playground_id labels to get true total
        baseline_orders = query_prometheus("sum(grodt_orders_placed_total)")
        logger.info(f"Baseline orders metric: {baseline_orders}")

        # --- Create live mock playground ---
        create_resp = client.CreateLivePlayground(ctx=Context(), request=CreateLivePlaygroundRequest(
            client_id=client_id,
            balance=50000,
            broker="tradier",
            account_type="mock",
            repositories=[Repository(
                symbol="AAPL",
                timespan_multiplier=1,
                timespan_unit="minute",
                indicators=[],
                history_in_days=0,
            )],
            environment="live",
        )
        )

        pg_id = create_resp.id
        logger.info(f"Playground created: {pg_id} (client_id: {client_id})")

        # --- Wrap in proxy for BaseStrategy compatibility ---
        proxy = PlaygroundProxy(pg_id, client)
        strategy = SimpleBuyStrategy(proxy, "AAPL", max_ticks=3)

        # --- Init OTel + heartbeat ---
        otel_shutdown = setup_otel(service_name="grodt-strategy-e2e")
        heartbeat = StrategyHeartbeat("SimpleBuyStrategy")
        heartbeat.start()
        heartbeat.set_state("active")

        # --- Run strategy tick loop ---
        try:
            while not strategy.is_complete():
                strategy.on_tick(None)  # tick_deltas not used by this strategy
                strategy._flush_decisions()
                heartbeat.record_tick()
        finally:
            heartbeat.set_state("idle")
            heartbeat.stop()
            if otel_shutdown:
                otel_shutdown()

        # --- Verify strategy state ---
        assert strategy.tick_count == 3
        assert strategy.order_placed
        logger.info(f"Strategy ran {strategy.tick_count} ticks, order_placed={strategy.order_placed}")

        # --- Verify order in playground ---
        account = client.GetAccount(ctx=Context(), request=GetAccountRequest(
            playground_id=pg_id,
            fetch_orders=True,
        ))

        assert len(account.orders) >= 1
        order = account.orders[0]
        assert order.symbol == "AAPL"
        assert order.side == "buy"
        assert order.quantity == 3.0
        assert order.status == "pending"
        logger.info(f"Order verified: {order.symbol} {order.side} qty={order.quantity} status={order.status}")

        reconcile_pg_id = account.meta.reconcile_playground_id
        assert reconcile_pg_id

        # --- Mock fill ---
        reconcile = client.GetAccount(ctx=Context(), request=GetAccountRequest(
            playground_id=reconcile_pg_id,
            fetch_orders=True,
        ))
        assert len(reconcile.orders) >= 1
        # Find the pending order (not a previously filled one)
        pending_orders = [o for o in reconcile.orders if o.status == "pending"]
        assert len(pending_orders) >= 1, f"No pending reconcile orders found"
        reconcile_order = pending_orders[-1]
        assert reconcile_order.external_id > 0
        logger.info(f"Reconcile order: id={reconcile_order.id} ext_id={reconcile_order.external_id}")

        client.MockFillOrder(ctx=Context(), request=MockFillOrderRequest(
            order_id=reconcile_order.external_id,
            price=251.0,
            status="filled",
            broker="tradier",
        ))
        logger.info("Mock fill sent")

        # --- Tick until trade appears ---
        trade_found = False
        for _ in range(30):
            tick_resp = client.NextTick(ctx=Context(), request=NextTickRequest(
                playground_id=pg_id,
                request_id=str(uuid.uuid4()),
            ))
            if len(tick_resp.new_trades) > 0:
                trade_found = True
                break
            time.sleep(1)

        assert trade_found, "Expected trade within 30 seconds"
        logger.info("Trade confirmed")

        # --- Verify filled position ---
        account = client.GetAccount(ctx=Context(), request=GetAccountRequest(
            playground_id=pg_id,
            fetch_orders=True,
        ))
        assert account.orders[0].status == "filled"
        assert "AAPL" in account.positions
        assert account.positions["AAPL"].quantity == 3.0
        logger.info(f"Position verified: AAPL qty={account.positions['AAPL'].quantity}")

        # --- Verify dashboard metrics ---
        logger.info("Waiting 45s for metric export + scrape...")
        time.sleep(45)

        after_orders = query_prometheus("sum(grodt_orders_placed_total)")
        logger.info(f"After orders metric: {after_orders}")
        assert after_orders, "Expected orders metric to have data"

        if baseline_orders:
            assert after_orders != baseline_orders, \
                f"Metric should increment: {baseline_orders} -> {after_orders}"
        logger.info("Dashboard metrics verified")

        logger.info("TEST PASSED: strategy ticked, signals recorded, order filled, dashboard updated")
