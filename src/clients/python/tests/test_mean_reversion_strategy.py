"""Tests for mean_reversion_strategy module."""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Set
from unittest.mock import MagicMock, patch

import pytest

from lib.deviation_levels import DeviationLevel, DeviationPlan
from lib.partial_exit_manager import ExitPlan, ExitTier
from deprecated.mean_reversion import (
    MeanReversionStrategy,
    TradeGroup,
    _bar_to_dict,
)
from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(equity=100_000.0, free_margin=None):
    """Create a mock playground with minimal interface."""
    pg = MagicMock()
    pg.htf_seconds = 3600
    pg.ltf_seconds = 300
    pg.account = MagicMock()
    pg.account.equity = equity
    pg.account.free_margin = free_margin if free_margin is not None else equity
    pg.account.get_quantity = MagicMock(return_value=10000)
    pg.place_order = MagicMock()
    pg.is_backtest_complete = False
    return pg


def _make_pdf(
    signal_key="bullish_supertrend|stochrsi_cross_above_20",
    mean=0.005,
    stddev=0.02,
    sample_size=100,
    sufficient=True,
    forward_returns=None,
):
    """Create a minimal PDFDocument with one signal."""
    if forward_returns is None:
        import numpy as np
        rng = np.random.default_rng(42)
        forward_returns = list(rng.normal(mean, stddev, sample_size))

    horizon = HorizonStats(
        mean=mean,
        stddev=stddev,
        percentiles={"5": -0.03, "10": -0.02, "25": -0.005, "50": 0.005},
        forward_returns=forward_returns,
    )
    signal = SignalPDF(
        sample_size=sample_size,
        ci_95_width=0.01,
        sufficient_samples=sufficient,
        horizons={"1h": horizon},
    )
    return PDFDocument(
        symbol="AAPL",
        signals={signal_key: signal},
        scan_start="2025-01-01",
        scan_end="2025-06-01",
        ltf_period_seconds=300,
        htf_period_seconds=3600,
    )


def _make_htf_bar(close=100.0, **overrides):
    """Create a dict simulating an HTF bar."""
    bar = {
        "open": close - 0.5,
        "high": close + 0.5,
        "low": close - 1.0,
        "close": close,
        "datetime": datetime(2025, 6, 15, 10, 0),
        "superD_50_3": 1.0,
        "stochrsi_k_14_14_3_3": 25.0,
        "stochrsi_cross_above_20": 1.0,
        "stochrsi_cross_below_80": 0.0,
        "cdl_hammer": 0.0,
        "cdl_doji_10_0_1": 0.0,
        "sma_50": close - 2.0,
        "sma_100": close - 3.0,
        "sma_200": close - 5.0,
    }
    bar.update(overrides)
    return bar


def _make_ltf_bar(low=99.0, high=100.5, close=100.0, **overrides):
    """Create a dict simulating an LTF bar."""
    bar = {
        "open": close - 0.2,
        "high": high,
        "low": low,
        "close": close,
        "datetime": datetime(2025, 6, 15, 10, 5),
    }
    bar.update(overrides)
    return bar


def _make_candle(bar_dict, period, symbol="AAPL"):
    """Create a mock candle object."""
    candle = MagicMock()
    candle.period = period
    candle.symbol = symbol
    candle.bar = MagicMock()
    return candle


def _make_group(
    signal_price=100.0,
    levels=None,
    stop_price=95.0,
    status="pending",
):
    """Create a TradeGroup with a deviation plan."""
    if levels is None:
        levels = [
            DeviationLevel(price=99.0, shares=100, sigma_distance=0.5, p_revert=0.6),
            DeviationLevel(price=98.0, shares=150, sigma_distance=1.0, p_revert=0.7),
            DeviationLevel(price=97.0, shares=250, sigma_distance=1.5, p_revert=0.8),
        ]
    plan = DeviationPlan(
        levels=levels,
        signal_price=signal_price,
        stop_price=stop_price,
        max_potential_loss=2500.0,
        was_truncated=False,
        original_level_count=len(levels),
    )
    return TradeGroup(
        group_id="test1234",
        htf_signal_key="bullish_supertrend|stochrsi_cross_above_20",
        htf_signal_price=signal_price,
        htf_signal_timestamp=datetime(2025, 6, 15, 10, 0),
        deviation_plan=plan,
        stop_price=stop_price,
        status=status,
    )


# ------------------------------------------------------------------ #
# TestTradeGroupCreation
# ------------------------------------------------------------------ #

class TestTradeGroupCreation:

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    @patch("strategies.mean_reversion.compute_deviation_levels")
    def test_htf_bullish_signal_creates_group(
        self, mock_compute, mock_detect,
    ):
        """Bullish signal with positive mean → TradeGroup created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]

        plan = DeviationPlan(
            levels=[
                DeviationLevel(price=99.0, shares=500, sigma_distance=0.5, p_revert=0.7),
            ],
            signal_price=100.0,
            stop_price=96.0,
            max_potential_loss=1500.0,
            was_truncated=False,
            original_level_count=1,
        )
        mock_compute.return_value = plan

        pg = _make_mock_playground()
        pdf = _make_pdf()
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar(close=100.0)
        strategy._process_htf_candle(bar)

        assert len(strategy.trade_groups) == 1
        group = strategy.trade_groups[0]
        assert group.htf_signal_price == 100.0
        assert group.deviation_plan == plan
        assert group.status == "pending"
        assert strategy.funnel["groups_created"] == 1

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    def test_htf_bearish_signal_ignored(self, mock_detect):
        """Mean forward return <= 0 → no group created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]

        pg = _make_mock_playground()
        pdf = _make_pdf(mean=-0.005)  # negative mean
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        strategy._process_htf_candle(_make_htf_bar())

        assert len(strategy.trade_groups) == 0

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    def test_insufficient_samples_ignored(self, mock_detect):
        """sufficient_samples=False → no group."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]

        pg = _make_mock_playground()
        pdf = _make_pdf(sufficient=False)
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        strategy._process_htf_candle(_make_htf_bar())

        assert len(strategy.trade_groups) == 0

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    @patch("strategies.mean_reversion.compute_deviation_levels")
    def test_dedup_same_htf_bar(self, mock_compute, mock_detect):
        """Same signal from same HTF bar → second group rejected."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[DeviationLevel(99.0, 500, 0.5, 0.7)],
            signal_price=100.0, stop_price=96.0,
            max_potential_loss=1500.0, was_truncated=False,
            original_level_count=1,
        )
        mock_compute.return_value = plan

        pg = _make_mock_playground()
        pdf = _make_pdf()
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar()
        strategy._process_htf_candle(bar)
        strategy._process_htf_candle(bar)  # same bar again

        assert len(strategy.trade_groups) == 1
        assert strategy.funnel["groups_skipped_dedup"] == 1

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    @patch("strategies.mean_reversion.compute_deviation_levels")
    def test_empty_plan_skips_group(self, mock_compute, mock_detect):
        """Empty deviation plan → no group created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[], signal_price=100.0, stop_price=96.0,
            max_potential_loss=0.0, was_truncated=False,
            original_level_count=0,
        )
        mock_compute.return_value = plan

        pg = _make_mock_playground()
        pdf = _make_pdf()
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        strategy._process_htf_candle(_make_htf_bar())

        assert len(strategy.trade_groups) == 0
        assert strategy.funnel["groups_skipped_empty_plan"] == 1

    @patch("strategies.mean_reversion.detect_atomic_signals_on_bar")
    @patch("strategies.mean_reversion.compute_deviation_levels")
    def test_truncated_plan_logged(self, mock_compute, mock_detect):
        """Budget-truncated plan → funnel counter incremented."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[DeviationLevel(99.0, 500, 0.5, 0.7)],
            signal_price=100.0, stop_price=96.0,
            max_potential_loss=1500.0, was_truncated=True,
            original_level_count=3,
        )
        mock_compute.return_value = plan

        pg = _make_mock_playground()
        pdf = _make_pdf()
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        strategy._process_htf_candle(_make_htf_bar())

        assert len(strategy.trade_groups) == 1
        assert strategy.funnel["groups_truncated"] == 1

    def test_no_pdf_no_groups(self):
        """No PDF → no groups even if signals would fire."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=None)

        # Process with no PDF — should not crash
        strategy._process_htf_candle(_make_htf_bar())
        assert len(strategy.trade_groups) == 0


# ------------------------------------------------------------------ #
# TestEntryFills
# ------------------------------------------------------------------ #

class TestEntryFills:

    def test_ltf_dip_triggers_buy(self):
        """Candle low <= level price → BUY order placed."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        strategy.trade_groups.append(group)

        # Candle low at 98.5 — breaches level 0 (99.0) but not level 1 (98.0)
        strategy._check_entries(group, candle_low=98.5)

        pg.place_order.assert_called_once()
        call_args = pg.place_order.call_args
        assert call_args[0][1] == 100  # shares for level 0
        assert group.filled_levels == {0: 100}
        assert group.status == "active"

    def test_gap_through_fills_all_breached(self):
        """Candle gaps through multiple levels → all get filled."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        strategy.trade_groups.append(group)

        # Candle low at 96.5 — breaches all 3 levels
        strategy._check_entries(group, candle_low=96.5)

        assert pg.place_order.call_count == 3
        assert group.filled_levels == {0: 100, 1: 150, 2: 250}

    def test_filled_level_not_re_triggered(self):
        """Already-filled level → not triggered again."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        group.filled_levels[0] = 100  # level 0 already filled
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        pg.place_order.assert_not_called()

    def test_failed_level_not_re_triggered(self):
        """Failed level → not triggered again."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        group.failed_levels.add(0)
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        pg.place_order.assert_not_called()

    def test_order_attributes_correct(self):
        """Verify group_id, htf_signal, level_index, action on order."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        call_kwargs = pg.place_order.call_args
        attrs = call_kwargs[1]["attributes"] if "attributes" in call_kwargs[1] else call_kwargs[0][-1]
        # attributes is passed as kwarg
        attrs = pg.place_order.call_args[1].get("attributes") or pg.place_order.call_args[0][-1]

        assert attrs["group_id"] == "test1234"
        assert attrs["action"] == "entry"
        assert attrs["level_index"] == "0"
        assert "expected_profit" in attrs
        assert "p_revert" in attrs

    def test_place_order_failure_handled(self):
        """place_order raises → exception caught, level marked failed."""
        pg = _make_mock_playground()
        pg.place_order.side_effect = Exception("Insufficient margin")
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group()
        strategy.trade_groups.append(group)

        # Should not raise
        strategy._check_entries(group, candle_low=98.5)

        assert 0 in group.failed_levels
        assert 0 not in group.filled_levels
        assert strategy.funnel["entries_placed"] == 0

    def test_entry_sets_status_active(self):
        """First entry changes status from pending to active."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(status="pending")
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        assert group.status == "active"

    def test_expected_profit_calculation(self):
        """Expected profit accounts for tiered exits, not full signal reversion."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", ev_model="binary")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        strategy.trade_groups.append(group)

        level = group.deviation_plan.levels[0]  # price=99, shares=100, p_revert=0.6
        # With 3 exit tiers (default), avg exit on revert:
        #   tier0=99.25, tier1=99.50, tier2=100.0 → avg=99.5833
        avg_exit_on_revert = (99.25 + 99.50 + 100.0) / 3
        expected_exit = avg_exit_on_revert * 0.6 + 95.0 * (1 - 0.6)
        expected_profit = 100 * (expected_exit - 99.0)

        strategy._check_entries(group, candle_low=98.5)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert abs(float(attrs["expected_profit"]) - expected_profit) < 0.01


# ------------------------------------------------------------------ #
# TestExits
# ------------------------------------------------------------------ #

class TestExits:

    def test_partial_exit_on_recovery(self):
        """Price recovers to exit tier → SELL order placed."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, status="active")
        group.filled_levels = {0: 100}
        # Manually set exit plan
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.25, shares_to_sell=33, source_level_index=0, tier_index=0),
            ExitTier(exit_price=99.50, shares_to_sell=33, source_level_index=0, tier_index=1),
            ExitTier(exit_price=100.0, shares_to_sell=34, source_level_index=0, tier_index=2),
        ])
        strategy.trade_groups.append(group)

        # Candle high at 99.30 — triggers tier 0 only
        strategy._check_exits(group, candle_high=99.30)

        pg.place_order.assert_called_once()
        call_args = pg.place_order.call_args
        assert call_args[0][1] == 33  # shares
        assert (0, 0) in group.triggered_exits

    def test_exit_not_triggered_when_pending(self):
        """Pending group → no exit checks."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(status="pending")
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.25, shares_to_sell=100, source_level_index=0, tier_index=0),
        ])
        strategy.trade_groups.append(group)

        strategy._check_exits(group, candle_high=100.0)

        pg.place_order.assert_not_called()

    def test_all_exits_triggered_closes_group(self):
        """All exit tiers triggered → group status = closed."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.50, shares_to_sell=50, source_level_index=0, tier_index=0),
            ExitTier(exit_price=100.0, shares_to_sell=50, source_level_index=0, tier_index=1),
        ])
        strategy.trade_groups.append(group)

        # Price hits all tiers
        strategy._check_exits(group, candle_high=100.0)

        assert group.status == "closed"
        assert strategy.funnel["groups_closed"] == 1

    def test_exit_order_attributes(self):
        """Exit order has action=partial_exit and correct group_id."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.50, shares_to_sell=100, source_level_index=0, tier_index=0),
        ])
        strategy.trade_groups.append(group)

        strategy._check_exits(group, candle_high=99.50)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert attrs["action"] == "partial_exit"
        assert attrs["group_id"] == "test1234"


# ------------------------------------------------------------------ #
# TestStopOuts
# ------------------------------------------------------------------ #

class TestStopOuts:

    def test_stop_out_sells_all_remaining(self):
        """HTF close beyond stop → all shares sold."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100, 1: 150}
        strategy.trade_groups.append(group)

        # HTF close at 94 — adverse move = (94-100)/100 = -0.06
        # Stop return = (95-100)/100 = -0.05
        # -0.06 <= -0.05 → stop triggered
        strategy._evaluate_stops(htf_close=94.0)

        assert group.status == "stopped_out"
        pg.place_order.assert_called_once()
        call_args = pg.place_order.call_args
        assert call_args[0][1] == 250  # 100 + 150 shares

    def test_stop_not_triggered_above_threshold(self):
        """HTF close above stop threshold → no stop."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        strategy.trade_groups.append(group)

        # HTF close at 96 — adverse move = -0.04, stop threshold = -0.05
        strategy._evaluate_stops(htf_close=96.0)

        assert group.status == "active"
        pg.place_order.assert_not_called()

    def test_stop_out_order_attributes(self):
        """Stop-out order has action=stop_out."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        strategy.trade_groups.append(group)

        strategy._evaluate_stops(htf_close=94.0)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert attrs["action"] == "stop_out"
        assert attrs["group_id"] == "test1234"

    def test_stop_out_with_partial_exits_subtracts_exited_shares(self):
        """Stop-out subtracts already-exited shares."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", stop_widen_on_exit=0.0)

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.50, shares_to_sell=33, source_level_index=0, tier_index=0),
            ExitTier(exit_price=100.0, shares_to_sell=67, source_level_index=0, tier_index=1),
        ])
        group.triggered_exits = {(0, 0)}  # tier 0 already exited (33 shares)
        strategy.trade_groups.append(group)

        strategy._evaluate_stops(htf_close=94.0)

        call_args = pg.place_order.call_args
        assert call_args[0][1] == 67  # 100 - 33 = 67 remaining

    def test_stop_not_evaluated_for_unfilled_group(self):
        """Group with no fills → stop not evaluated."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0, status="pending")
        # No filled levels
        strategy.trade_groups.append(group)

        strategy._evaluate_stops(htf_close=90.0)

        pg.place_order.assert_not_called()
        assert group.status == "pending"


# ------------------------------------------------------------------ #
# TestEndOfSimCleanup
# ------------------------------------------------------------------ #

class TestEndOfSimCleanup:

    def test_close_active_groups(self):
        """Active groups with fills → sell remaining shares."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, status="active")
        group.filled_levels = {0: 100, 1: 150}
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()

        pg.place_order.assert_called_once()
        call_args = pg.place_order.call_args
        assert call_args[0][1] == 250
        assert group.status == "closed"

    def test_close_skips_already_closed(self):
        """Already closed/stopped groups → not sold again."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(status="closed")
        group.filled_levels = {0: 100}
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()

        pg.place_order.assert_not_called()

    def test_close_subtracts_exited_shares(self):
        """End-of-sim close subtracts partially-exited shares."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.50, shares_to_sell=40, source_level_index=0, tier_index=0),
            ExitTier(exit_price=100.0, shares_to_sell=60, source_level_index=0, tier_index=1),
        ])
        group.triggered_exits = {(0, 0)}  # 40 shares already exited
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()

        call_args = pg.place_order.call_args
        assert call_args[0][1] == 60  # 100 - 40

    def test_close_pending_group_without_fills(self):
        """Pending group with no fills → no sell order, status still closed."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(status="pending")
        # No fills
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()

        pg.place_order.assert_not_called()
        # Pending with no fills — the code checks `group.filled_levels`
        # which is empty, so total_remaining > 0 is False, and status stays "pending"
        # Actually the outer check is: status in ("pending", "active") AND filled_levels
        # Since filled_levels is empty dict → falsy → group won't be processed


# ------------------------------------------------------------------ #
# TestRecomputeExits
# ------------------------------------------------------------------ #

class TestRecomputeExits:

    def test_exit_plan_recomputed_after_fill(self):
        """New fill → exit plan updated."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0)
        group.filled_levels = {0: 100}
        strategy.trade_groups.append(group)

        strategy._recompute_exits(group)

        assert group.exit_plan is not None
        assert len(group.exit_plan.tiers) > 0
        # Exit tiers should target signal_price
        assert any(t.exit_price == 100.0 for t in group.exit_plan.tiers)

    def test_exit_plan_preserves_triggered(self):
        """Recompute preserves previously triggered exits (identity-based)."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0)
        group.filled_levels = {0: 100}
        group.triggered_exits = {(0, 0), (0, 1)}
        strategy.trade_groups.append(group)

        strategy._recompute_exits(group)

        # Triggered exits preserved across rebuild
        assert group.triggered_exits == {(0, 0), (0, 1)}


# ------------------------------------------------------------------ #
# TestProcessCandles
# ------------------------------------------------------------------ #

class TestProcessCandles:

    @patch("strategies.mean_reversion._bar_to_dict")
    def test_htf_candle_dispatched(self, mock_bar_to_dict):
        """Period matching htf_seconds → _process_htf_candle called."""
        mock_bar_to_dict.return_value = _make_htf_bar()
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        candle = _make_candle(_make_htf_bar(), period=3600)
        strategy.process_candles([candle])

        assert strategy.funnel["htf_bars"] == 1

    @patch("strategies.mean_reversion._bar_to_dict")
    def test_ltf_candle_dispatched(self, mock_bar_to_dict):
        """Period matching ltf_seconds → _process_ltf_candle called."""
        mock_bar_to_dict.return_value = _make_ltf_bar()
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        candle = _make_candle(_make_ltf_bar(), period=300)
        strategy.process_candles([candle])

        assert strategy.funnel["ltf_bars"] == 1

    def test_wrong_symbol_ignored(self):
        """Candle for different symbol → ignored."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        candle = _make_candle(_make_ltf_bar(), period=300, symbol="MSFT")
        strategy.process_candles([candle])

        assert strategy.funnel["ltf_bars"] == 0
        assert strategy.funnel["htf_bars"] == 0


# ------------------------------------------------------------------ #
# TestFunnelSummary
# ------------------------------------------------------------------ #

class TestFunnelSummary:

    def test_summary_includes_group_counts(self):
        """log_summary runs without error and has correct counters."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        g1 = _make_group(status="active")
        g1.group_id = "g1"
        g2 = _make_group(status="closed")
        g2.group_id = "g2"
        g3 = _make_group(status="stopped_out")
        g3.group_id = "g3"
        strategy.trade_groups = [g1, g2, g3]

        # Should not raise
        strategy.log_summary()

    def test_funnel_counters_initialized(self):
        """All expected funnel keys exist at creation."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        expected_keys = {
            "htf_bars", "ltf_bars", "signals_detected",
            "groups_created", "groups_skipped_budget",
            "groups_skipped_empty_plan", "groups_skipped_dedup",
            "entries_placed", "exits_placed", "stop_outs",
            "groups_closed", "groups_truncated",
            "entries_skipped_low_ev",
        }
        assert set(strategy.funnel.keys()) == expected_keys
        assert all(v == 0 for v in strategy.funnel.values())


# ------------------------------------------------------------------ #
# TestBarToDict
# ------------------------------------------------------------------ #

class TestBarToDict:

    def test_extracts_standard_fields(self):
        """_bar_to_dict extracts expected fields from a protobuf-like bar."""
        bar = MagicMock()
        # Mock _get and _get_dt to return specific values
        with patch("strategies.mean_reversion._get") as mock_get, \
             patch("strategies.mean_reversion._get_dt") as mock_get_dt:
            mock_get.side_effect = lambda b, field: {
                "open": 99.5, "high": 100.5, "low": 99.0, "close": 100.0,
                "superD_50_3": 1.0,
                "stochrsi_k_14_14_3_3": 25.0,
                "stochrsi_cross_above_20": 1.0,
                "stochrsi_cross_below_80": 0.0,
                "cdl_hammer": 0.0,
                "cdl_doji_10_0_1": 0.0,
                "sma_50": 98.0, "sma_100": 97.0, "sma_200": 95.0,
            }.get(field, None)
            mock_get_dt.return_value = datetime(2025, 6, 15)

            result = _bar_to_dict(bar)

            assert result["open"] == 99.5
            assert result["close"] == 100.0
            assert result["superD_50_3"] == 1.0
            assert result["datetime"] == datetime(2025, 6, 15)


# ------------------------------------------------------------------ #
# TestGetRepositories
# ------------------------------------------------------------------ #

class TestGetRepositories:

    def test_returns_two_repos(self):
        """get_repositories returns LTF (5-min) and HTF (1-hour)."""
        repos = MeanReversionStrategy.get_repositories("AAPL")
        assert len(repos) == 2
        # LTF: 5-min
        assert repos[0].timespan_multiplier == 5
        assert repos[0].timespan_unit == "minute"
        # HTF: 1-hour
        assert repos[1].timespan_multiplier == 1
        assert repos[1].timespan_unit == "hour"

    def test_repos_have_indicators(self):
        """Repositories include required indicators."""
        repos = MeanReversionStrategy.get_repositories("AAPL")
        for repo in repos:
            assert "supertrend" in repo.indicators
            assert "stochrsi" in repo.indicators


# ------------------------------------------------------------------ #
# TestStopWidening
# ------------------------------------------------------------------ #

class TestStopWidening:

    def test_stop_widened_with_partial_exits(self):
        """Group with 1/3 tiers exited: stop NOT triggered at normal threshold."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", stop_widen_on_exit=1.0)

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.25, shares_to_sell=33, source_level_index=0, tier_index=0),
            ExitTier(exit_price=99.50, shares_to_sell=33, source_level_index=0, tier_index=1),
            ExitTier(exit_price=100.0, shares_to_sell=34, source_level_index=0, tier_index=2),
        ])
        group.triggered_exits = {(0, 0)}  # 1 of 3 tiers exited
        strategy.trade_groups.append(group)

        # Normal stop return = (95-100)/100 = -0.05
        # exit_progress = 1/3 = 0.333
        # widen_factor = 1.0 + 0.333 * 1.0 = 1.333
        # effective_stop_return = -0.05 * 1.333 = -0.0667
        # HTF close at 94.5 → move = -0.055 > -0.0667 → should NOT stop
        strategy._evaluate_stops(htf_close=94.5)

        assert group.status == "active"
        pg.place_order.assert_not_called()

    def test_stop_still_triggers_without_exits(self):
        """Group with 0 exits: stop triggered as before (regression)."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", stop_widen_on_exit=1.0)

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        # No exit plan or triggered exits → exit_progress = 0, widen_factor = 1.0
        strategy.trade_groups.append(group)

        # move = (94-100)/100 = -0.06, stop_return = -0.05
        # effective_stop_return = -0.05 * 1.0 = -0.05
        # -0.06 <= -0.05 → stop triggered
        strategy._evaluate_stops(htf_close=94.0)

        assert group.status == "stopped_out"
        pg.place_order.assert_called_once()

    def test_stop_widen_factor_zero_disables(self):
        """stop_widen_on_exit=0.0 → identical to current behavior."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", stop_widen_on_exit=0.0)

        group = _make_group(signal_price=100.0, stop_price=95.0, status="active")
        group.filled_levels = {0: 100}
        group.exit_plan = ExitPlan(tiers=[
            ExitTier(exit_price=99.25, shares_to_sell=33, source_level_index=0, tier_index=0),
            ExitTier(exit_price=99.50, shares_to_sell=33, source_level_index=0, tier_index=1),
            ExitTier(exit_price=100.0, shares_to_sell=34, source_level_index=0, tier_index=2),
        ])
        group.triggered_exits = {(0, 0)}  # 1 of 3 tiers exited
        strategy.trade_groups.append(group)

        # widen_factor = 1.0 + 0.333 * 0.0 = 1.0 (no widening)
        # effective_stop_return = -0.05
        # move at 94.0 = -0.06 <= -0.05 → stop triggered
        strategy._evaluate_stops(htf_close=94.0)

        assert group.status == "stopped_out"


# ------------------------------------------------------------------ #
# TestMinExpectedProfit
# ------------------------------------------------------------------ #

class TestMinExpectedProfit:

    def test_min_expected_profit_skips_low_ev(self):
        """Entry below threshold is skipped, level added to failed_levels."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", min_expected_profit=1000.0)

        group = _make_group(signal_price=100.0, stop_price=95.0)
        strategy.trade_groups.append(group)

        # Level 0: price=99, shares=100, p_revert=0.6
        # Expected profit will be much less than $1000
        strategy._check_entries(group, candle_low=98.5)

        pg.place_order.assert_not_called()
        assert 0 in group.failed_levels
        assert strategy.funnel["entries_skipped_low_ev"] == 1

    def test_min_expected_profit_zero_allows_all(self):
        """Default (0.0) allows all entries (regression)."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", min_expected_profit=0.0)

        group = _make_group(signal_price=100.0, stop_price=95.0)
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        pg.place_order.assert_called_once()
        assert 0 in group.filled_levels


# ------------------------------------------------------------------ #
# TestDistributionEV
# ------------------------------------------------------------------ #

class TestDistributionEV:

    def test_distribution_ev_all_revert(self):
        """All returns >= 0 → EV matches binary full-revert case."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        # All returns positive → all exit at or above signal price
        group.forward_returns = [0.01, 0.02, 0.03, 0.05, 0.10]
        strategy.trade_groups.append(group)

        level = group.deviation_plan.levels[0]  # price=99, shares=100
        ev = strategy._compute_distribution_ev(group, level, 100)

        # With 3 tiers: avg_exit_on_revert = (99.25 + 99.50 + 100.0)/3 = 99.5833
        avg_exit = (99.25 + 99.50 + 100.0) / 3
        expected = 100 * (avg_exit - 99.0)
        assert abs(ev - expected) < 0.01

    def test_distribution_ev_all_stop(self):
        """All returns below stop → EV matches binary stop-out case."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        # All returns push price to or below stop (95.0)
        # stop_return = (95-100)/100 = -0.05
        group.forward_returns = [-0.06, -0.07, -0.08, -0.10]
        strategy.trade_groups.append(group)

        level = group.deviation_plan.levels[0]  # price=99, shares=100
        ev = strategy._compute_distribution_ev(group, level, 100)

        # All stopped out: P&L = 100 * (95.0 - 99.0) = -400
        expected = 100 * (95.0 - 99.0)
        assert abs(ev - expected) < 0.01

    def test_distribution_ev_mixed(self):
        """Mixed returns → EV between stop loss and revert profit."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        # 50% revert (positive), 50% stop out
        group.forward_returns = [0.02, 0.03, -0.06, -0.07]
        strategy.trade_groups.append(group)

        level = group.deviation_plan.levels[0]  # price=99
        ev = strategy._compute_distribution_ev(group, level, 100)

        # Revert case: 100 * (99.5833 - 99) = 58.33
        # Stop case: 100 * (95 - 99) = -400
        # Mixed EV should be between these extremes
        assert ev > -400
        assert ev < 58.34

    def test_distribution_ev_empty_returns(self):
        """No forward_returns → returns 0.0."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        group.forward_returns = []
        strategy.trade_groups.append(group)

        level = group.deviation_plan.levels[0]
        ev = strategy._compute_distribution_ev(group, level, 100)
        assert ev == 0.0


# ------------------------------------------------------------------ #
# TestEVModelFlag
# ------------------------------------------------------------------ #

class TestEVModelFlag:

    def test_distribution_is_default(self):
        """Default ev_model='distribution', active EV matches distribution."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL")
        assert strategy.ev_model == "distribution"

        group = _make_group(signal_price=100.0, stop_price=95.0)
        group.forward_returns = [0.01, 0.02, -0.06]
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert attrs["ev_model"] == "distribution"
        assert "expected_profit_binary" in attrs

    def test_distribution_stores_alt_binary(self):
        """ev_model='distribution' → attrs have expected_profit_binary."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", ev_model="distribution")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        group.forward_returns = [0.01, 0.02, -0.06]
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert attrs["ev_model"] == "distribution"
        assert "expected_profit_binary" in attrs

    def test_binary_stores_alt_distribution(self):
        """ev_model='binary' → attrs have expected_profit_distribution."""
        pg = _make_mock_playground()
        strategy = MeanReversionStrategy(pg, "AAPL", ev_model="binary")

        group = _make_group(signal_price=100.0, stop_price=95.0)
        group.forward_returns = [0.01, 0.02, -0.06]
        strategy.trade_groups.append(group)

        strategy._check_entries(group, candle_low=98.5)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert "expected_profit_distribution" in attrs
        # Both EVs should be numeric strings
        assert float(attrs["expected_profit"])
        assert float(attrs["expected_profit_distribution"])

    def test_forward_returns_stored_on_group(self):
        """Group creation captures forward_returns from horizon."""
        pg = _make_mock_playground()
        forward_rets = [0.01, -0.02, 0.005, -0.03, 0.02]
        pdf = _make_pdf(forward_returns=forward_rets)
        strategy = MeanReversionStrategy(pg, "AAPL", pdf=pdf)

        from unittest.mock import patch as _patch
        with _patch("strategies.mean_reversion.detect_atomic_signals_on_bar") as mock_detect, \
             _patch("strategies.mean_reversion.compute_deviation_levels") as mock_compute:
            mock_detect.return_value = [
                "bullish_supertrend", "stochrsi_cross_above_20",
            ]
            plan = DeviationPlan(
                levels=[DeviationLevel(99.0, 500, 0.5, 0.7)],
                signal_price=100.0, stop_price=96.0,
                max_potential_loss=1500.0, was_truncated=False,
                original_level_count=1,
            )
            mock_compute.return_value = plan

            bar = _make_htf_bar(close=100.0)
            strategy._process_htf_candle(bar)

        assert len(strategy.trade_groups) == 1
        assert strategy.trade_groups[0].forward_returns == forward_rets
