"""Tests for mean_reversion_report module."""

from datetime import datetime

import pytest

from deviation_levels import DeviationLevel, DeviationPlan
from mean_reversion_strategy import TradeGroup
from partial_exit_manager import ExitPlan, ExitTier
from mean_reversion_report import (
    build_group_summary,
    build_order_rows,
    compute_metrics,
    generate_mean_reversion_report,
)


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_group(
    group_id="g1",
    signal_price=100.0,
    stop_price=95.0,
    status="closed",
    levels=None,
    filled=None,
    exit_plan=None,
    triggered_exits=None,
):
    """Create a TradeGroup for testing."""
    if levels is None:
        levels = [
            DeviationLevel(price=99.0, shares=100, sigma_distance=0.5, p_revert=0.6),
            DeviationLevel(price=98.0, shares=200, sigma_distance=1.0, p_revert=0.7),
        ]
    plan = DeviationPlan(
        levels=levels,
        signal_price=signal_price,
        stop_price=stop_price,
        max_potential_loss=1500.0,
        was_truncated=False,
        original_level_count=len(levels),
    )
    group = TradeGroup(
        group_id=group_id,
        htf_signal_key="bullish_supertrend|stochrsi_cross_above_20",
        htf_signal_price=signal_price,
        htf_signal_timestamp=datetime(2025, 6, 15, 10, 0),
        deviation_plan=plan,
        stop_price=stop_price,
        status=status,
    )
    if filled is not None:
        group.filled_levels = filled
    if exit_plan is not None:
        group.exit_plan = exit_plan
    if triggered_exits is not None:
        group.triggered_exits = triggered_exits
    return group


# ------------------------------------------------------------------ #
# TestBuildOrderRows
# ------------------------------------------------------------------ #

class TestBuildOrderRows:

    def test_entry_rows_created(self):
        """Filled levels → entry rows with expected_profit."""
        group = _make_group(filled={0: 100, 1: 200})
        df = build_order_rows([group])

        entries = df[df["action"] == "entry"]
        assert len(entries) == 2
        assert list(entries["quantity"]) == [100, 200]
        assert all(entries["side"] == "BUY")
        assert all(entries["expected_profit"].notna())

    def test_exit_rows_created(self):
        """Triggered exit tiers → exit rows."""
        group = _make_group(
            filled={0: 100},
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=99.50, shares_to_sell=50, source_level_index=0, tier_index=0),
                ExitTier(exit_price=100.0, shares_to_sell=50, source_level_index=0, tier_index=1),
            ]),
            triggered_exits={(0, 0), (0, 1)},
        )
        df = build_order_rows([group])

        exits = df[df["action"] == "partial_exit"]
        assert len(exits) == 2
        assert all(exits["side"] == "SELL")

    def test_stop_out_row_created(self):
        """Stopped-out group → stop_out row."""
        group = _make_group(
            filled={0: 100},
            status="stopped_out",
        )
        df = build_order_rows([group])

        stops = df[df["action"] == "stop_out"]
        assert len(stops) == 1
        assert stops.iloc[0]["quantity"] == 100

    def test_stop_out_subtracts_exited(self):
        """Stop-out quantity excludes already-exited shares."""
        group = _make_group(
            filled={0: 100},
            status="stopped_out",
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=99.50, shares_to_sell=40, source_level_index=0, tier_index=0),
                ExitTier(exit_price=100.0, shares_to_sell=60, source_level_index=0, tier_index=1),
            ]),
            triggered_exits={(0, 0)},  # 40 shares exited
        )
        df = build_order_rows([group])

        stops = df[df["action"] == "stop_out"]
        assert stops.iloc[0]["quantity"] == 60  # 100 - 40

    def test_empty_groups(self):
        """No groups → empty DataFrame with correct columns."""
        df = build_order_rows([])
        assert len(df) == 0
        assert "group_id" in df.columns
        assert "expected_profit" in df.columns

    def test_report_columns_present(self):
        """All expected columns are present."""
        group = _make_group(filled={0: 100})
        df = build_order_rows([group])

        expected_cols = {
            "group_id", "htf_signal", "signal_price", "action", "side",
            "quantity", "price", "sigma_distance", "p_revert",
            "expected_profit", "model_name", "stop_price", "level_index",
            "was_truncated", "timestamp",
        }
        assert expected_cols.issubset(set(df.columns))

    def test_expected_profit_formula(self):
        """Expected profit = shares * (E[exit] - entry)."""
        group = _make_group(
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},
        )
        df = build_order_rows([group])

        entry = df[df["action"] == "entry"].iloc[0]
        # level 0: price=99, p_revert=0.6
        expected_exit = 100.0 * 0.6 + 95.0 * 0.4  # 60 + 38 = 98
        expected_profit = 100 * (98.0 - 99.0)  # -100
        assert abs(entry["expected_profit"] - expected_profit) < 0.01


# ------------------------------------------------------------------ #
# TestBuildGroupSummary
# ------------------------------------------------------------------ #

class TestBuildGroupSummary:

    def test_group_summary_fields(self):
        """Summary has correct fields."""
        group = _make_group(
            filled={0: 100},
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=100.0, shares_to_sell=100, source_level_index=0, tier_index=0),
            ]),
            triggered_exits={(0, 0)},
        )
        df = build_group_summary([group])

        assert len(df) == 1
        row = df.iloc[0]
        assert row["group_id"] == "g1"
        assert row["status"] == "closed"
        assert row["levels_planned"] == 2
        assert row["levels_filled"] == 1

    def test_realized_profit_from_exits(self):
        """Realized profit computed from exit price - entry price."""
        group = _make_group(
            signal_price=100.0,
            filled={0: 100},  # entry at 99.0
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=100.0, shares_to_sell=100, source_level_index=0, tier_index=0),
            ]),
            triggered_exits={(0, 0)},
        )
        df = build_group_summary([group])

        row = df.iloc[0]
        # 100 shares * (100.0 - 99.0) = 100.0
        assert abs(row["total_realized_profit"] - 100.0) < 0.01

    def test_realized_profit_stop_out(self):
        """Stopped-out group: remaining shares at stop price."""
        group = _make_group(
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},  # entry at 99.0
            status="stopped_out",
        )
        df = build_group_summary([group])

        row = df.iloc[0]
        # 100 shares * (95.0 - 99.0) = -400.0
        assert abs(row["total_realized_profit"] - (-400.0)) < 0.01

    def test_prediction_error_sign(self):
        """Stopped-out group: negative realized, positive expected → correct error."""
        group = _make_group(
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},  # entry at 99.0, p_revert=0.6
            status="stopped_out",
        )
        df = build_group_summary([group])

        row = df.iloc[0]
        # expected = 100 * (98.0 - 99.0) = -100
        # realized = 100 * (95.0 - 99.0) = -400
        # error = realized - expected = -400 - (-100) = -300
        assert row["prediction_error"] < 0
        assert abs(row["prediction_error"] - (-300.0)) < 0.01

    def test_multiple_groups(self):
        """Multiple groups → multiple rows."""
        g1 = _make_group(group_id="g1", filled={0: 100})
        g2 = _make_group(group_id="g2", filled={0: 200})
        df = build_group_summary([g1, g2])

        assert len(df) == 2

    def test_unfilled_group_excluded(self):
        """Group with no fills → excluded from summary."""
        group = _make_group(filled={})
        df = build_group_summary([group])

        assert len(df) == 0

    def test_empty_groups(self):
        """No groups → empty DataFrame."""
        df = build_group_summary([])
        assert len(df) == 0
        assert "group_id" in df.columns


# ------------------------------------------------------------------ #
# TestComputeMetrics
# ------------------------------------------------------------------ #

class TestComputeMetrics:

    def test_mae_calculation(self):
        """MAE = mean(abs(expected - realized))."""
        g1 = _make_group(
            group_id="g1",
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=100.0, shares_to_sell=100, source_level_index=0, tier_index=0),
            ]),
            triggered_exits={(0, 0)},
        )
        summary = build_group_summary([g1])
        metrics = compute_metrics(summary)

        assert metrics.mae >= 0
        assert metrics.num_groups == 1

    def test_directional_accuracy_both_positive(self):
        """Both expected and realized positive → accuracy = 1.0."""
        # Build a group where exit is at signal price (fully reverted)
        group = _make_group(
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},
            levels=[
                DeviationLevel(price=90.0, shares=100, sigma_distance=1.0, p_revert=0.9),
            ],
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=100.0, shares_to_sell=100, source_level_index=0, tier_index=0),
            ]),
            triggered_exits={(0, 0)},
        )
        summary = build_group_summary([group])
        metrics = compute_metrics(summary)

        # Both expected and realized should be positive (bought at 90, sold at 100)
        assert summary.iloc[0]["total_expected_profit"] > 0
        assert summary.iloc[0]["total_realized_profit"] > 0
        assert metrics.directional_accuracy == 1.0

    def test_empty_summary(self):
        """Empty summary → zero metrics."""
        import pandas as pd
        summary = pd.DataFrame(columns=[
            "group_id", "htf_signal", "status", "levels_planned",
            "levels_filled", "total_expected_profit", "total_realized_profit",
            "prediction_error", "prediction_accuracy",
        ])
        metrics = compute_metrics(summary)

        assert metrics.mae == 0.0
        assert metrics.num_groups == 0

    def test_totals_sum_correctly(self):
        """Total expected/realized are sums across groups."""
        g1 = _make_group(
            group_id="g1",
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},
            exit_plan=ExitPlan(tiers=[
                ExitTier(exit_price=100.0, shares_to_sell=100, source_level_index=0, tier_index=0),
            ]),
            triggered_exits={(0, 0)},
        )
        g2 = _make_group(
            group_id="g2",
            signal_price=100.0,
            stop_price=95.0,
            filled={0: 100},
            status="stopped_out",
        )
        summary = build_group_summary([g1, g2])
        metrics = compute_metrics(summary)

        assert metrics.num_groups == 2
        assert abs(metrics.total_expected - summary["total_expected_profit"].sum()) < 0.01
        assert abs(metrics.total_realized - summary["total_realized_profit"].sum()) < 0.01


# ------------------------------------------------------------------ #
# TestGenerateReport
# ------------------------------------------------------------------ #

class TestGenerateReport:

    def test_returns_three_elements(self):
        """generate_mean_reversion_report returns (order_df, group_df, metrics)."""
        group = _make_group(filled={0: 100})
        order_df, group_df, metrics = generate_mean_reversion_report([group])

        assert len(order_df) > 0
        assert len(group_df) > 0
        assert metrics.num_groups == 1

    def test_saves_csv(self, tmp_path):
        """Output path → CSV file created."""
        group = _make_group(filled={0: 100})
        csv_path = str(tmp_path / "report.csv")

        generate_mean_reversion_report([group], output_path=csv_path)

        import pandas as pd
        saved = pd.read_csv(csv_path)
        assert len(saved) > 0

    def test_empty_groups_no_crash(self):
        """No groups → empty results, no crash."""
        order_df, group_df, metrics = generate_mean_reversion_report([])

        assert len(order_df) == 0
        assert len(group_df) == 0
        assert metrics.num_groups == 0
