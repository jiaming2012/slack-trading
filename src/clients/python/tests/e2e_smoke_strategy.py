#!/usr/bin/env python3
"""
Minimal "smoke strategy" driver (e2e-sim-smoke-test).

This module exists to keep the end-to-end smoke test's runtime and
pass/fail condition bounded and deterministic. Unlike the covered-call
or credit-spread strategies, it does NOT wait on a market-data-dependent
signal to fire (which takes an unpredictable number of ticks, or may not
fire at all on a short window). Instead it places exactly one deterministic
market buy order for the configured underlying stock at a fixed tick index,
then reports whether that order has reached ``filled`` status.

It exercises the identical ``PlaceOrder`` -> Simulated Broker -> fill ->
account-update path that any real strategy uses, but with timing fully
under the harness's control.

The class is intentionally tiny and side-effect-only: it owns no tick
loop. The test module (`test_e2e_smoke.py`) drives the loop and calls
``on_tick`` / ``is_order_filled`` at the appropriate points.
"""

from engine.types import OrderSide

# Terminal fill status reported by the Go server (see
# src/go/backtester/models/backtester_order_status.go: OrderRecordStatusFilled).
FILLED_STATUS = "filled"


class SmokeStrategy:
    """Places one deterministic market buy order and tracks its fill state.

    Args:
        playground:  a ready ``BacktesterPlaygroundClient``.
        symbol:      underlying stock symbol to trade (e.g. "AAPL").
        quantity:    number of shares to buy (must be > 0).
        place_at_tick: the 1-based tick index at which the single order is
                       placed. Defaults to 1 so the order is placed and can
                       fill as early as possible, keeping the loop short.
    """

    def __init__(self, playground, symbol: str, quantity: float, place_at_tick: int = 1):
        if quantity <= 0:
            raise ValueError(f"SmokeStrategy quantity must be > 0, got {quantity}")
        self.playground = playground
        self.symbol = symbol
        self.quantity = quantity
        self.place_at_tick = place_at_tick
        self.order_id = None
        self.client_request_id = "e2e-smoke-order-1"

    def on_tick(self, tick_index: int):
        """Place the single deterministic order when ``place_at_tick`` is reached.

        Idempotent: the order is placed at most once for the lifetime of the
        strategy, even if called repeatedly at or after ``place_at_tick``.
        Returns the order id if an order was placed on this call, else None.
        """
        if self.order_id is not None:
            return None
        if tick_index < self.place_at_tick:
            return None

        resp = self.playground.place_order(
            symbol=self.symbol,
            quantity=self.quantity,
            side=OrderSide.BUY,
            asset_class="equity",
            tag="e2e-smoke",
            client_request_id=self.client_request_id,
        )
        self.order_id = resp.id
        return self.order_id

    def is_order_filled(self) -> bool:
        """Return True once the placed order has reached ``filled`` status.

        Returns False if no order has been placed yet, or the order is still
        pending. Queries the server for authoritative order state each call.
        """
        if self.order_id is None:
            return False
        for order in self.playground.fetch_orders():
            if order.id == self.order_id:
                return order.status == FILLED_STATUS
        return False

    def order_status(self) -> str:
        """Return the current server-side status string for the placed order.

        Returns "<not-placed>" if no order has been placed, or "<unknown>"
        if the order id is not found on the server. Used for diagnostics in
        failure messages.
        """
        if self.order_id is None:
            return "<not-placed>"
        for order in self.playground.fetch_orders():
            if order.id == self.order_id:
                return order.status
        return "<unknown>"
