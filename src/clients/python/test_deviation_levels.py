"""Tests for deviation_levels module."""

import math

import numpy as np
import pytest

from deviation_levels import (
    DeviationLevel,
    DeviationPlan,
    compute_deviation_levels,
    _auto_sigma_steps,
    _compute_p_revert,
)


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_returns(n=200, mean=-0.005, std=0.02, seed=42):
    """Generate synthetic forward returns."""
    rng = np.random.default_rng(seed)
    return list(rng.normal(mean, std, n))


# ------------------------------------------------------------------ #
# TestComputeDeviationLevels
# ------------------------------------------------------------------ #

class TestComputeDeviationLevels:

    def test_basic_levels_known_inputs(self):
        """Explicit sigma steps produce correct prices and distances."""
        signal_price = 100.0
        stddev = 0.01  # 1% stddev — levels at 99, 98.5, 98
        returns = _make_returns(200, mean=-0.002, std=0.01)
        plan = compute_deviation_levels(
            signal_price=signal_price,
            stddev=stddev,
            forward_returns=returns,
            stop_percentile=0.99,  # tight stop so levels aren't clipped
            max_loss_budget=100_000,
            total_shares=1000,
            sigma_steps=[0.5, 1.0, 1.5],
        )
        assert len(plan.levels) == 3
        # Verify prices: signal * (1 - sigma * stddev)
        for lv in plan.levels:
            expected_price = signal_price * (1.0 - lv.sigma_distance * stddev)
            assert abs(lv.price - expected_price) < 1e-10
        # Sigma distances should be sorted ascending
        sigmas = [lv.sigma_distance for lv in plan.levels]
        assert sigmas == sorted(sigmas)

    def test_shares_proportional_to_p_revert(self):
        """Deeper levels have higher p_revert and get more shares.

        p_revert = P(return >= -dip_magnitude), i.e. the fraction of
        returns that did NOT dip as far as this level. For deeper levels,
        this fraction is higher (most returns are less extreme), meaning
        the strategy allocates MORE shares at deeper levels — which is
        the intended behavior (scale in more at extreme dips).
        """
        returns = _make_returns(500, mean=-0.005, std=0.025)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.025,
            forward_returns=returns,
            stop_percentile=0.99,
            max_loss_budget=100_000,
            total_shares=1000,
            sigma_steps=[0.5, 1.0, 1.5],
        )
        assert len(plan.levels) >= 2
        # Deeper levels (higher sigma) have HIGHER p_revert and MORE shares
        assert plan.levels[-1].p_revert >= plan.levels[0].p_revert
        assert plan.levels[-1].shares >= plan.levels[0].shares

    def test_budget_truncation_removes_deepest(self):
        """Small budget truncates deepest levels."""
        returns = _make_returns(200, mean=-0.002, std=0.01)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.01,
            forward_returns=returns,
            stop_percentile=0.99,  # tight stop so levels survive
            max_loss_budget=1.0,  # very tight budget
            total_shares=100,
            sigma_steps=[0.5, 1.0, 1.5],
        )
        assert plan.was_truncated is True
        assert len(plan.levels) < plan.original_level_count

    def test_budget_preserves_minimum_one_level(self):
        """Even with tiny budget, at least 1 level is preserved."""
        returns = _make_returns(200)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.02,
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=0.01,  # extremely tight
            total_shares=10,
            sigma_steps=[1.0, 2.0],
        )
        # Should have at least 1 level (or empty if edge case)
        # With only 10 shares and small budget, might keep 1
        assert len(plan.levels) >= 1 or plan.original_level_count == 0

    def test_auto_sigma_steps_from_distribution(self):
        """Without explicit steps, levels auto-generated from tail."""
        returns = _make_returns(500, mean=-0.01, std=0.03)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.03,
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=100_000,
            total_shares=1000,
            sigma_steps=None,  # auto
        )
        assert len(plan.levels) >= 1
        # First level should be at 1σ
        assert abs(plan.levels[0].sigma_distance - 1.0) < 1e-10

    def test_stop_price_at_percentile(self):
        """Stop price matches the expected percentile of returns."""
        returns = [-0.05, -0.03, -0.01, 0.01, 0.03]
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.03,
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=100_000,
            total_shares=100,
            sigma_steps=[1.0],
        )
        expected_stop_return = float(np.percentile(returns, 5.0))
        expected_stop_price = 100.0 * (1.0 + expected_stop_return)
        assert abs(plan.stop_price - expected_stop_price) < 1e-10

    def test_zero_stddev_returns_empty(self):
        """stddev=0 → empty plan."""
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.0,
            forward_returns=_make_returns(50),
            stop_percentile=0.95,
            max_loss_budget=10_000,
            total_shares=100,
        )
        assert len(plan.levels) == 0

    def test_tiny_stddev_returns_empty(self):
        """stddev < 1e-8 → empty plan."""
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=1e-9,
            forward_returns=_make_returns(50),
            stop_percentile=0.95,
            max_loss_budget=10_000,
            total_shares=100,
        )
        assert len(plan.levels) == 0

    def test_empty_returns_returns_empty(self):
        """forward_returns=[] → empty plan."""
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.02,
            forward_returns=[],
            stop_percentile=0.95,
            max_loss_budget=10_000,
            total_shares=100,
        )
        assert len(plan.levels) == 0

    def test_few_returns_returns_empty(self):
        """< 3 forward returns → empty plan."""
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.02,
            forward_returns=[0.01, -0.01],
            stop_percentile=0.95,
            max_loss_budget=10_000,
            total_shares=100,
        )
        assert len(plan.levels) == 0

    def test_stop_above_entry_skips_level(self):
        """Levels where stop >= entry price are excluded."""
        # Craft returns where the stop is very high (near signal)
        # by using returns that are mostly positive (stop will be high)
        returns = [0.05, 0.04, 0.03, 0.02, 0.01, -0.001]
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.001,  # tiny stddev = levels very close to signal
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=10_000,
            total_shares=100,
            sigma_steps=[1.0],
        )
        # With stddev=0.001, level at 100*(1-0.001) = 99.9
        # Stop at 5th percentile of returns = -0.001, so stop = 99.9
        # Level might be skipped since stop >= level
        for lv in plan.levels:
            assert plan.stop_price < lv.price

    def test_max_loss_calculation(self):
        """max_potential_loss = sum(shares_i * abs(level_i - stop))."""
        returns = _make_returns(200)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.02,
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=100_000,
            total_shares=1000,
            sigma_steps=[1.0, 1.5],
        )
        expected_loss = sum(
            lv.shares * abs(lv.price - plan.stop_price)
            for lv in plan.levels
        )
        assert abs(plan.max_potential_loss - expected_loss) < 1e-6

    def test_total_shares_sum(self):
        """All shares are distributed — sum equals total_shares."""
        returns = _make_returns(200)
        plan = compute_deviation_levels(
            signal_price=100.0,
            stddev=0.02,
            forward_returns=returns,
            stop_percentile=0.95,
            max_loss_budget=100_000,
            total_shares=1000,
            sigma_steps=[1.0, 1.5, 2.0],
        )
        if plan.levels:
            total = sum(lv.shares for lv in plan.levels)
            assert total == 1000


# ------------------------------------------------------------------ #
# TestAutoSigmaSteps
# ------------------------------------------------------------------ #

class TestAutoSigmaSteps:

    def test_starts_at_1_sigma(self):
        """First step is at 1.0σ."""
        returns = np.array(_make_returns(500, std=0.03))
        steps = _auto_sigma_steps(returns, stddev=0.03)
        assert len(steps) >= 1
        assert steps[0] == 1.0

    def test_stops_at_tail_threshold(self):
        """Stops when < 2% of returns reached the level."""
        returns = np.array(_make_returns(500, std=0.02))
        steps = _auto_sigma_steps(returns, stddev=0.02)
        # The last step should have >= 2% of returns at that level
        for s in steps:
            frac = float(np.mean(returns <= -s * 0.02))
            assert frac >= 0.02

    def test_empty_for_very_narrow_distribution(self):
        """Very tight returns with no tail → no steps."""
        returns = np.array([0.001] * 100)
        steps = _auto_sigma_steps(returns, stddev=0.001)
        # All returns are positive, so no returns reach -1σ
        assert len(steps) == 0


# ------------------------------------------------------------------ #
# TestComputePRevert
# ------------------------------------------------------------------ #

class TestComputePRevert:

    def test_all_positive_returns(self):
        """All positive returns → p_revert = 1.0 for any dip."""
        returns = np.array([0.01, 0.02, 0.03, 0.04, 0.05])
        p = _compute_p_revert(returns, dip_magnitude=0.01)
        assert p == 1.0

    def test_all_negative_returns(self):
        """All negative returns → p_revert = 0 for shallow dip."""
        returns = np.array([-0.01, -0.02, -0.03, -0.04, -0.05])
        p = _compute_p_revert(returns, dip_magnitude=0.005)
        # p_not_as_bad = fraction of returns >= -0.005 = 0/5 = 0
        assert p == 0.0

    def test_mixed_returns(self):
        """Mix of positive and negative → p_revert between 0 and 1."""
        returns = np.array([-0.03, -0.02, -0.01, 0.01, 0.02])
        p = _compute_p_revert(returns, dip_magnitude=0.015)
        # p_not_as_bad = fraction >= -0.015 = 3/5 = 0.6
        assert abs(p - 0.6) < 1e-10
