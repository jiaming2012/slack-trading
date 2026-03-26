"""Tests for partial_exit_manager module."""

import pytest

from partial_exit_manager import (
    ExitPlan,
    ExitTier,
    check_exits,
    compute_exit_plan,
    _split_shares,
)


# ------------------------------------------------------------------ #
# TestComputeExitPlan
# ------------------------------------------------------------------ #

class TestComputeExitPlan:

    def test_three_tiers_evenly_spaced(self):
        """3 tiers between entry and signal price."""
        plan = compute_exit_plan(
            filled_levels=[(95.0, 300)],
            signal_price=100.0,
            num_tiers=3,
        )
        assert len(plan.tiers) == 3

        # Tier 0: 95 + 1/4 * 5 = 96.25
        assert abs(plan.tiers[0].exit_price - 96.25) < 1e-10
        # Tier 1: 95 + 2/4 * 5 = 97.50
        assert abs(plan.tiers[1].exit_price - 97.50) < 1e-10
        # Tier 2: signal price = 100.00
        assert abs(plan.tiers[2].exit_price - 100.0) < 1e-10

    def test_share_split_even(self):
        """300 shares / 3 tiers → 100, 100, 100."""
        plan = compute_exit_plan([(95.0, 300)], 100.0, num_tiers=3)
        shares = [t.shares_to_sell for t in plan.tiers]
        assert shares == [100, 100, 100]

    def test_share_split_uneven(self):
        """100 shares / 3 tiers → split with remainder to last tiers."""
        plan = compute_exit_plan([(95.0, 100)], 100.0, num_tiers=3)
        shares = [t.shares_to_sell for t in plan.tiers]
        assert sum(shares) == 100
        # base=33, remainder=1 → [33, 33, 34]
        assert shares == [33, 33, 34]

    def test_single_share_goes_to_final_tier(self):
        """1 share, 3 tiers → only last tier gets 1 share."""
        plan = compute_exit_plan([(95.0, 1)], 100.0, num_tiers=3)
        shares = [t.shares_to_sell for t in plan.tiers]
        # base=0, remainder=1 → [0, 0, 1]
        # Tiers with 0 shares are excluded
        assert len(plan.tiers) == 1
        assert plan.tiers[0].shares_to_sell == 1
        assert plan.tiers[0].tier_index == 2  # last tier

    def test_multiple_filled_levels(self):
        """Each filled level gets independent exit tiers."""
        plan = compute_exit_plan(
            filled_levels=[(95.0, 300), (90.0, 600)],
            signal_price=100.0,
            num_tiers=3,
        )
        # 3 tiers for level 0 + 3 tiers for level 1 = 6 total
        assert len(plan.tiers) == 6
        level_0_tiers = [t for t in plan.tiers if t.source_level_index == 0]
        level_1_tiers = [t for t in plan.tiers if t.source_level_index == 1]
        assert len(level_0_tiers) == 3
        assert len(level_1_tiers) == 3
        assert sum(t.shares_to_sell for t in level_0_tiers) == 300
        assert sum(t.shares_to_sell for t in level_1_tiers) == 600

    def test_signal_at_entry_no_tiers(self):
        """signal_price == entry_price → no exit tiers."""
        plan = compute_exit_plan([(100.0, 300)], 100.0, num_tiers=3)
        assert len(plan.tiers) == 0

    def test_signal_below_entry_no_tiers(self):
        """signal_price < entry_price → no exit tiers."""
        plan = compute_exit_plan([(100.0, 300)], 95.0, num_tiers=3)
        assert len(plan.tiers) == 0

    def test_empty_filled_levels(self):
        """No fills → empty ExitPlan."""
        plan = compute_exit_plan([], 100.0, num_tiers=3)
        assert len(plan.tiers) == 0

    def test_zero_shares(self):
        """Zero shares in a level → no tiers for that level."""
        plan = compute_exit_plan([(95.0, 0)], 100.0, num_tiers=3)
        assert len(plan.tiers) == 0

    def test_tier_indices_correct(self):
        """Tier indices are sequential within each level."""
        plan = compute_exit_plan([(95.0, 300)], 100.0, num_tiers=3)
        indices = [t.tier_index for t in plan.tiers]
        assert indices == [0, 1, 2]

    def test_exit_prices_monotonically_increasing(self):
        """Exit prices for a single level are monotonically increasing."""
        plan = compute_exit_plan([(90.0, 500)], 100.0, num_tiers=5)
        prices = [t.exit_price for t in plan.tiers]
        for i in range(1, len(prices)):
            assert prices[i] > prices[i - 1]


# ------------------------------------------------------------------ #
# TestCheckExits
# ------------------------------------------------------------------ #

class TestCheckExits:

    def _make_plan(self):
        """Helper: 3 tiers at 96.25, 97.50, 100.00."""
        return compute_exit_plan([(95.0, 300)], 100.0, num_tiers=3)

    def test_price_at_tier1_triggers_tier0_and_1(self):
        """Price at tier 1 triggers tier 0 and tier 1."""
        plan = self._make_plan()
        triggered = check_exits(97.50, plan, set())
        assert len(triggered) == 2
        assert triggered[0].tier_index == 0  # 96.25
        assert triggered[1].tier_index == 1  # 97.50

    def test_already_triggered_skipped(self):
        """Previously triggered tiers are not re-triggered."""
        plan = self._make_plan()
        # Tier 0 already triggered
        triggered = check_exits(97.50, plan, already_triggered={(0, 0)})
        assert len(triggered) == 1
        assert triggered[0].tier_index == 1

    def test_price_below_all_tiers(self):
        """Price below all exit prices → empty list."""
        plan = self._make_plan()
        triggered = check_exits(95.50, plan, set())
        assert len(triggered) == 0

    def test_price_above_all_tiers(self):
        """Price >= signal_price → all untriggered tiers."""
        plan = self._make_plan()
        triggered = check_exits(100.0, plan, set())
        assert len(triggered) == 3

    def test_price_above_all_some_triggered(self):
        """Price high, but tier 0 and 1 already triggered."""
        plan = self._make_plan()
        triggered = check_exits(100.0, plan, already_triggered={(0, 0), (0, 1)})
        assert len(triggered) == 1
        assert triggered[0].tier_index == 2

    def test_exact_price_match(self):
        """Price exactly at tier exit → triggers."""
        plan = self._make_plan()
        triggered = check_exits(96.25, plan, set())
        assert len(triggered) == 1
        assert triggered[0].tier_index == 0


# ------------------------------------------------------------------ #
# TestSplitShares
# ------------------------------------------------------------------ #

class TestSplitShares:

    def test_even_split(self):
        assert _split_shares(300, 3) == [100, 100, 100]

    def test_remainder_to_last(self):
        assert _split_shares(100, 3) == [33, 33, 34]

    def test_two_remainder(self):
        assert _split_shares(101, 3) == [33, 34, 34]

    def test_single_tier(self):
        assert _split_shares(500, 1) == [500]

    def test_more_tiers_than_shares(self):
        result = _split_shares(2, 5)
        assert sum(result) == 2
        assert result == [0, 0, 0, 1, 1]

    def test_zero_shares(self):
        assert _split_shares(0, 3) == [0, 0, 0]

    def test_zero_tiers(self):
        assert _split_shares(100, 0) == []
