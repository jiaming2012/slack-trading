"""
Unit tests for Kelly criterion and position sizing.

Run::

    cd src/clients/python
    /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_kelly_sizing.py -v
"""

import pytest

from risk_management import (
    kelly_fraction,
    adjusted_kelly,
    max_contracts,
    allocate_contracts,
)


# ================================================================== #
# Kelly fraction
# ================================================================== #

class TestKellyFraction:
    def test_basic_kelly(self):
        # p=0.6, b=1.5 → f* = 0.6 - 0.4/1.5 = 0.333...
        assert kelly_fraction(0.6, 1.5) == pytest.approx(1 / 3, abs=1e-6)

    def test_even_odds(self):
        # p=0.5, b=1.0 → f* = 0.5 - 0.5/1.0 = 0.0
        assert kelly_fraction(0.5, 1.0) == pytest.approx(0.0)

    def test_no_edge(self):
        # p=0.4, b=1.0 → f* = 0.4 - 0.6/1.0 = -0.2 → clamped to 0
        assert kelly_fraction(0.4, 1.0) == 0.0

    def test_certain_win(self):
        # p=1.0, b=2.0 → f* = 1.0 - 0.0/2.0 = 1.0
        assert kelly_fraction(1.0, 2.0) == 1.0

    def test_zero_win_prob(self):
        assert kelly_fraction(0.0, 2.0) == 0.0

    def test_zero_ratio(self):
        assert kelly_fraction(0.6, 0.0) == 0.0

    def test_high_edge(self):
        # p=0.9, b=3.0 → f* = 0.9 - 0.1/3.0 = 0.8667
        assert kelly_fraction(0.9, 3.0) == pytest.approx(0.8667, abs=1e-3)


# ================================================================== #
# Adjusted Kelly (drawdown + fraction)
# ================================================================== #

class TestAdjustedKelly:
    def test_half_kelly(self):
        f = adjusted_kelly(f_star=0.4, current_equity=100000,
                           peak_equity=100000, fraction=0.5)
        assert f == pytest.approx(0.2)

    def test_drawdown_scaling(self):
        # equity at 80% of peak → 80% of half-Kelly
        f = adjusted_kelly(f_star=0.4, current_equity=80000,
                           peak_equity=100000, fraction=0.5)
        assert f == pytest.approx(0.4 * 0.5 * 0.8)

    def test_full_kelly(self):
        f = adjusted_kelly(f_star=0.4, current_equity=100000,
                           peak_equity=100000, fraction=1.0)
        assert f == pytest.approx(0.4)

    def test_zero_equity(self):
        assert adjusted_kelly(0.4, 0, 100000) == 0.0

    def test_zero_peak(self):
        assert adjusted_kelly(0.4, 100000, 0) == 0.0

    def test_equity_above_peak_clamped(self):
        # current > peak → ratio clamped to 1.0
        f = adjusted_kelly(f_star=0.4, current_equity=120000,
                           peak_equity=100000, fraction=0.5)
        assert f == pytest.approx(0.2)

    def test_severe_drawdown(self):
        # 50% drawdown → half the half-Kelly
        f = adjusted_kelly(f_star=0.4, current_equity=50000,
                           peak_equity=100000, fraction=0.5)
        assert f == pytest.approx(0.4 * 0.5 * 0.5)


# ================================================================== #
# Max contracts
# ================================================================== #

class TestMaxContracts:
    def test_basic(self):
        # 10% of $100k = $10k, at $230/share → $23k per contract → 0
        assert max_contracts(0.1, 100000, 23000) == 0

        # 20% of $100k = $20k, at $200/share → $20k per contract → 1
        assert max_contracts(0.2, 100000, 20000) == 1

    def test_multiple_contracts(self):
        # 50% of $100k = $50k, at $10k per contract → 5
        assert max_contracts(0.5, 100000, 10000) == 5

    def test_zero_fraction(self):
        assert max_contracts(0.0, 100000, 10000) == 0

    def test_zero_margin(self):
        assert max_contracts(0.5, 100000, 0) == 0


# ================================================================== #
# Contract allocation
# ================================================================== #

class TestAllocateContracts:
    def test_proportional_allocation(self):
        # 3 strikes at probabilities 50%, 30%, 20% → 10 contracts
        alloc = allocate_contracts(10, [0.5, 0.3, 0.2])
        assert sum(alloc) == 10
        # Higher probability gets more contracts
        assert alloc[0] >= alloc[1] >= alloc[2]

    def test_each_strike_gets_at_least_one(self):
        alloc = allocate_contracts(5, [0.5, 0.3, 0.2])
        assert sum(alloc) == 5
        assert all(a >= 1 for a in alloc)

    def test_not_enough_for_all(self):
        # 2 contracts for 4 strikes
        alloc = allocate_contracts(2, [0.4, 0.3, 0.2, 0.1])
        assert sum(alloc) == 2
        # Highest prob strikes should get the contracts
        assert alloc[0] == 1
        assert alloc[1] == 1

    def test_single_strike(self):
        alloc = allocate_contracts(5, [0.5])
        assert alloc == [5]

    def test_zero_contracts(self):
        alloc = allocate_contracts(0, [0.5, 0.3])
        assert alloc == [0, 0]

    def test_empty_strikes(self):
        alloc = allocate_contracts(5, [])
        assert alloc == []

    def test_equal_probabilities(self):
        alloc = allocate_contracts(9, [0.25, 0.25, 0.25, 0.25])
        assert sum(alloc) == 9
        # Should be roughly equal: [3, 2, 2, 2] or similar
        assert max(alloc) - min(alloc) <= 1
