"""Tests for mean_reversion_report module."""

from unittest.mock import MagicMock, patch

import pandas as pd
import pytest

from tools.mean_reversion_report import (
    build_group_summary,
    build_order_rows,
    compute_metrics,
    generate_mean_reversion_report,
)


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_order(
    order_id=1,
    symbol="AAPL",
    side="buy",
    quantity=100,
    price=99.0,
    requested_price=99.0,
    status="filled",
    create_date="2025-06-15T10:00:00Z",
    trades=None,
    pl=None,
    attributes=None,
):
    """Create a mock protobuf Order."""
    order = MagicMock()
    order.id = order_id
    order.symbol = symbol
    order.side = side
    order.quantity = quantity
    order.price = price
    order.requested_price = requested_price
    order.status = status
    order.create_date = create_date

    if trades is None:
        trade = MagicMock()
        trade.price = price
        order.trades = [trade]
    else:
        order.trades = trades

    if pl is not None:
        order.pl = pl
        order.HasField = lambda f: f == "pl"
    else:
        order.pl = 0.0
        order.HasField = lambda f: False

    order.attributes = attributes or {}
    return order


def _make_entry_order(
    order_id=1,
    group_id="g1",
    quantity=100,
    price=99.0,
    signal_price=100.0,
    stop_price=95.0,
    p_revert=0.6,
    sigma_distance=0.5,
    expected_profit=None,
    model_name="empirical",
    level_index=0,
):
    """Create a mock entry order with standard attributes."""
    if expected_profit is None:
        expected_exit = signal_price * p_revert + stop_price * (1 - p_revert)
        expected_profit = quantity * (expected_exit - price)

    return _make_order(
        order_id=order_id,
        quantity=quantity,
        price=price,
        side="buy",
        attributes={
            "group_id": group_id,
            "action": "entry",
            "htf_signal": "bullish_supertrend|stochrsi_cross_above_20",
            "signal_price": str(signal_price),
            "stop_price": str(stop_price),
            "p_revert": str(p_revert),
            "sigma_distance": str(sigma_distance),
            "expected_profit": f"{expected_profit:.2f}",
            "model_name": model_name,
            "level_index": str(level_index),
        },
    )


def _make_exit_order(
    order_id=2,
    group_id="g1",
    quantity=50,
    price=100.0,
    exit_tier=0,
    source_level=0,
    pl=None,
):
    """Create a mock partial exit order with server-computed P&L."""
    return _make_order(
        order_id=order_id,
        quantity=quantity,
        price=price,
        side="sell",
        pl=pl,
        attributes={
            "group_id": group_id,
            "action": "partial_exit",
            "exit_tier": str(exit_tier),
            "level_index": str(source_level),
        },
    )


def _make_stop_order(
    order_id=3,
    group_id="g1",
    quantity=100,
    price=95.0,
    stop_price=95.0,
    pl=None,
):
    """Create a mock stop-out order with server-computed P&L."""
    return _make_order(
        order_id=order_id,
        quantity=quantity,
        price=price,
        side="sell",
        pl=pl,
        attributes={
            "group_id": group_id,
            "action": "stop_out",
            "stop_price": str(stop_price),
            "htf_close": str(price),
        },
    )


# ------------------------------------------------------------------ #
# TestBuildOrderRows
# ------------------------------------------------------------------ #

class TestBuildOrderRows:

    def test_entry_rows_created(self):
        """Entry orders → entry rows with expected_pl."""
        orders = [
            _make_entry_order(order_id=1, level_index=0, quantity=100),
            _make_entry_order(order_id=2, level_index=1, quantity=200, price=98.0),
        ]
        df = build_order_rows(orders)

        entries = df[df["action"] == "entry"]
        assert len(entries) == 2
        assert list(entries["quantity"]) == [100, 200]
        assert all(entries["side"] == "buy")
        assert all(entries["expected_pl"].notna())

    def test_exit_rows_have_realized_pl(self):
        """Exit orders with server pl → realized_pl populated."""
        orders = [
            _make_exit_order(order_id=1, quantity=50, price=100.0, pl=50.0),
            _make_exit_order(order_id=2, quantity=50, price=99.50, pl=25.0),
        ]
        df = build_order_rows(orders)

        exits = df[df["action"] == "partial_exit"]
        assert len(exits) == 2
        assert all(exits["side"] == "sell")
        assert list(exits["realized_pl"]) == [50.0, 25.0]

    def test_entry_has_no_realized_pl(self):
        """Entry orders → realized_pl is None (not yet closed)."""
        orders = [_make_entry_order()]
        df = build_order_rows(orders)
        assert df.iloc[0]["realized_pl"] is None

    def test_exit_has_no_expected_pl(self):
        """Exit orders → expected_pl is None (only on entries)."""
        orders = [_make_exit_order(pl=50.0)]
        df = build_order_rows(orders)
        assert df.iloc[0]["expected_pl"] is None

    def test_stop_out_row_created(self):
        """Stop-out order → stop_out row."""
        orders = [_make_stop_order(quantity=100, pl=-400.0)]
        df = build_order_rows(orders)

        stops = df[df["action"] == "stop_out"]
        assert len(stops) == 1
        assert stops.iloc[0]["quantity"] == 100
        assert stops.iloc[0]["realized_pl"] == -400.0

    def test_non_strategy_orders_ignored(self):
        """Orders without 'action' attribute are excluded."""
        orders = [
            _make_order(attributes={}),  # no action
            _make_entry_order(),          # has action
        ]
        df = build_order_rows(orders)
        assert len(df) == 1

    def test_empty_orders(self):
        """No orders → empty DataFrame with correct columns."""
        df = build_order_rows([])
        assert len(df) == 0
        assert "group_id" in df.columns
        assert "expected_pl" in df.columns
        assert "realized_pl" in df.columns

    def test_report_columns_present(self):
        """All expected columns are present."""
        orders = [_make_entry_order()]
        df = build_order_rows(orders)

        expected_cols = {
            "order_id", "group_id", "htf_signal", "signal_price", "action",
            "symbol", "side", "quantity", "fill_price", "requested_price",
            "sigma_distance", "p_revert_unbounded", "p_revert_bounded",
            "expected_pl", "expected_pl_bucket", "realized_pl",
            "duration_min", "model_name", "stop_price", "level_index",
            "exit_tier", "status", "timestamp",
        }
        assert expected_cols.issubset(set(df.columns))

    def test_expected_pl_from_attributes(self):
        """Expected P&L read from order attribute matches formula."""
        # level 0: price=99, p_revert=0.6, signal=100, stop=95
        # expected_exit = 100*0.6 + 95*0.4 = 98
        # expected_pl = 100 * (98 - 99) = -100
        orders = [_make_entry_order(quantity=100, price=99.0, p_revert=0.6)]
        df = build_order_rows(orders)

        entry = df.iloc[0]
        assert abs(entry["expected_pl"] - (-100.0)) < 0.01

    def test_fill_price_from_trades(self):
        """Fill price comes from trade, not order.price."""
        trade = MagicMock()
        trade.price = 99.50
        order = _make_order(
            price=99.0,
            trades=[trade],
            attributes={"action": "entry", "group_id": "g1"},
        )
        df = build_order_rows([order])
        assert df.iloc[0]["fill_price"] == 99.50


# ------------------------------------------------------------------ #
# TestBuildGroupSummary
# ------------------------------------------------------------------ #

class TestBuildGroupSummary:

    def test_group_summary_fields(self):
        """Summary has correct fields."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=100.0),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        assert len(df) == 1
        row = df.iloc[0]
        assert row["group_id"] == "g1"
        assert row["num_entries"] == 1
        assert row["num_exits"] == 1

    def test_realized_pl_from_server(self):
        """Realized P&L uses server's authoritative pl field."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100, price=99.0),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=100.0),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        row = df.iloc[0]
        assert abs(row["total_realized_pl"] - 100.0) < 0.01

    def test_realized_pl_stop_out(self):
        """Stopped-out group: realized P&L from server pl."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100, price=99.0),
            _make_stop_order(order_id=2, group_id="g1", quantity=100, price=95.0, pl=-400.0),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        row = df.iloc[0]
        assert abs(row["total_realized_pl"] - (-400.0)) < 0.01
        assert row["status"] == "stopped_out"

    def test_prediction_error_sign(self):
        """Stopped-out: negative realized, negative expected → correct error."""
        orders = [
            _make_entry_order(
                order_id=1, group_id="g1", quantity=100, price=99.0,
                p_revert=0.6, signal_price=100.0, stop_price=95.0,
            ),
            _make_stop_order(order_id=2, group_id="g1", quantity=100, price=95.0, pl=-400.0),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        row = df.iloc[0]
        # expected_pl = -100 (from entry tag)
        # realized_pl = -400 (from server)
        # error = -400 - (-100) = -300
        assert row["prediction_error"] < 0
        assert abs(row["prediction_error"] - (-300.0)) < 0.01

    def test_multiple_groups(self):
        """Multiple groups → multiple rows."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100),
            _make_entry_order(order_id=2, group_id="g2", quantity=200),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)
        assert len(df) == 2

    def test_empty_orders(self):
        """Empty orders → empty DataFrame."""
        order_df = build_order_rows([])
        df = build_group_summary(order_df)
        assert len(df) == 0
        assert "group_id" in df.columns

    def test_status_inferred_from_orders(self):
        """Group status is inferred: closed, stopped_out, active."""
        # Closed group (has exit but no stop)
        orders_closed = [
            _make_entry_order(order_id=1, group_id="g1"),
            _make_exit_order(order_id=2, group_id="g1", pl=50.0),
        ]
        df = build_group_summary(build_order_rows(orders_closed))
        assert df.iloc[0]["status"] == "closed"

        # Active group (entry only)
        orders_active = [_make_entry_order(order_id=3, group_id="g2")]
        df = build_group_summary(build_order_rows(orders_active))
        assert df.iloc[0]["status"] == "active"

    def test_multiple_exits_sum_realized_pl(self):
        """Multiple exit orders → realized P&L is sum of server pl values."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=300, price=99.0),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=100.0),
            _make_exit_order(order_id=3, group_id="g1", quantity=100, price=100.5, pl=150.0, exit_tier=1),
            _make_exit_order(order_id=4, group_id="g1", quantity=100, price=101.0, pl=200.0, exit_tier=2),
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        # 100 + 150 + 200 = 450
        assert abs(df.iloc[0]["total_realized_pl"] - 450.0) < 0.01

    def test_no_server_pl_falls_back_to_zero(self):
        """Exits without server pl → realized P&L is 0."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100),
            _make_exit_order(order_id=2, group_id="g1", quantity=100),  # no pl
        ]
        order_df = build_order_rows(orders)
        df = build_group_summary(order_df)

        assert df.iloc[0]["total_realized_pl"] == 0.0


# ------------------------------------------------------------------ #
# TestComputeMetrics
# ------------------------------------------------------------------ #

class TestComputeMetrics:

    def test_mae_calculation(self):
        """MAE = mean(abs(expected - realized))."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100, price=99.0),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=100.0),
        ]
        summary = build_group_summary(build_order_rows(orders))
        metrics = compute_metrics(summary)

        assert metrics.mae >= 0
        assert metrics.num_groups == 1

    def test_directional_accuracy_both_positive(self):
        """Both expected and realized positive → accuracy = 1.0."""
        # Buy at 90, expect positive (high p_revert), sell at 100
        orders = [
            _make_entry_order(
                order_id=1, group_id="g1", quantity=100, price=90.0,
                signal_price=100.0, stop_price=85.0, p_revert=0.9,
            ),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=1000.0),
        ]
        summary = build_group_summary(build_order_rows(orders))
        metrics = compute_metrics(summary)

        assert summary.iloc[0]["total_expected_pl"] > 0
        assert summary.iloc[0]["total_realized_pl"] > 0
        assert metrics.directional_accuracy == 1.0

    def test_empty_summary(self):
        """Empty summary → zero metrics."""
        summary = pd.DataFrame(columns=[
            "group_id", "htf_signal", "status",
            "num_entries", "num_exits",
            "total_expected_pl", "total_realized_pl",
            "prediction_error", "prediction_accuracy",
        ])
        metrics = compute_metrics(summary)

        assert metrics.mae == 0.0
        assert metrics.num_groups == 0

    def test_totals_sum_correctly(self):
        """Total expected/realized are sums across groups."""
        orders = [
            _make_entry_order(order_id=1, group_id="g1", quantity=100, price=99.0),
            _make_exit_order(order_id=2, group_id="g1", quantity=100, price=100.0, pl=100.0),
            _make_entry_order(order_id=3, group_id="g2", quantity=100, price=99.0),
            _make_stop_order(order_id=4, group_id="g2", quantity=100, price=95.0, pl=-400.0),
        ]
        summary = build_group_summary(build_order_rows(orders))
        metrics = compute_metrics(summary)

        assert metrics.num_groups == 2
        assert abs(metrics.total_expected_pl - summary["total_expected_pl"].sum()) < 0.01
        assert abs(metrics.total_realized_pl - summary["total_realized_pl"].sum()) < 0.01


# ------------------------------------------------------------------ #
# TestGenerateReport
# ------------------------------------------------------------------ #

class TestGenerateReport:

    @patch("tools.mean_reversion_report.fetch_orders")
    def test_returns_three_elements(self, mock_fetch):
        """generate_mean_reversion_report returns (order_df, group_df, metrics)."""
        mock_fetch.return_value = [_make_entry_order()]
        order_df, group_df, metrics = generate_mean_reversion_report("pg-123")

        assert len(order_df) > 0
        assert len(group_df) > 0
        assert metrics.num_groups == 1

    @patch("tools.mean_reversion_report.fetch_orders")
    def test_saves_csv(self, mock_fetch, tmp_path):
        """Output path → CSV file created."""
        mock_fetch.return_value = [_make_entry_order()]
        csv_path = str(tmp_path / "report.csv")

        generate_mean_reversion_report("pg-123", output_path=csv_path)

        saved = pd.read_csv(csv_path)
        assert len(saved) > 0
        assert "expected_pl" in saved.columns
        assert "realized_pl" in saved.columns

    @patch("tools.mean_reversion_report.fetch_orders")
    def test_empty_orders_no_crash(self, mock_fetch):
        """No orders → empty results, no crash."""
        mock_fetch.return_value = []
        order_df, group_df, metrics = generate_mean_reversion_report("pg-123")

        assert len(order_df) == 0
        assert len(group_df) == 0
        assert metrics.num_groups == 0
