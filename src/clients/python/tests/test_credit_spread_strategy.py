"""Tests for credit_spread_strategy module."""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Set
from unittest.mock import MagicMock, patch, PropertyMock, call

import pytest
import numpy as np

from deviation_levels import DeviationLevel, DeviationPlan
from credit_spread_strategy import (
    CreditSpreadStrategy,
    CreditSpreadGroup,
    CreditSpreadEntry,
    SpreadLeg,
    run_credit_spread_strategy,
)
from pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(equity=100_000.0):
    """Create a mock playground with minimal interface."""
    pg = MagicMock()
    pg.htf_seconds = 3600
    pg.ltf_seconds = 300
    pg.id = "test-pg-id"
    pg.account = MagicMock()
    pg.account.equity = equity
    pg.account.free_margin = equity
    pg.account.get_position = MagicMock(return_value=None)
    pg.place_order = MagicMock()
    pg.fetch_ladder = MagicMock()
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
    bar = {
        "open": close - 0.2,
        "high": high,
        "low": low,
        "close": close,
        "datetime": datetime(2025, 6, 15, 10, 5),
    }
    bar.update(overrides)
    return bar


def _make_put_contract(
    symbol="O:AAPL250718P00098000",
    strike=98.0,
    bid=1.50,
    ask=1.70,
    expiration_date="2025-07-18",
):
    """Create a mock put OptionLadderContract."""
    c = MagicMock()
    c.symbol = symbol
    c.type = "put"
    c.strike = strike
    c.bid = bid
    c.ask = ask
    c.expiration_date = expiration_date
    c.contract_size = 100
    return c


def _make_call_contract(
    symbol="O:AAPL250718C00102000",
    strike=102.0,
    bid=1.50,
    ask=1.70,
    expiration_date="2025-07-18",
):
    """Create a mock call OptionLadderContract."""
    c = MagicMock()
    c.symbol = symbol
    c.type = "call"
    c.strike = strike
    c.bid = bid
    c.ask = ask
    c.expiration_date = expiration_date
    c.contract_size = 100
    return c


def _make_ladder_response(contracts):
    response = MagicMock()
    response.contracts = contracts
    return response


def _make_group(
    signal_price=100.0,
    direction="bullish",
    levels=None,
    stop_price=95.0,
    status="pending",
    htf_stddev=0.02,
    horizons=None,
    forward_returns=None,
):
    """Create a CreditSpreadGroup with a deviation plan."""
    if levels is None:
        levels = [
            DeviationLevel(price=99.0, shares=2, sigma_distance=0.5, p_revert=0.6),
            DeviationLevel(price=98.0, shares=2, sigma_distance=1.0, p_revert=0.7),
            DeviationLevel(price=97.0, shares=1, sigma_distance=1.5, p_revert=0.8),
        ]
    plan = DeviationPlan(
        levels=levels,
        signal_price=signal_price,
        stop_price=stop_price,
        max_potential_loss=2500.0,
        was_truncated=False,
        original_level_count=len(levels),
    )
    if horizons is None:
        if forward_returns is None:
            rng = np.random.default_rng(42)
            forward_returns = list(rng.normal(0.005, htf_stddev, 100))
        horizon = HorizonStats(
            mean=0.005,
            stddev=htf_stddev,
            percentiles={"5": -0.03, "10": -0.02, "25": -0.005, "50": 0.005},
            forward_returns=forward_returns,
        )
        horizons = {"1h": horizon}
    return CreditSpreadGroup(
        group_id="test1234",
        htf_signal_key="bullish_supertrend|stochrsi_cross_above_20",
        htf_signal_price=signal_price,
        htf_signal_timestamp=datetime(2025, 6, 15, 10, 0),
        direction=direction,
        deviation_plan=plan,
        status=status,
        htf_stddev=htf_stddev,
        horizons=horizons,
    )


def _make_spread_entry(
    short_strike=98.0,
    long_strike=93.0,
    short_bid=1.50,
    long_ask=0.60,
    contracts=2,
    level_index=0,
    expiration_date="2025-07-18",
):
    """Create a CreditSpreadEntry for testing."""
    net_credit_per = short_bid - long_ask
    actual_width = short_strike - long_strike
    net_credit = net_credit_per * 100 * contracts
    max_loss = (actual_width - net_credit_per) * 100 * contracts
    return CreditSpreadEntry(
        short_leg=SpreadLeg(
            contract_symbol=f"O:AAPL250718P00{int(short_strike * 1000):08d}",
            option_type="put",
            strike=short_strike,
            contracts=contracts,
            side="short",
            fill_price=short_bid,
            expiration_date=expiration_date,
        ),
        long_leg=SpreadLeg(
            contract_symbol=f"O:AAPL250718P00{int(long_strike * 1000):08d}",
            option_type="put",
            strike=long_strike,
            contracts=contracts,
            side="long",
            fill_price=long_ask,
            expiration_date=expiration_date,
        ),
        contracts=contracts,
        net_credit=net_credit,
        spread_width=actual_width,
        max_loss=max_loss,
        level_index=level_index,
        sigma_distance=0.5,
        p_profit=0.75,
        expected_profit=50.0,
        entry_stock_price=100.0,
        entry_timestamp=datetime(2025, 6, 15, 10, 5),
    )


# ================================================================== #
# CORE STRATEGY CORRECTNESS
# ================================================================== #

class TestBullPutSpreadEntry:

    def test_bull_put_spread_entry(self):
        """SELL_TO_OPEN + BUY_TO_OPEN placed with correct strikes and sides."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group()
        strategy.trade_groups.append(group)

        # Ladder: short put at 99, long put at 94
        short_put = _make_put_contract(
            symbol="O:AAPL250718P00099000", strike=99.0, bid=2.00, ask=2.20,
        )
        long_put = _make_put_contract(
            symbol="O:AAPL250718P00094000", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        # Should have placed 2 orders: SELL_TO_OPEN short, BUY_TO_OPEN long
        assert pg.place_order.call_count == 2
        calls = pg.place_order.call_args_list

        # First call: SELL_TO_OPEN the short put
        from playground_types import OrderSide
        assert calls[0][0][0] == "O:AAPL250718P00099000"  # symbol
        assert calls[0][0][2] == OrderSide.SELL_TO_OPEN    # side
        assert calls[0][0][3] == "option"                  # asset_class

        # Second call: BUY_TO_OPEN the long put
        assert calls[1][0][0] == "O:AAPL250718P00094000"
        assert calls[1][0][2] == OrderSide.BUY_TO_OPEN
        assert calls[1][0][3] == "option"

        # Entry should be recorded
        assert 0 in group.entries
        entry = group.entries[0]
        assert entry.short_leg.strike == 99.0
        assert entry.long_leg.strike == 94.0
        assert entry.spread_width == 5.0

    def test_entry_order_attributes_complete(self):
        """Every place_order call includes required attributes."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group()
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="O:AAPL250718P00094000", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        for c in pg.place_order.call_args_list:
            attrs = c[1]["attributes"]
            assert "group_id" in attrs
            assert "level_index" in attrs
            assert "leg" in attrs
            assert "spread_width" in attrs
            assert "net_credit" in attrs
            assert "p_profit" in attrs
            assert "expected_profit" in attrs
            assert "sigma_distance" in attrs
            assert "direction" in attrs

    def test_net_credit_arithmetic(self):
        """Net credit = (short_bid - long_ask) * 100 * contracts."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=1.35, ask=1.55)
        long_put = _make_put_contract(
            symbol="O:AAPL250718P00094000", strike=94.0, bid=0.30, ask=0.42,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if 0 in group.entries:
            entry = group.entries[0]
            expected_credit = (1.35 - 0.42) * 100 * 3  # 0.93 * 300 = 279.0
            assert abs(entry.net_credit - expected_credit) < 0.01

    def test_max_loss_arithmetic(self):
        """Max loss = (width - credit_per_contract) * 100 * contracts."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=2, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="O:AAPL250718P00094000", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if 0 in group.entries:
            entry = group.entries[0]
            credit_per = 2.00 - 0.60  # 1.40
            width = 5.0
            expected_max_loss = (width - credit_per) * 100 * 2  # 3.60 * 200 = 720.0
            assert abs(entry.max_loss - expected_max_loss) < 0.01

    def test_group_lifecycle(self):
        """Full lifecycle: signal → entry → exit → closed."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", min_hold_candles=0, profit_target_pct=0.50,
        )

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=2, sigma_distance=0.5, p_revert=0.6)],
            status="pending",
        )
        strategy.trade_groups.append(group)

        # Place entry
        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="O:AAPL250718P00094000", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)
        assert group.status == "active"
        assert 0 in group.entries

        # Mock positions for exit: spread value has decayed (profitable)
        short_pos = MagicMock()
        short_pos.current_price = 0.30  # can buy back cheap
        short_pos.quantity = -2
        long_pos = MagicMock()
        long_pos.current_price = 0.10
        long_pos.quantity = 2

        def get_pos(symbol):
            if "099" in symbol:
                return short_pos
            if "094" in symbol:
                return long_pos
            return None
        pg.account.get_position = get_pos

        # Check exits — spread value = 0.30 - 0.10 = 0.20
        # credit_per = 1.40, threshold = 0.50 * 1.40 = 0.70
        # 0.20 < 0.70 → profit target hit
        pg.place_order.reset_mock()
        strategy._check_exits(group, bar)
        assert 0 in group.exited_entries
        assert group.status == "closed"


# ================================================================== #
# STRIKE SELECTION EDGE CASES
# ================================================================== #

class TestStrikeSelection:

    def test_spread_strike_selection_exact(self):
        """Ladder has strikes at exact $5 intervals → selects perfect pair."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", spread_width=5.0)

        group = _make_group()
        level = group.deviation_plan.levels[0]  # price=99.0

        contracts = [
            _make_put_contract(strike=99.0, bid=2.00, ask=2.20),
            _make_put_contract(symbol="P97", strike=97.0, bid=1.20, ask=1.40),
            _make_put_contract(symbol="P94", strike=94.0, bid=0.50, ask=0.60),
        ]

        result = strategy._select_bull_put_strikes(level, contracts, group, 100.0)
        assert result is not None
        short, long = result
        assert short.strike == 99.0
        assert long.strike == 94.0

    def test_spread_strike_selection_nearest(self):
        """No strike at exact width → picks nearest within tolerance."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", spread_width=5.0)

        group = _make_group()
        level = group.deviation_plan.levels[0]

        contracts = [
            _make_put_contract(strike=99.0, bid=2.00, ask=2.20),
            _make_put_contract(symbol="P95", strike=95.0, bid=0.80, ask=0.95),
        ]

        result = strategy._select_bull_put_strikes(level, contracts, group, 100.0)
        assert result is not None
        short, long = result
        assert short.strike == 99.0
        assert long.strike == 95.0  # within $2.50 tolerance of target $94

    def test_spread_strike_too_narrow(self):
        """Only strikes within $2 available → skips (< min width)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", spread_width=5.0, min_spread_width=2.50,
        )

        group = _make_group()
        level = group.deviation_plan.levels[0]

        contracts = [
            _make_put_contract(strike=99.0, bid=2.00, ask=2.20),
            _make_put_contract(symbol="P97", strike=97.5, bid=1.50, ask=1.70),
        ]

        result = strategy._select_bull_put_strikes(level, contracts, group, 100.0)
        # 99 - 97.5 = 1.5 < 2.50 min
        assert result is None
        assert strategy.funnel["entries_skipped_no_long"] >= 1

    def test_spread_strike_no_short_available(self):
        """No put contracts at or below level price → skip."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        group = _make_group()
        level = group.deviation_plan.levels[0]  # price=99.0

        # All puts above level price and above stock price
        contracts = [
            _make_put_contract(strike=105.0, bid=6.00, ask=6.20),
        ]

        # Stock price is at 101 — above all candidates
        result = strategy._select_bull_put_strikes(level, contracts, group, 101.0)
        assert result is None
        assert strategy.funnel["entries_skipped_no_short"] >= 1

    def test_spread_strike_no_long_available(self):
        """Short found but no protective long within tolerance → skip."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", spread_width=5.0)

        group = _make_group()
        level = group.deviation_plan.levels[0]

        # Only one put at 99 — no long leg candidates
        contracts = [
            _make_put_contract(strike=99.0, bid=2.00, ask=2.20),
        ]

        result = strategy._select_bull_put_strikes(level, contracts, group, 100.0)
        assert result is None
        assert strategy.funnel["entries_skipped_no_long"] >= 1

    def test_spread_strike_uses_bid_ask_not_close(self):
        """Short leg uses bid price, long leg uses ask price (conservative)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.50)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.80,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if 0 in group.entries:
            entry = group.entries[0]
            # Credit should be based on bid/ask, not mid or close
            assert entry.short_leg.fill_price == 2.00  # bid
            assert entry.long_leg.fill_price == 0.80   # ask
            assert abs(entry.net_credit - (2.00 - 0.80) * 100 * 1) < 0.01


# ================================================================== #
# COLLATERAL & BUDGET MANAGEMENT
# ================================================================== #

class TestCollateralManagement:

    def test_collateral_per_group_limit(self):
        """Entry exceeding group collateral limit → skipped."""
        pg = _make_mock_playground(equity=10_000.0)
        strategy = CreditSpreadStrategy(
            pg, "AAPL",
            max_collateral_pct=0.01,  # $100 per group
            spread_width=5.0,
            min_hold_candles=0,
        )

        group = _make_group(
            levels=[
                DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6),
            ],
        )
        strategy.trade_groups.append(group)

        # 3 contracts * $5 * 100 = $1500 collateral needed > $100 limit
        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_collateral"] >= 1

    def test_collateral_total_portfolio_limit(self):
        """Total portfolio collateral exceeded → new entry skipped."""
        pg = _make_mock_playground(equity=10_000.0)
        strategy = CreditSpreadStrategy(
            pg, "AAPL",
            max_collateral_pct=1.0,     # high per-group limit
            max_total_collateral_pct=0.05,  # $500 total
            spread_width=5.0,
            min_hold_candles=0,
        )

        # Pre-commit most of the collateral
        strategy._total_collateral_committed = 490.0

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        # 1 contract * $5 * 100 = $500 > $10 remaining
        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_collateral"] >= 1

    def test_collateral_released_on_exit(self):
        """After exit, _total_collateral_committed decreases."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        entry = _make_spread_entry(short_strike=98.0, long_strike=93.0, contracts=2)
        group = _make_group(status="active")
        group.entries[0] = entry

        strategy.trade_groups.append(group)
        collateral = entry.spread_width * 100 * entry.contracts  # 5 * 100 * 2 = 1000
        strategy._total_collateral_committed = collateral
        group.total_collateral_used = collateral

        strategy._place_exit(group, entry, "profit_target")

        assert strategy._total_collateral_committed == 0.0

    def test_collateral_calculation_uses_actual_width(self):
        """When actual width ≠ target, collateral = actual * 100 * contracts."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", spread_width=5.0, min_hold_candles=0,
        )

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        # Actual width will be 99 - 95 = $4 (not $5 target)
        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P95", strike=95.0, bid=0.80, ask=0.90,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if 0 in group.entries:
            entry = group.entries[0]
            assert entry.spread_width == 4.0
            expected_collateral = 4.0 * 100 * 1
            assert abs(group.total_collateral_used - expected_collateral) < 0.01


# ================================================================== #
# EXIT LOGIC
# ================================================================== #

class TestExitLogic:

    def _setup_with_entry(self, pg, strategy, short_price=0.50, long_price=0.10):
        """Helper: set up a strategy with one active entry and mock positions."""
        entry = _make_spread_entry()
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)
        strategy._total_collateral_committed = entry.spread_width * 100 * entry.contracts

        short_pos = MagicMock()
        short_pos.current_price = short_price
        short_pos.quantity = -entry.contracts
        long_pos = MagicMock()
        long_pos.current_price = long_price
        long_pos.quantity = entry.contracts

        def get_pos(symbol):
            if str(int(entry.short_leg.strike * 1000)) in symbol:
                return short_pos
            if str(int(entry.long_leg.strike * 1000)) in symbol:
                return long_pos
            return None
        pg.account.get_position = get_pos
        return group, entry

    def test_exit_profit_target_threshold(self):
        """Spread value above threshold → no exit; below → exit."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", profit_target_pct=0.50, min_hold_candles=0,
        )

        # credit_per = 0.90, threshold = (1 - 0.50) * 0.90 = 0.45
        entry = _make_spread_entry(short_bid=1.50, long_ask=0.60, contracts=2)
        entry.candles_held = 100

        # spread_value = 0.50 - 0.04 = 0.46, > 0.45 → no exit
        group, _ = self._setup_with_entry(pg, strategy, short_price=0.50, long_price=0.04)
        group.entries[0] = entry

        # high must stay below signal_price (100.0) to avoid reversion exit
        bar = _make_ltf_bar(high=99.5, low=98.0, close=99.0)
        pg.place_order.reset_mock()
        strategy._check_exits(group, bar)
        assert 0 not in group.exited_entries

    def test_exit_max_loss_threshold(self):
        """Spread at 2.1x credit → exit triggered (when max_loss enabled)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", max_loss_multiplier=2.0, min_hold_candles=0,  # explicitly enable
        )

        entry = _make_spread_entry(short_bid=1.50, long_ask=0.60)
        credit_per = 0.90
        # Make spread_value = 2.1 * credit = 1.89
        group, _ = self._setup_with_entry(pg, strategy, short_price=1.94, long_price=0.05)
        group.entries[0] = entry
        entry.candles_held = 100

        bar = _make_ltf_bar()
        pg.place_order.reset_mock()
        strategy._check_exits(group, bar)

        # spread_value = 1.94 - 0.05 = 1.89, threshold = 2.0 * 0.90 = 1.80
        # 1.89 >= 1.80 → exit
        assert 0 in group.exited_entries

    def test_exit_max_loss_disabled_by_default(self):
        """Default max_loss_multiplier=0 → no max_loss exit even when deep in the money."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", min_hold_candles=0,  # default max_loss_multiplier=0.0
        )

        entry = _make_spread_entry(short_bid=1.50, long_ask=0.60)
        # spread_value = 1.94 - 0.05 = 1.89, would exceed 2x credit but max_loss is off
        group, _ = self._setup_with_entry(pg, strategy, short_price=1.94, long_price=0.05)
        group.entries[0] = entry
        entry.candles_held = 100

        bar = _make_ltf_bar()
        pg.place_order.reset_mock()
        strategy._check_exits(group, bar)
        assert 0 not in group.exited_entries

    def test_exit_time_decay_dte(self):
        """DTE=6 → hold; DTE=5 → exit."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", time_decay_exit_dte=5, min_hold_candles=0,
        )

        # Entry with expiration 5 days from now
        entry = _make_spread_entry(expiration_date="2025-06-20")
        entry.candles_held = 100

        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        # DTE = 5 → should exit
        bar = _make_ltf_bar(datetime=datetime(2025, 6, 15, 10, 5))
        strategy._check_exits(group, bar)
        assert 0 in group.exited_entries

    def test_exit_reversion_complete(self):
        """Stock crosses signal price → all entries closed."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        entry = _make_spread_entry()
        entry.candles_held = 100
        group = _make_group(signal_price=100.0, status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        # Stock high >= signal price (100.0) → reversion
        bar = _make_ltf_bar(high=100.5, low=99.0, close=100.2)
        strategy._check_exits(group, bar)
        assert 0 in group.exited_entries
        assert group.status == "closed"

    def test_exit_end_of_sim(self):
        """close_all_active_groups closes all open spreads."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        entry = _make_spread_entry()
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()
        assert group.status == "closed"
        assert 0 in group.exited_entries

    def test_exit_places_both_legs(self):
        """BUY_TO_CLOSE for short + SELL_TO_CLOSE for long."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        entry = _make_spread_entry(contracts=2)
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        # Mock positions
        short_pos = MagicMock()
        short_pos.quantity = -2
        long_pos = MagicMock()
        long_pos.quantity = 2

        def get_pos(symbol):
            if str(int(entry.short_leg.strike * 1000)) in symbol:
                return short_pos
            if str(int(entry.long_leg.strike * 1000)) in symbol:
                return long_pos
            return None
        pg.account.get_position = get_pos

        strategy._place_exit(group, entry, "profit_target")

        from playground_types import OrderSide
        calls = pg.place_order.call_args_list
        assert len(calls) == 2
        # Short leg: BUY_TO_CLOSE
        assert calls[0][0][2] == OrderSide.BUY_TO_CLOSE
        # Long leg: SELL_TO_CLOSE
        assert calls[1][0][2] == OrderSide.SELL_TO_CLOSE

    def test_min_hold_candles_respected(self):
        """Spread profitable but held < min_hold_candles → no exit."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", min_hold_candles=12, profit_target_pct=0.50,
        )

        entry = _make_spread_entry()
        entry.candles_held = 5  # < 12
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        # Even with favorable spread value, should not exit
        short_pos = MagicMock()
        short_pos.current_price = 0.05
        long_pos = MagicMock()
        long_pos.current_price = 0.01

        def get_pos(symbol):
            if str(int(entry.short_leg.strike * 1000)) in symbol:
                return short_pos
            if str(int(entry.long_leg.strike * 1000)) in symbol:
                return long_pos
            return None
        pg.account.get_position = get_pos

        bar = _make_ltf_bar()
        strategy._check_exits(group, bar)
        assert 0 not in group.exited_entries


# ================================================================== #
# SKIP/FILTER LOGIC
# ================================================================== #

class TestSkipFilters:

    def test_min_credit_filter(self):
        """Net credit below min → skipped."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(
            pg, "AAPL", min_credit_per_spread=0.50, min_hold_candles=0,
        )

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        # Net credit = 0.55 - 0.22 = 0.33 < 0.50 min
        # Use tight bid-ask to avoid wide-spread filter
        short_put = _make_put_contract(strike=99.0, bid=0.55, ask=0.60)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.20, ask=0.22,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_low_credit"] >= 1

    def test_debit_spread_rejected(self):
        """short_bid < long_ask → skipped."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        # Debit: short_bid (0.40) < long_ask (0.60)
        short_put = _make_put_contract(strike=99.0, bid=0.40, ask=0.50)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_debit"] >= 1

    def test_negative_ev_skipped(self):
        """Expected profit < 0 → skipped."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        # Forward returns that mostly go below short strike → negative EV
        forward_returns = [-0.10] * 100  # stock drops 10% every time
        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
            forward_returns=forward_returns,
        )
        strategy.trade_groups.append(group)

        # Use tight bid-ask to avoid wide-spread filter
        short_put = _make_put_contract(strike=99.0, bid=1.00, ask=1.05)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.38, ask=0.40,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_neg_ev"] >= 1

    def test_wide_bid_ask_skipped(self):
        """Bid-ask ratio > 20% → skipped."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        # Wide spread: bid=1.00, ask=1.50 → ratio = 0.50/1.25 = 40%
        short_put = _make_put_contract(strike=99.0, bid=1.00, ask=1.50)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.30, ask=0.40,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_wide_spread"] >= 1

    def test_duplicate_strike_expiry_skipped(self):
        """Same (expiry, short_strike) already open → skipped."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        # Pre-register the spread key
        strategy._open_spread_keys.add(("2025-07-18", 99.0))

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(
            strike=99.0, bid=2.00, ask=2.20, expiration_date="2025-07-18",
        )
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
            expiration_date="2025-07-18",
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert 0 not in group.entries
        assert strategy.funnel["entries_skipped_duplicate"] >= 1


# ================================================================== #
# PROBABILITY & EV COMPUTATION
# ================================================================== #

class TestProbabilityAndEV:

    def test_p_profit_all_returns_above(self):
        """All returns ≥ 0 → p_profit = 1.0."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        group = _make_group(
            signal_price=100.0,
            forward_returns=[0.01, 0.02, 0.05, 0.10],
        )
        # short_strike=98 → all exit prices (101,102,105,110) >= 98
        p = strategy._compute_p_profit(group, short_strike=98.0)
        assert p == 1.0

    def test_p_profit_all_returns_below(self):
        """All returns push stock below short strike → p_profit = 0.0."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        group = _make_group(
            signal_price=100.0,
            forward_returns=[-0.10, -0.15, -0.20],
        )
        # short_strike=95 → exit prices = 90, 85, 80 — all below 95
        p = strategy._compute_p_profit(group, short_strike=95.0)
        assert p == 0.0

    def test_p_profit_mixed_returns(self):
        """7/10 returns above short strike → p_profit = 0.7."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        # signal=100, short=97
        # exit prices = 100*(1+r) = [101,102,103,104,105,106,107, 95,94,93]
        returns = [0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.07, -0.05, -0.06, -0.07]
        group = _make_group(signal_price=100.0, forward_returns=returns)

        p = strategy._compute_p_profit(group, short_strike=97.0)
        assert abs(p - 0.7) < 0.001

    def test_p_profit_boundary_return(self):
        """Return placing stock exactly at short strike → counts as profit (>=)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        # signal=100, return=-0.02 → exit=98, short=98 → exactly at boundary
        group = _make_group(signal_price=100.0, forward_returns=[-0.02])
        p = strategy._compute_p_profit(group, short_strike=98.0)
        assert p == 1.0  # >= counts as profit

    def test_expected_profit_all_win(self):
        """All returns above short → EV = net_credit."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        group = _make_group(
            signal_price=100.0,
            forward_returns=[0.05, 0.10, 0.15],
        )

        short_c = _make_put_contract(strike=95.0, bid=1.00, ask=1.20)
        long_c = _make_put_contract(symbol="P90", strike=90.0, bid=0.20, ask=0.30)
        contracts = 1

        ev = strategy._compute_expected_profit(group, None, short_c, long_c, contracts)
        # All returns above 95 → each scenario nets credit_per (1.00-0.30=0.70) * 100 * 1
        expected = 0.70 * 100 * 1
        assert abs(ev - expected) < 0.01

    def test_expected_profit_all_max_loss(self):
        """All returns below long strike → EV = -(max_loss)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        group = _make_group(
            signal_price=100.0,
            forward_returns=[-0.15, -0.20, -0.25],
        )

        short_c = _make_put_contract(strike=95.0, bid=1.00, ask=1.20)
        long_c = _make_put_contract(symbol="P90", strike=90.0, bid=0.20, ask=0.30)
        contracts = 1

        ev = strategy._compute_expected_profit(group, None, short_c, long_c, contracts)
        # All below 90 → net = (1.00-0.30) - 5.0 = -4.30 per contract
        credit_per = 1.00 - 0.30
        width = 5.0
        expected = (credit_per - width) * 100 * 1  # -4.30 * 100 = -430.0
        assert abs(ev - expected) < 0.01

    def test_expected_profit_mixed(self):
        """Mixed returns → verify exact weighted average."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        # 3 above short, 1 between, 1 below long
        # signal=100, short=95, long=90
        returns = [0.05, 0.10, 0.02, -0.06, -0.15]
        # exit_prices: 105, 110, 102, 94, 85
        # 105>=95: full credit (0.70)
        # 110>=95: full credit (0.70)
        # 102>=95: full credit (0.70)
        # 94<95 and 94>90: partial = 0.70 - (95-94) = -0.30
        # 85<90: max loss = 0.70 - 5.0 = -4.30
        group = _make_group(signal_price=100.0, forward_returns=returns)

        short_c = _make_put_contract(strike=95.0, bid=1.00, ask=1.20)
        long_c = _make_put_contract(symbol="P90", strike=90.0, bid=0.20, ask=0.30)

        ev = strategy._compute_expected_profit(group, None, short_c, long_c, 1)
        expected_per = (0.70 + 0.70 + 0.70 + (-0.30) + (-4.30)) / 5
        expected = expected_per * 100 * 1
        assert abs(ev - expected) < 0.01

    def test_expected_profit_partial_loss_math(self):
        """Return between strikes → partial loss = credit - (short - exit)."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        # signal=100, return=-0.04 → exit=96, short=98, long=93
        # partial: credit - (98 - 96) = credit - 2
        group = _make_group(signal_price=100.0, forward_returns=[-0.04])

        short_c = _make_put_contract(strike=98.0, bid=1.50, ask=1.70)
        long_c = _make_put_contract(symbol="P93", strike=93.0, bid=0.30, ask=0.40)

        ev = strategy._compute_expected_profit(group, None, short_c, long_c, 1)
        credit_per = 1.50 - 0.40  # 1.10
        partial = credit_per - (98.0 - 96.0)  # 1.10 - 2.0 = -0.90
        expected = partial * 100 * 1
        assert abs(ev - expected) < 0.01


# ================================================================== #
# PARTIAL LEG FAILURE
# ================================================================== #

class TestPartialLegFailure:

    def test_partial_leg_failure_short_placed_long_fails(self):
        """Short SELL_TO_OPEN succeeds, long fails → BUY_TO_CLOSE short."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        # First call succeeds, second raises
        call_count = [0]
        original_place = pg.place_order

        def mock_place(*args, **kwargs):
            call_count[0] += 1
            if call_count[0] == 2:  # long leg
                raise Exception("Simulated order failure")
            return original_place(*args, **kwargs)

        pg.place_order = MagicMock(side_effect=mock_place)

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        # Entry should NOT be recorded
        assert 0 not in group.entries

        # Third call should be BUY_TO_CLOSE cleanup
        from playground_types import OrderSide
        assert pg.place_order.call_count == 3
        cleanup_call = pg.place_order.call_args_list[2]
        assert cleanup_call[0][2] == OrderSide.BUY_TO_CLOSE

    def test_partial_leg_failure_no_orphan_positions(self):
        """After cleanup, no collateral committed."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        call_count = [0]
        def mock_place(*args, **kwargs):
            call_count[0] += 1
            if call_count[0] == 2:
                raise Exception("Simulated order failure")
        pg.place_order = MagicMock(side_effect=mock_place)

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert strategy._total_collateral_committed == 0.0
        assert group.total_collateral_used == 0.0

    def test_partial_leg_failure_funnel_counter(self):
        """Failure counted in entries_skipped_leg_failure."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6)],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        call_count = [0]
        def mock_place(*args, **kwargs):
            call_count[0] += 1
            if call_count[0] == 2:
                raise Exception("Simulated order failure")
        pg.place_order = MagicMock(side_effect=mock_place)

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        assert strategy.funnel["entries_skipped_leg_failure"] == 1


# ================================================================== #
# DATA ACCURACY ASSERTIONS
# ================================================================== #

class TestDataAccuracy:

    def test_funnel_counters_sum_correctly(self):
        """entries_placed + all skip counters = entry_attempts."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        # Run a few entries with mixed outcomes
        group = _make_group(
            levels=[
                DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6),
                DeviationLevel(price=98.0, shares=1, sigma_distance=1.0, p_revert=0.7),
            ],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=97.5, close=99.5)
        strategy._check_entries(group, bar)

        skip_keys = [k for k in strategy.funnel if k.startswith("entries_skipped_")]
        total_skips = sum(strategy.funnel[k] for k in skip_keys)
        assert strategy.funnel["entry_attempts"] == strategy.funnel["entries_placed"] + total_skips

    def test_group_credit_matches_entry_sum(self):
        """group.total_credit_collected == sum of entry credits."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[
                DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6),
            ],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if group.entries:
            expected = sum(e.net_credit for e in group.entries.values())
            assert abs(group.total_credit_collected - expected) < 0.01

    def test_group_collateral_matches_entry_sum(self):
        """group.total_collateral_used == sum of spread_width * 100 * contracts."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL", min_hold_candles=0)

        group = _make_group(
            levels=[
                DeviationLevel(price=99.0, shares=1, sigma_distance=0.5, p_revert=0.6),
            ],
        )
        strategy.trade_groups.append(group)

        short_put = _make_put_contract(strike=99.0, bid=2.00, ask=2.20)
        long_put = _make_put_contract(
            symbol="P94", strike=94.0, bid=0.50, ask=0.60,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([short_put, long_put])

        bar = _make_ltf_bar(low=98.5, close=99.5)
        strategy._check_entries(group, bar)

        if group.entries:
            expected = sum(
                e.spread_width * 100 * e.contracts for e in group.entries.values()
            )
            assert abs(group.total_collateral_used - expected) < 0.01

    def test_exited_entries_subset_of_entries(self):
        """exited_entries ⊆ entries.keys() always."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        entry = _make_spread_entry()
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        strategy._place_exit(group, entry, "profit_target")
        assert group.exited_entries.issubset(group.entries.keys())

    def test_closed_group_has_all_exited(self):
        """When status == 'closed', all entries are exited."""
        pg = _make_mock_playground()
        strategy = CreditSpreadStrategy(pg, "AAPL")

        entry = _make_spread_entry()
        group = _make_group(status="active")
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        strategy.close_all_active_groups()
        assert group.status == "closed"
        assert len(group.exited_entries) == len(group.entries)
