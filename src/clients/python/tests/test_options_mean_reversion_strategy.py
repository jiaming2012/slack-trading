"""Tests for options_mean_reversion_strategy module."""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Set
from unittest.mock import MagicMock, patch, PropertyMock

import pytest

import numpy as np

from lib.deviation_levels import DeviationLevel, DeviationPlan, _compute_p_revert
from deprecated.options_mean_reversion import (
    OptionsMeanReversionStrategy,
    OptionsTradeGroup,
    OptionsContractEntry,
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
    pg.id = "test-pg-id"
    pg.account = MagicMock()
    pg.account.equity = equity
    pg.account.free_margin = free_margin if free_margin is not None else equity
    pg.account.get_quantity = MagicMock(return_value=0)
    pg.account.get_position = MagicMock(return_value=None)
    pg.place_order = MagicMock()
    pg.fetch_ladder = MagicMock()
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


def _make_option_contract(
    symbol="O:AAPL250620C00099000",
    option_type="call",
    strike=99.0,
    bid=2.50,
    ask=2.70,
    expiration_date="2025-06-20",
    contract_size=100,
):
    """Create a mock OptionLadderContract."""
    contract = MagicMock()
    contract.symbol = symbol
    contract.type = option_type
    contract.strike = strike
    contract.bid = bid
    contract.ask = ask
    contract.expiration_date = expiration_date
    contract.contract_size = contract_size
    return contract


def _make_default_horizons(mean=0.005, stddev=0.02, forward_returns=None):
    """Create default horizons dict for test groups."""
    import numpy as np
    if forward_returns is None:
        rng = np.random.default_rng(42)
        forward_returns = list(rng.normal(mean, stddev, 100))
    horizon = HorizonStats(
        mean=mean,
        stddev=stddev,
        percentiles={"5": -0.03, "10": -0.02, "25": -0.005, "50": 0.005},
        forward_returns=forward_returns,
    )
    return {"1h": horizon}


def _make_group(
    signal_price=100.0,
    direction="bullish",
    levels=None,
    stop_price=95.0,
    status="pending",
    htf_stddev=0.02,
    horizons=None,
):
    """Create an OptionsTradeGroup with a deviation plan."""
    if levels is None:
        levels = [
            DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6),
            DeviationLevel(price=98.0, shares=4, sigma_distance=1.0, p_revert=0.7),
            DeviationLevel(price=97.0, shares=3, sigma_distance=1.5, p_revert=0.8),
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
        horizons = _make_default_horizons(stddev=htf_stddev)
    return OptionsTradeGroup(
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


def _make_ladder_response(contracts):
    """Create a mock ladder response."""
    response = MagicMock()
    response.contracts = contracts
    return response


# ------------------------------------------------------------------ #
# TestTradeGroupCreation
# ------------------------------------------------------------------ #

class TestTradeGroupCreation:

    @patch("deprecated.options_mean_reversion.detect_atomic_signals_on_bar")
    @patch("deprecated.options_mean_reversion.compute_deviation_levels")
    def test_bullish_signal_creates_call_group(
        self, mock_compute, mock_detect,
    ):
        """Positive mean forward return → bullish group created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[
                DeviationLevel(price=99.0, shares=5, sigma_distance=0.5, p_revert=0.7),
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
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar(close=100.0)
        strategy._process_htf_candle(bar)

        assert len(strategy.trade_groups) == 1
        group = strategy.trade_groups[0]
        assert group.direction == "bullish"
        assert group.htf_signal_price == 100.0
        assert group.status == "pending"
        assert strategy.funnel["bullish_signals"] == 1

    @patch("deprecated.options_mean_reversion.detect_atomic_signals_on_bar")
    @patch("deprecated.options_mean_reversion.compute_deviation_levels")
    def test_bearish_signal_creates_put_group(
        self, mock_compute, mock_detect,
    ):
        """Negative mean forward return → bearish group created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[
                DeviationLevel(price=99.0, shares=5, sigma_distance=0.5, p_revert=0.7),
            ],
            signal_price=100.0,
            stop_price=96.0,
            max_potential_loss=1500.0,
            was_truncated=False,
            original_level_count=1,
        )
        mock_compute.return_value = plan

        pg = _make_mock_playground()
        # PDF with negative mean
        pdf = _make_pdf(mean=-0.005)
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar(close=100.0)
        strategy._process_htf_candle(bar)

        assert len(strategy.trade_groups) == 1
        group = strategy.trade_groups[0]
        assert group.direction == "bearish"
        assert strategy.funnel["bearish_signals"] == 1

    @patch("deprecated.options_mean_reversion.detect_atomic_signals_on_bar")
    @patch("deprecated.options_mean_reversion.compute_deviation_levels")
    def test_dedup_same_htf_bar(self, mock_compute, mock_detect):
        """Same signal from same HTF bar → second group rejected."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        plan = DeviationPlan(
            levels=[
                DeviationLevel(price=99.0, shares=5, sigma_distance=0.5, p_revert=0.7),
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
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar(close=100.0)
        strategy._process_htf_candle(bar)
        strategy._process_htf_candle(bar)  # same bar again

        assert len(strategy.trade_groups) == 1
        assert strategy.funnel["groups_skipped_dedup"] == 1

    @patch("deprecated.options_mean_reversion.detect_atomic_signals_on_bar")
    def test_insufficient_samples_ignored(self, mock_detect):
        """Insufficient samples → no group created."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]

        pg = _make_mock_playground()
        pdf = _make_pdf(sufficient=False)
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", pdf=pdf)

        bar = _make_htf_bar(close=100.0)
        strategy._process_htf_candle(bar)

        assert len(strategy.trade_groups) == 0


# ------------------------------------------------------------------ #
# TestEntryLogic
# ------------------------------------------------------------------ #

class TestEntryLogic:

    def test_bullish_dip_triggers_buy_call(self):
        """Candle low <= bullish level → BUY_TO_OPEN call placed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_expected_profit=-float("inf"))

        group = _make_group(signal_price=100.0, direction="bullish")
        strategy.trade_groups.append(group)

        # Mock ladder response with a call contract
        contract = _make_option_contract(
            symbol="O:AAPL250620C00099000",
            option_type="call",
            strike=99.0,
            bid=2.50,
            ask=2.70,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert pg.place_order.called
        call_args = pg.place_order.call_args
        assert call_args[0][0] == "O:AAPL250620C00099000"  # symbol
        assert call_args[0][2].value == "buy_to_open"  # side
        assert call_args[0][3] == "option"  # asset_class
        assert 0 in group.entries
        assert group.entries[0].option_type == "call"

    def test_bearish_rally_triggers_buy_put(self):
        """Candle high >= bearish level → BUY_TO_OPEN put placed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_expected_profit=-float("inf"))

        # Bearish group: levels are above signal price
        levels = [
            DeviationLevel(price=101.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        group = _make_group(
            signal_price=100.0, direction="bearish", levels=levels,
        )
        strategy.trade_groups.append(group)

        contract = _make_option_contract(
            symbol="O:AAPL250620P00101000",
            option_type="put",
            strike=101.0,
            bid=2.30,
            ask=2.50,
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=99.0, high=101.5, close=101.0)
        strategy._check_entries(group, bar)

        assert pg.place_order.called
        call_args = pg.place_order.call_args
        assert "P" in call_args[0][0]  # put symbol
        assert call_args[0][2].value == "buy_to_open"
        assert 0 in group.entries
        assert group.entries[0].option_type == "put"

    def test_premium_budget_caps_contracts(self):
        """Budget limits reduce contracts placed."""
        pg = _make_mock_playground(equity=10_000.0)
        # Very small premium pct: $10000 * 0.001 = $10 budget
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", max_premium_pct=0.001,
        )

        levels = [
            DeviationLevel(price=99.0, shares=10, sigma_distance=0.5, p_revert=0.7),
        ]
        group = _make_group(signal_price=100.0, levels=levels)
        strategy.trade_groups.append(group)

        # Premium $5.00 → cost per contract = $500. Budget = $10. Can't afford any.
        contract = _make_option_contract(strike=99.0, bid=4.80, ask=5.00)
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert not pg.place_order.called
        assert strategy.funnel["entries_skipped_budget"] == 1

    def test_no_contract_at_strike_skips(self):
        """Empty ladder → level skipped."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0)
        strategy.trade_groups.append(group)

        pg.fetch_ladder.return_value = _make_ladder_response([])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert not pg.place_order.called

    def test_wide_spread_skips_contract(self):
        """Wide bid-ask spread (>20%) → contract skipped."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0)
        strategy.trade_groups.append(group)

        # Wide spread: bid=0.50, ask=1.00 → spread/mid = 0.50/0.75 = 67%
        contract = _make_option_contract(strike=99.0, bid=0.50, ask=1.00)
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert not pg.place_order.called
        assert strategy.funnel["entries_skipped_wide_spread"] == 1

    def test_order_attributes_correct(self):
        """Entry order has all required attributes."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_expected_profit=-float("inf"))

        group = _make_group(signal_price=100.0, direction="bullish")
        strategy.trade_groups.append(group)

        contract = _make_option_contract(
            strike=99.0, bid=2.50, ask=2.70,
            expiration_date="2025-06-20",
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        attrs = pg.place_order.call_args[1]["attributes"]
        assert attrs["group_id"] == group.group_id
        assert attrs["action"] == "entry"
        assert attrs["direction"] == "bullish"
        assert attrs["option_type"] == "call"
        assert "premium_per_contract" in attrs
        assert "strike" in attrs
        assert "expiration" in attrs
        assert "p_revert_unbounded" in attrs
        assert "p_revert_bounded" in attrs
        assert "sigma_distance" in attrs
        assert "expected_profit" in attrs
        assert "model_name" in attrs
        assert "strike_strategy" in attrs

    def test_entry_activates_group(self):
        """First entry moves group from pending → active."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_expected_profit=-float("inf"))

        group = _make_group(signal_price=100.0, status="pending")
        strategy.trade_groups.append(group)

        contract = _make_option_contract(strike=99.0, bid=2.50, ask=2.70)
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert group.status == "active"


# ------------------------------------------------------------------ #
# TestExitLogic
# ------------------------------------------------------------------ #

class TestExitLogic:

    def _setup_active_group(self, pg, strategy, direction="bullish"):
        """Helper: set up an active group with one filled entry."""
        group = _make_group(
            signal_price=100.0, direction=direction, status="active",
        )
        entry = OptionsContractEntry(
            contract_symbol="O:AAPL250620C00099000",
            option_type="call" if direction == "bullish" else "put",
            strike=99.0,
            contracts=3,
            premium_per_contract=2.70,
            total_premium=810.0,  # 3 * 2.70 * 100
            level_index=0,
            sigma_distance=0.5,
            p_revert=0.6,
            entry_stock_price=99.5,
            expiration_date="2025-06-20",
        )
        group.entries[0] = entry
        strategy.trade_groups.append(group)
        return group, entry

    def test_bullish_reversion_exits_all(self):
        """Bar high >= signal_price → all entries closed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_hold_candles=0)

        # Mock position exists
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 5.00
        pg.account.get_position.return_value = pos

        group, entry = self._setup_active_group(pg, strategy, "bullish")

        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        call_args = pg.place_order.call_args
        assert call_args[0][2].value == "sell_to_close"
        assert 0 in group.exited_entries
        assert group.status == "closed"

    def test_bearish_reversion_exits_all(self):
        """Bar low <= signal_price → bearish group closed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_hold_candles=0)

        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 2.80  # below profit target (needs 1.5 * 750/300 = 3.75)
        pg.account.get_position.return_value = pos

        levels = [
            DeviationLevel(price=101.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        group = _make_group(
            signal_price=100.0, direction="bearish", levels=levels, status="active",
        )
        entry = OptionsContractEntry(
            contract_symbol="O:AAPL250620P00101000",
            option_type="put",
            strike=101.0,
            contracts=3,
            premium_per_contract=2.50,
            total_premium=750.0,
            level_index=0,
            sigma_distance=0.5,
            p_revert=0.6,
            entry_stock_price=101.0,
            expiration_date="2025-06-20",
        )
        group.entries[0] = entry
        strategy.trade_groups.append(group)

        bar = _make_ltf_bar(low=99.5, high=101.0, close=100.0)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert group.status == "closed"
        assert strategy.funnel["exits_reversion"] == 1

    def test_profit_target_exits_single(self):
        """Option value >= 150% of premium → that entry closed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", profit_target_pct=0.50, min_hold_candles=0,
        )

        group, entry = self._setup_active_group(pg, strategy, "bullish")

        # Position worth 150% of premium: 810 * 1.5 = 1215
        # current_price * contracts * 100 >= 1215
        # current_price >= 1215 / (3 * 100) = 4.05
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 4.10  # 4.10 * 3 * 100 = 1230 > 1215
        pg.account.get_position.return_value = pos

        # Price NOT at signal yet (no reversion)
        bar = _make_ltf_bar(low=99.0, high=99.8, close=99.5)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries
        assert strategy.funnel["exits_profit"] == 1

    def test_time_decay_forces_exit(self):
        """DTE <= threshold → entry force-closed."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", time_decay_exit_dte=3, min_hold_candles=0,
        )

        group, entry = self._setup_active_group(pg, strategy)

        # Position exists but not profitable
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 1.00  # not profitable
        pg.account.get_position.return_value = pos

        # Bar datetime very close to expiration (2025-06-20)
        bar = _make_ltf_bar(
            low=98.0, high=99.0, close=98.5,
            datetime=datetime(2025, 6, 18, 10, 5),  # 2 DTE
        )
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries
        assert strategy.funnel["exits_time_decay"] == 1

    def test_end_of_sim_closes_all(self):
        """close_all_active_groups closes remaining entries."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 2.00
        pg.account.get_position.return_value = pos

        group, entry = self._setup_active_group(pg, strategy)

        strategy.close_all_active_groups()

        assert pg.place_order.called
        assert group.status == "closed"

    def test_no_position_skips_exit(self):
        """If server has no position, mark as exited without placing order."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_hold_candles=0)

        # No position
        pg.account.get_position.return_value = None

        group, entry = self._setup_active_group(pg, strategy)

        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        # Should mark as exited but not place order
        assert not pg.place_order.called
        assert 0 in group.exited_entries

    def test_min_hold_blocks_early_exit(self):
        """Reversion within min_hold_candles window does NOT exit."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_hold_candles=6)

        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 5.00
        pg.account.get_position.return_value = pos

        group, entry = self._setup_active_group(pg, strategy, "bullish")
        entry.candles_held = 2  # only 2 candles held, need 6

        # Price reverts to signal
        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        # Should NOT exit — min hold not met
        assert not pg.place_order.called
        assert 0 not in group.exited_entries

    def test_min_hold_allows_exit_after_threshold(self):
        """Reversion after min_hold_candles → exits normally."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL", min_hold_candles=6)

        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 5.00
        pg.account.get_position.return_value = pos

        group, entry = self._setup_active_group(pg, strategy, "bullish")
        entry.candles_held = 6  # exactly at threshold

        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries


# ------------------------------------------------------------------ #
# TestStrikeStrategy
# ------------------------------------------------------------------ #

class TestStrikeStrategy:

    def test_model_strategy_shallow_dip_itm(self):
        """σ < 1.5 → model strategy selects ITM strike (below level)."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="model",
        )

        level = DeviationLevel(
            price=98.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=98.5)
        # ITM: 98.0 - 0.5 * (100 - 98) = 98.0 - 1.0 = 97.0
        assert target == pytest.approx(97.0, abs=0.01)
        assert target < level.price  # below dip level (ITM for call)

    def test_model_strategy_medium_dip_deeper_itm(self):
        """1.5 ≤ σ < 2.5 → deeper ITM below dip level."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="model",
        )

        level = DeviationLevel(
            price=96.0, shares=4, sigma_distance=2.0, p_revert=0.75,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=96.5)
        # 96.0 - 1.0 * (100 - 96) = 96.0 - 4.0 = 92.0
        assert target == pytest.approx(92.0, abs=0.01)
        assert target < level.price

    def test_model_strategy_deep_dip_very_itm(self):
        """σ ≥ 2.5 → very deep ITM strike."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="model",
        )

        level = DeviationLevel(
            price=94.0, shares=3, sigma_distance=3.0, p_revert=0.85,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=94.5)
        # 94.0 - 1.5 * |100 - 94| = 94.0 - 9.0 = 85.0
        assert target == pytest.approx(85.0, abs=0.01)
        assert target < level.price

    def test_model_strategy_bearish_itm(self):
        """Bearish model strategy: puts ITM above level price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="model",
        )

        level = DeviationLevel(
            price=102.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bearish")

        target = strategy._compute_target_strike(level, group, stock_price=101.5)
        # 102.0 + 0.5 * (102 - 100) = 102.0 + 1.0 = 103.0
        assert target == pytest.approx(103.0, abs=0.01)
        assert target > level.price  # above level (ITM for put)

    def test_model_strategy_intrinsic_at_signal(self):
        """Model strategy ensures meaningful intrinsic at signal price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="model",
        )

        level = DeviationLevel(
            price=98.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=98.5)
        # Intrinsic at signal = signal - strike = 100 - 97 = 3 per share
        intrinsic_at_signal = 100.0 - target
        assert intrinsic_at_signal > 0, "Call must have intrinsic at signal"
        # Intrinsic should exceed the level-to-signal distance (the move)
        assert intrinsic_at_signal > (100.0 - 98.0)

    def test_atm_strategy(self):
        """ATM strategy always picks strike near current stock price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="atm",
        )

        level = DeviationLevel(
            price=98.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=98.5)
        assert target == 98.5

    def test_otm_strategy_bullish(self):
        """OTM strategy for calls: strike above current stock price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="otm",
        )

        level = DeviationLevel(
            price=98.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bullish")

        target = strategy._compute_target_strike(level, group, stock_price=98.5)
        # 98.5 + 0.5 * (100 - 98.5) = 98.5 + 0.75 = 99.25
        assert target == pytest.approx(99.25, abs=0.01)
        assert target > 98.5  # above stock price

    def test_otm_strategy_bearish(self):
        """OTM strategy for puts: strike below current stock price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="otm",
        )

        level = DeviationLevel(
            price=102.0, shares=3, sigma_distance=1.0, p_revert=0.6,
        )
        group = _make_group(signal_price=100.0, direction="bearish")

        target = strategy._compute_target_strike(level, group, stock_price=101.5)
        # 101.5 - 0.5 * (101.5 - 100) = 101.5 - 0.75 = 100.75
        assert target == pytest.approx(100.75, abs=0.01)
        assert target < 101.5  # below stock price


# ------------------------------------------------------------------ #
# TestExpectedProfit
# ------------------------------------------------------------------ #

class TestExpectedProfit:

    def _make_horizons_with_p_revert(self, p_revert, direction="bullish"):
        """Create horizons where time-bounded p_revert = p_revert.

        For bullish: P(return >= 0) = p_revert
        For bearish: P(return <= 0) = p_revert
        """
        n = 100
        n_revert = int(p_revert * n)
        if direction == "bullish":
            returns = [0.01] * n_revert + [-0.01] * (n - n_revert)
        else:
            returns = [-0.01] * n_revert + [0.01] * (n - n_revert)
        horizon = HorizonStats(
            mean=0.005, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.005},
            forward_returns=returns,
        )
        return {"2d": horizon}

    def test_expected_profit_call(self):
        """Expected profit for calls via distribution integration."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        horizons = self._make_horizons_with_p_revert(0.6, "bullish")
        group = _make_group(
            signal_price=100.0, direction="bullish", horizons=horizons,
        )
        level = group.deviation_plan.levels[0]  # price=99, shares=3

        contract = _make_option_contract(strike=99.0, ask=2.70)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # 60 returns of +0.01: exit=101, intrinsic=max(0,101-99)=2.0
        # 40 returns of -0.01: exit=99,  intrinsic=max(0,99-99)=0.0
        # E[intrinsic] = 60*2.0/100 = 1.20
        # E[value] = 1.20 * 100 * 3 = 360
        # E[profit] = 360 - 810 = -450
        assert expected == pytest.approx(-450.0, abs=0.01)

    def test_expected_profit_put(self):
        """Expected profit for puts via distribution integration."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        levels = [
            DeviationLevel(price=101.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        horizons = self._make_horizons_with_p_revert(0.6, "bearish")
        group = _make_group(
            signal_price=100.0, direction="bearish", levels=levels,
            horizons=horizons,
        )
        level = group.deviation_plan.levels[0]

        contract = _make_option_contract(
            option_type="put", strike=101.0, ask=2.50,
        )

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # 60 returns of -0.01: exit=99,  intrinsic=max(0,101-99)=2.0
        # 40 returns of +0.01: exit=101, intrinsic=max(0,101-101)=0.0
        # E[intrinsic] = 60*2.0/100 = 1.20
        # E[value] = 1.20 * 100 * 3 = 360
        # E[profit] = 360 - 750 = -390
        assert expected == pytest.approx(-390.0, abs=0.01)

    def test_expected_profit_deep_itm_call(self):
        """Deep ITM call benefits from distribution: even partial reversion has intrinsic."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        horizons = self._make_horizons_with_p_revert(0.9, "bullish")
        group = _make_group(
            signal_price=110.0, direction="bullish", horizons=horizons,
        )
        level = DeviationLevel(
            price=100.0, shares=2, sigma_distance=2.5, p_revert=0.9,
        )

        contract = _make_option_contract(strike=95.0, ask=8.00)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=2,
        )

        # 90 returns of +0.01: exit=111.10, intrinsic=16.10
        # 10 returns of -0.01: exit=108.90, intrinsic=13.90 (still ITM!)
        # E[intrinsic] = (90*16.10 + 10*13.90)/100 = 15.88
        # E[value] = 15.88 * 100 * 2 = 3176
        # E[profit] = 3176 - 1600 = 1576
        assert expected == pytest.approx(1576.0, abs=1.0)


# ------------------------------------------------------------------ #
# TestContractSelection
# ------------------------------------------------------------------ #

class TestContractSelection:

    def test_selects_closest_strike(self):
        """Selects call with strike closest to target."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", strike_strategy="atm",
        )

        group = _make_group(signal_price=100.0, direction="bullish")
        level = DeviationLevel(
            price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6,
        )

        contracts = [
            _make_option_contract(symbol="O:C97", option_type="call", strike=97.0, bid=4.0, ask=4.2),
            _make_option_contract(symbol="O:C98", option_type="call", strike=98.0, bid=3.0, ask=3.2),
            _make_option_contract(symbol="O:C99", option_type="call", strike=99.0, bid=2.0, ask=2.2),
            _make_option_contract(symbol="O:C100", option_type="call", strike=100.0, bid=1.0, ask=1.2),
        ]

        bar = {"close": 98.5}
        selected = strategy._select_contract(level, contracts, group, bar)

        # ATM strategy: target = stock_price = 98.5 → closest is 98 or 99
        assert selected.strike in (98.0, 99.0)

    def test_filters_by_option_type(self):
        """Bullish group only selects calls, not puts."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0, direction="bullish")
        level = DeviationLevel(
            price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6,
        )

        contracts = [
            _make_option_contract(option_type="put", strike=99.0, bid=2.0, ask=2.2),
        ]

        bar = {"close": 99.0}
        selected = strategy._select_contract(level, contracts, group, bar)

        assert selected is None

    def test_returns_none_for_empty(self):
        """Empty contracts list → None."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        group = _make_group(signal_price=100.0)
        level = DeviationLevel(
            price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6,
        )

        bar = {"close": 99.0}
        selected = strategy._select_contract(level, [], group, bar)

        assert selected is None


# ------------------------------------------------------------------ #
# TestExpirationDays
# ------------------------------------------------------------------ #

class TestExpirationDays:

    def test_computes_friday_expirations(self):
        """Returns DTE values corresponding to Fridays."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", target_dte=14, min_dte=5, max_dte=30,
        )

        # Wednesday 2025-06-11
        current_ts = datetime(2025, 6, 11, 10, 0)
        result = strategy._compute_expiration_days(current_ts)

        assert len(result) >= 1
        # All should be Fridays
        for dte in result:
            future = current_ts + __import__("datetime").timedelta(days=dte)
            assert future.weekday() == 4  # Friday

    def test_fallback_to_target_dte(self):
        """If no Fridays found, returns target_dte."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", target_dte=14, min_dte=13, max_dte=14,
        )

        # The range [13, 14] may not contain a Friday
        current_ts = datetime(2025, 6, 11, 10, 0)  # Wednesday
        # 13 days = Tuesday Jun 24, 14 days = Wednesday Jun 25 → no Friday
        result = strategy._compute_expiration_days(current_ts)

        # Should fall back to [14]
        assert result == [14]


# ------------------------------------------------------------------ #
# TestLogSummary
# ------------------------------------------------------------------ #

class TestLogSummary:

    def test_log_summary_no_crash(self):
        """log_summary runs without error on empty state."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")
        strategy.log_summary()  # should not raise

    def test_log_summary_with_groups(self):
        """log_summary runs with active/closed groups."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        g1 = _make_group(direction="bullish", status="active")
        g2 = _make_group(direction="bearish", status="closed")
        g2.group_id = "test5678"
        g2.total_premium_spent = 500.0
        strategy.trade_groups.extend([g1, g2])

        strategy.log_summary()  # should not raise


# ------------------------------------------------------------------ #
# TestMinExpectedProfitFilter
# ------------------------------------------------------------------ #

class TestMinExpectedProfitFilter:

    def test_negative_expected_profit_skips_entry(self):
        """Entry with expected_profit < min_expected_profit is skipped."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", min_expected_profit=50.0,
        )

        # Level at $99 with signal at $100 — with a cheap OTM call,
        # expected profit will be negative
        levels = [
            DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        group = _make_group(signal_price=100.0, levels=levels)
        strategy.trade_groups.append(group)

        # Call at strike $99.5 → intrinsic at signal = max(0, 100-99.5)*100*3 = $150
        # expected = 0.6 * 150 - 3 * 2.70 * 100 = 90 - 810 = -720
        contract = _make_option_contract(
            strike=99.5, bid=2.50, ask=2.70, option_type="call",
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        assert not pg.place_order.called
        assert strategy.funnel["entries_skipped_expected_profit"] == 1

    def test_positive_expected_profit_allows_entry(self):
        """Entry with expected_profit >= min_expected_profit proceeds."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", min_expected_profit=50.0,
        )

        # Create horizons with 90% bounded p_revert (bullish: P(return >= 0) = 0.9)
        returns_2d = [0.01] * 90 + [-0.01] * 10
        horizon_2d = HorizonStats(
            mean=0.008, stddev=0.02,
            percentiles={"5": -0.01, "50": 0.01},
            forward_returns=returns_2d,
        )

        # Deep ITM call: strike $90, signal $110
        levels = [
            DeviationLevel(price=100.0, shares=2, sigma_distance=2.5, p_revert=0.9),
        ]
        group = _make_group(
            signal_price=110.0, levels=levels,
            horizons={"2d": horizon_2d},
        )
        strategy.trade_groups.append(group)

        # Distribution: 90 returns of +0.01 → exit=111.10, intrinsic=21.10
        #               10 returns of -0.01 → exit=108.90, intrinsic=18.90
        # E[intrinsic] = (90*21.10+10*18.90)/100 = 20.88
        # E[value] = 20.88*100*2 = 4176, E[profit] = 4176-2400 = 1776 >> $50
        contract = _make_option_contract(
            strike=90.0, bid=11.50, ask=12.00, option_type="call",
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=99.5, high=101.0, close=100.0)
        strategy._check_entries(group, bar)

        assert pg.place_order.called
        assert strategy.funnel["entries_placed"] == 1

    def test_zero_min_expected_profit_disables_filter(self):
        """min_expected_profit=0 allows entries with any non-negative expected profit."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", min_expected_profit=-float("inf"),
        )

        levels = [
            DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        group = _make_group(signal_price=100.0, levels=levels)
        strategy.trade_groups.append(group)

        contract = _make_option_contract(
            strike=99.0, bid=2.50, ask=2.70, option_type="call",
        )
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.5)
        strategy._check_entries(group, bar)

        # Should proceed despite negative expected profit
        assert pg.place_order.called


# ------------------------------------------------------------------ #
# TestOptionPriceBasedExit
# ------------------------------------------------------------------ #

class TestOptionPriceBasedExit:

    def _setup_active_group_with_entry(self, pg, strategy, direction="bullish"):
        """Helper: active group with one filled entry."""
        group = _make_group(
            signal_price=100.0, direction=direction, status="active",
        )
        entry = OptionsContractEntry(
            contract_symbol="O:AAPL250620C00097000",
            option_type="call" if direction == "bullish" else "put",
            strike=97.0,
            contracts=3,
            premium_per_contract=4.00,
            total_premium=1200.0,  # 3 * 4.00 * 100
            level_index=0,
            sigma_distance=1.0,
            p_revert=0.6,
            entry_stock_price=98.0,
            expiration_date="2025-06-20",
            candles_held=10,  # past min hold
        )
        group.entries[0] = entry
        strategy.trade_groups.append(group)
        return group, entry

    def test_profit_target_exits_before_reversion(self):
        """Profit target (primary) fires even when stock hasn't fully reverted."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", profit_target_pct=0.50, min_hold_candles=0,
        )

        group, entry = self._setup_active_group_with_entry(pg, strategy)

        # Option at 150% of premium: 1200 * 1.5 = 1800
        # current_price * 3 * 100 >= 1800 → current_price >= 6.0
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 6.50  # 6.50 * 3 * 100 = 1950 > 1800
        pg.account.get_position.return_value = pos

        # Stock NOT at signal (no reversion)
        bar = _make_ltf_bar(low=98.0, high=99.5, close=99.0)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries
        assert strategy.funnel["exits_profit"] == 1

    def test_reversion_exits_when_profit_target_not_met(self):
        """Reversion (secondary) fires when profit target wasn't hit."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", profit_target_pct=0.50, min_hold_candles=0,
        )

        group, entry = self._setup_active_group_with_entry(pg, strategy)

        # Option NOT at profit target
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 4.50  # 4.50 * 3 * 100 = 1350 < 1800
        pg.account.get_position.return_value = pos

        # Stock IS at signal (reversion complete)
        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries
        assert strategy.funnel["exits_reversion"] == 1

    def test_profit_target_takes_priority_over_reversion(self):
        """When both conditions met, profit target fires (checked first)."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL", profit_target_pct=0.50, min_hold_candles=0,
        )

        group, entry = self._setup_active_group_with_entry(pg, strategy)

        # Option at profit target AND stock at signal
        pos = MagicMock()
        pos.quantity = 3
        pos.current_price = 7.00  # 7.00 * 3 * 100 = 2100 > 1800
        pg.account.get_position.return_value = pos

        # Stock also at signal
        bar = _make_ltf_bar(low=99.0, high=101.0, close=100.5)
        strategy._check_exits(group, bar)

        assert pg.place_order.called
        assert 0 in group.exited_entries
        # Should be profit exit, not reversion
        assert strategy.funnel["exits_profit"] == 1
        assert strategy.funnel["exits_reversion"] == 0


# ------------------------------------------------------------------ #
# Time-bounded reversion probability
# ------------------------------------------------------------------ #

class TestTimeBoundedReversion:
    """Tests proving the need for time-bounded reversion probability
    and verifying the implementation correctness."""

    def test_unbounded_p_revert_overestimates_options_profit(self):
        """Demonstrate that unbounded p_revert (from deviation_levels)
        yields misleadingly positive expected profit, while the time-bounded
        version correctly shows the trade is unprofitable.

        Dummy data: 100 returns where most don't dip as far as the level
        (high unbounded p_revert) but only 40% actually revert to signal
        price (low bounded p_revert).
        """
        # 60 slightly negative returns (didn't revert to 0, but > -0.02)
        # 10 deep dips (< -0.02)
        # 30 positive returns (reverted above signal)
        returns = [-0.005] * 60 + [-0.03] * 10 + [0.008] * 30
        returns_arr = np.array(returns)

        dip_magnitude = 0.02  # σ=1 * stddev=0.02

        # Unbounded: P(return >= -0.02) — 90 of 100 are >= -0.02
        unbounded = _compute_p_revert(returns_arr, dip_magnitude)
        assert unbounded == pytest.approx(0.90), f"Expected ~0.90, got {unbounded}"

        # Bounded: P(return >= 0) — only 30 of 100
        bounded = float(np.mean(returns_arr >= 0))
        assert bounded == pytest.approx(0.30), f"Expected ~0.30, got {bounded}"

        # Impact on expected profit for an options trade:
        # signal=100, strike=98 (ITM call), premium=$2.00, 3 contracts
        signal_price = 100.0
        strike = 98.0
        premium = 2.00
        contracts = 3
        total_premium = contracts * premium * 100  # $600
        intrinsic_at_signal = max(0, signal_price - strike) * 100 * contracts  # $600

        # With unbounded p_revert (0.90): looks slightly unprofitable but close
        # 0.90 * 600 - 600 = 540 - 600 = -60 ... let's use premium=$1.50 instead
        premium = 1.50
        total_premium = contracts * premium * 100  # $450

        # With unbounded p_revert: looks profitable
        ep_unbounded = unbounded * intrinsic_at_signal - total_premium
        assert ep_unbounded > 0, (
            f"Unbounded expected profit should be positive: {ep_unbounded}"
            f" (= {unbounded} * {intrinsic_at_signal} - {total_premium})"
        )

        # With bounded p_revert: correctly shows unprofitable
        ep_bounded = bounded * intrinsic_at_signal - total_premium
        assert ep_bounded < 0, (
            f"Bounded expected profit should be negative: {ep_bounded}"
            f" (= {bounded} * {intrinsic_at_signal} - {total_premium})"
        )

    def test_time_bounded_p_revert_uses_longest_horizon(self):
        """Verify _compute_time_bounded_p_revert picks the longest
        available horizon, not the HTF horizon."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # 1h horizon: 80% reverted (high)
        returns_1h = [0.01] * 80 + [-0.01] * 20
        horizon_1h = HorizonStats(
            mean=0.005, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.005},
            forward_returns=returns_1h,
        )

        # 2d horizon: 40% reverted (low — more realistic for options)
        returns_2d = [0.01] * 40 + [-0.01] * 60
        horizon_2d = HorizonStats(
            mean=-0.002, stddev=0.03,
            percentiles={"5": -0.05, "50": -0.002},
            forward_returns=returns_2d,
        )

        group = _make_group(
            horizons={"1h": horizon_1h, "2d": horizon_2d},
        )

        result = strategy._compute_time_bounded_p_revert(group)
        # Should use 2d (52 bars) not 1h (4 bars)
        assert result == pytest.approx(0.40), (
            f"Expected ~0.40 (from 2d horizon), got {result}"
        )

    def test_time_bounded_expected_profit_filters_bad_entries(self):
        """End-to-end: min_expected_profit filter correctly rejects entries
        that look profitable with unbounded p_revert but are unprofitable
        with time-bounded p_revert."""
        pg = _make_mock_playground()

        # 2d horizon where only 35% revert
        returns_2d = [0.01] * 35 + [-0.01] * 65
        horizon_2d = HorizonStats(
            mean=-0.003, stddev=0.025,
            percentiles={"5": -0.04, "50": -0.003},
            forward_returns=returns_2d,
        )
        # 1h horizon (used for signal detection) with positive mean
        returns_1h = [0.01] * 70 + [-0.01] * 30
        horizon_1h = HorizonStats(
            mean=0.004, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.004},
            forward_returns=returns_1h,
        )
        horizons = {"1h": horizon_1h, "2d": horizon_2d}

        strategy = OptionsMeanReversionStrategy(
            pg, "AAPL",
            min_expected_profit=50.0,  # default filter
        )

        group = _make_group(
            signal_price=100.0,
            levels=[
                DeviationLevel(price=99.0, shares=3, sigma_distance=0.5, p_revert=0.95),
            ],
            horizons=horizons,
        )

        # ITM call: strike=97, premium=$4.00, 3 contracts
        # intrinsic_at_signal = (100 - 97) * 100 * 3 = $900
        # total_premium = 3 * 4.00 * 100 = $1200
        # bounded p_revert = 0.35
        # expected_profit = 0.35 * 900 - 1200 = 315 - 1200 = -$885
        # This should be filtered out (< $50)
        contract = _make_option_contract(strike=97.0, bid=3.80, ask=4.00)
        pg.fetch_ladder.return_value = _make_ladder_response([contract])

        # Low breaches level price (99.0) → triggers entry attempt
        bar = _make_ltf_bar(low=98.5, high=100.0, close=99.0)

        strategy.trade_groups.append(group)

        strategy._check_entries(group, bar)

        assert not pg.place_order.called, (
            "Entry should have been filtered by min_expected_profit"
        )
        assert strategy.funnel["entries_skipped_expected_profit"] == 1

    def test_time_bounded_p_revert_fallback_single_horizon(self):
        """If only 1h horizon exists, use it as fallback."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        returns_1h = [0.01] * 55 + [-0.01] * 45
        horizon_1h = HorizonStats(
            mean=0.001, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.001},
            forward_returns=returns_1h,
        )

        group = _make_group(horizons={"1h": horizon_1h})

        result = strategy._compute_time_bounded_p_revert(group)
        assert result == pytest.approx(0.55), (
            f"Expected ~0.55 (from 1h fallback), got {result}"
        )

    def test_bearish_time_bounded_p_revert(self):
        """For bearish signals, time-bounded p_revert measures P(return <= 0)
        since reversion means the stock price fell back to signal."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # 2d returns: 30% went negative (reverted for bearish), 70% stayed positive
        returns_2d = [-0.01] * 30 + [0.01] * 70
        horizon_2d = HorizonStats(
            mean=0.004, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.004},
            forward_returns=returns_2d,
        )

        group = _make_group(
            direction="bearish",
            signal_price=100.0,
            levels=[
                DeviationLevel(price=101.0, shares=3, sigma_distance=0.5, p_revert=0.6),
            ],
            horizons={"2d": horizon_2d},
        )

        result = strategy._compute_time_bounded_p_revert(group)
        assert result == pytest.approx(0.30), (
            f"Expected ~0.30 for bearish (P(return <= 0)), got {result}"
        )

    def test_prefers_2w_over_shorter_horizons(self):
        """When 1h, 1d, and 2w horizons are available, uses 2w."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # 1h: 80% revert, 1d: 60% revert, 2w: 45% revert
        horizon_1h = HorizonStats(
            mean=0.005, stddev=0.02,
            percentiles={"5": -0.03, "50": 0.005},
            forward_returns=[0.01] * 80 + [-0.01] * 20,
        )
        horizon_1d = HorizonStats(
            mean=0.003, stddev=0.025,
            percentiles={"5": -0.04, "50": 0.003},
            forward_returns=[0.01] * 60 + [-0.01] * 40,
        )
        horizon_2w = HorizonStats(
            mean=0.001, stddev=0.03,
            percentiles={"5": -0.05, "50": 0.001},
            forward_returns=[0.01] * 45 + [-0.01] * 55,
        )

        group = _make_group(
            horizons={"1h": horizon_1h, "1d": horizon_1d, "2w": horizon_2w},
        )

        result = strategy._compute_time_bounded_p_revert(group)
        assert result == pytest.approx(0.45), (
            f"Expected ~0.45 (from 2w horizon), got {result}"
        )


# ------------------------------------------------------------------ #
# Distribution-based expected profit
# ------------------------------------------------------------------ #

class TestExpectedProfitDistribution:
    """Tests for the distribution-integrated expected profit formula."""

    def test_partial_reversion_positive_ev(self):
        """Partial reversion still produces positive intrinsic for ITM calls.

        Unlike the binary model (which assigns 0 value to non-reversion),
        the distribution model recognizes that a stock partially reverting
        still leaves an ITM call with intrinsic value.
        """
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # All 100 returns are -0.01 (partial reversion: stock went from
        # dip to signal-1%). None fully revert but all end above strike.
        returns = [-0.01] * 100  # exit = 100*(1-0.01) = 99.0
        horizon = HorizonStats(
            mean=-0.01, stddev=0.005,
            percentiles={"5": -0.01, "50": -0.01},
            forward_returns=returns,
        )
        group = _make_group(
            signal_price=100.0, direction="bullish",
            horizons={"2d": horizon},
        )
        level = group.deviation_plan.levels[0]

        # Deep ITM call: strike=95
        contract = _make_option_contract(strike=95.0, ask=3.00)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # exit=99.0, intrinsic=max(0, 99-95)=4.0 for ALL returns
        # E[value] = 4.0 * 100 * 3 = 1200
        # E[profit] = 1200 - 900 = 300
        assert expected == pytest.approx(300.0, abs=1.0)
        assert expected > 0, "Partial reversion should be profitable for deep ITM"

    def test_overshoot_increases_ev(self):
        """Returns that overshoot signal price produce higher EV."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # All returns are +0.05 (stock overshoots signal by 5%)
        returns = [0.05] * 100  # exit = 100*1.05 = 105.0
        horizon = HorizonStats(
            mean=0.05, stddev=0.01,
            percentiles={"5": 0.04, "50": 0.05},
            forward_returns=returns,
        )
        group = _make_group(
            signal_price=100.0, direction="bullish",
            horizons={"2d": horizon},
        )
        level = group.deviation_plan.levels[0]
        contract = _make_option_contract(strike=99.0, ask=2.00)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # exit=105.0, intrinsic=max(0, 105-99)=6.0
        # E[value] = 6.0 * 100 * 3 = 1800
        # E[profit] = 1800 - 600 = 1200
        assert expected == pytest.approx(1200.0, abs=1.0)

    def test_otm_strike_low_ev(self):
        """Deeply OTM strike with mild returns has low/negative EV."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # Returns between -0.01 and +0.01 — stock stays near signal
        returns = [-0.005] * 50 + [0.005] * 50
        horizon = HorizonStats(
            mean=0.0, stddev=0.005,
            percentiles={"5": -0.005, "50": 0.0},
            forward_returns=returns,
        )
        group = _make_group(
            signal_price=100.0, direction="bullish",
            horizons={"2d": horizon},
        )
        level = group.deviation_plan.levels[0]

        # OTM call: strike=105 (above signal!)
        contract = _make_option_contract(strike=105.0, ask=0.50)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # exit prices: 50 at 99.50, 50 at 100.50 — both below strike 105
        # intrinsic = 0 for all returns
        # E[value] = 0
        # E[profit] = 0 - 150 = -150
        assert expected == pytest.approx(-150.0, abs=1.0)

    def test_bearish_put_distribution(self):
        """Distribution integration works for bearish puts."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # Stock drops 2%: put becomes profitable
        returns = [-0.02] * 70 + [0.01] * 30
        horizon = HorizonStats(
            mean=-0.011, stddev=0.015,
            percentiles={"5": -0.02, "50": -0.02},
            forward_returns=returns,
        )
        levels = [
            DeviationLevel(price=101.0, shares=3, sigma_distance=0.5, p_revert=0.6),
        ]
        group = _make_group(
            signal_price=100.0, direction="bearish", levels=levels,
            horizons={"2d": horizon},
        )
        level = group.deviation_plan.levels[0]

        # ITM put: strike=102
        contract = _make_option_contract(
            option_type="put", strike=102.0, ask=3.00,
        )

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=3,
        )

        # 70 returns of -0.02: exit=98.0, intrinsic=max(0,102-98)=4.0
        # 30 returns of +0.01: exit=101.0, intrinsic=max(0,102-101)=1.0
        # E[intrinsic] = (70*4.0 + 30*1.0)/100 = 3.10
        # E[value] = 3.10 * 100 * 3 = 930
        # E[profit] = 930 - 900 = 30
        assert expected == pytest.approx(30.0, abs=1.0)

    def test_horizon_selection_for_dte(self):
        """_select_horizon_for_dte picks the best horizon for the option's DTE."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        h_1h = HorizonStats(mean=0.0, stddev=0.01, percentiles={}, forward_returns=[0.01])
        h_1d = HorizonStats(mean=0.0, stddev=0.01, percentiles={}, forward_returns=[0.01])
        h_1w = HorizonStats(mean=0.0, stddev=0.01, percentiles={}, forward_returns=[0.01])
        h_2w = HorizonStats(mean=0.0, stddev=0.01, percentiles={}, forward_returns=[0.01])

        group = _make_group(
            horizons={"1h": h_1h, "1d": h_1d, "1w": h_1w, "2w": h_2w},
        )

        # DTE=7 days → should pick 1w (5 trading days)
        result = strategy._select_horizon_for_dte(group, dte_days=7)
        assert result is h_1w

        # DTE=14 days → should pick 2w (10 trading days)
        result = strategy._select_horizon_for_dte(group, dte_days=14)
        assert result is h_2w

        # DTE=3 days → should pick 1d (1 trading day)
        result = strategy._select_horizon_for_dte(group, dte_days=3)
        assert result is h_1d

        # DTE=0 → should pick 1h (smallest, ~0.15 days ≤ 0? no)
        # Actually 1h = 0.15 days > 0, so nothing fits → falls back to longest
        result = strategy._select_horizon_for_dte(group, dte_days=0)
        assert result is h_2w  # fallback to longest

    def test_degenerate_matches_binary_at_signal(self):
        """When all returns end exactly at 0, distribution EV equals
        binary model with p_revert=1.0 and intrinsic at signal price."""
        pg = _make_mock_playground()
        strategy = OptionsMeanReversionStrategy(pg, "AAPL")

        # All returns are exactly 0 (stock at signal price)
        returns = [0.0] * 100
        horizon = HorizonStats(
            mean=0.0, stddev=0.0,
            percentiles={"50": 0.0},
            forward_returns=returns,
        )
        group = _make_group(
            signal_price=100.0, direction="bullish",
            horizons={"2d": horizon},
        )
        level = group.deviation_plan.levels[0]

        contract = _make_option_contract(strike=98.0, ask=3.00)

        expected = strategy._compute_expected_profit(
            group, level, contract, contracts=2,
        )

        # exit=100.0, intrinsic=max(0,100-98)=2.0 for all
        # E[value] = 2.0 * 100 * 2 = 400
        # E[profit] = 400 - 600 = -200
        # Binary model with p_revert=1.0: 1.0 * (100-98)*100*2 - 600 = -200
        assert expected == pytest.approx(-200.0, abs=0.01)
