"""
Behavioral diff test: WheelStrategy (V1) vs WheelStrategyV2 (V2).
Proves zero metric drift -- identical signal detection on identical inputs.

This test validates that the datasource extraction in V2 produces the same
check_for_put_signal behavior as V1's inline _get_feature_vector() call.

Tests both Wheel phases:
  - Phase 1 (SELL_PUTS): put signal detection via wheel_signals datasource
  - Phase 2 (SELL_CALLS): delegates to parent (OptionsStrategyBasicV2), which
    uses covered_call_signals datasource

Pattern from test_credit_spread_diff.py / test_covered_call_diff.py:
1. Import both V1 and V2 strategy classes
2. Mock the playground and candle state
3. Call signal detection on both with identical inputs
4. Assert identical results
"""

from datetime import datetime
from unittest.mock import MagicMock, patch

import pytest
import pandas as pd


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(symbol="AAPL", ltf_seconds=3600, htf_seconds=86400):
    """Create a mock playground with minimal interface for wheel strategy."""
    pg = MagicMock()
    pg.id = "test-pg-id"
    pg.ltf_seconds = ltf_seconds
    pg.htf_seconds = htf_seconds
    pg.timestamp = datetime(2025, 6, 15, 10, 0)
    pg.is_backtest_complete = MagicMock(return_value=False)

    # Account mock -- Phase 1 by default (< 100 shares)
    pg.account = MagicMock()
    pg.account.equity = 100_000.0
    pg.account.balance = 100_000.0
    pg.account.positions = {}
    pg.account.get_position = MagicMock(return_value=None)

    # Options and candle mocks
    pg.get_options_quantity = MagicMock(return_value=0)
    pg.get_option_positions = MagicMock(return_value={})
    pg.get_current_candle = MagicMock()
    pg.place_order = MagicMock()
    pg.fetch_ladder = MagicMock(return_value=None)

    # Stats mock
    pg.stats = MagicMock()
    pg.stats.calculate_local_model_volatility = MagicMock(return_value=0.2)

    # fetch_candles_v2 for BaseOpenStrategyV2 init
    pg.fetch_candles_v2 = MagicMock(return_value=[])

    return pg


def _make_candle_bar(close=100.0, superD=1.0, superT=98.0, dt="2025-06-15T10:00:00Z"):
    """Create a mock candle bar with supertrend fields."""
    bar = MagicMock()
    bar.close = close
    bar.open = close - 0.5
    bar.high = close + 0.5
    bar.low = close - 1.0
    bar.datetime = dt
    bar.superD_50_3 = superD
    bar.superT_50_3 = superT
    bar.volume = 1000
    return bar


def _build_candles_ltf_df(candle_data):
    """Build a candles_ltf DataFrame from a list of (close, superD, superT) tuples."""
    rows = []
    for i, (close, superD, superT) in enumerate(candle_data):
        rows.append({
            "close": close,
            "open": close - 0.5,
            "high": close + 0.5,
            "low": close - 1.0,
            "datetime": f"2025-06-15T{10 + i}:00:00Z",
            "superD_50_3": superD,
            "superT_50_3": superT,
            "volume": 1000,
        })
    total = len(rows) * 2
    df = pd.DataFrame(index=range(total), columns=rows[0].keys() if rows else [])
    for i, row in enumerate(rows):
        df.iloc[i] = row
    return df


def _setup_strategy(strategy_cls, candle_data, symbol="AAPL"):
    """Create a strategy instance with pre-loaded candle state."""
    pg = _make_mock_playground(symbol)

    mock_candle = MagicMock()
    mock_candle_dict = {
        "close": 100.0, "open": 99.5, "high": 100.5, "low": 99.0,
        "datetime": "2025-06-15T10:00:00Z",
        "superD_50_3": 1.0, "superT_50_3": 98.0, "volume": 1000,
    }

    with patch("deprecated.base_open_strategy_v2.MessageToDict", return_value=mock_candle_dict):
        pg.fetch_candles_v2.return_value = [mock_candle]
        strategy = strategy_cls(pg, symbol, logger=MagicMock(), max_open_count=3)

    strategy.candles_ltf = _build_candles_ltf_df(candle_data)
    strategy.candles_ltf_idx = len(candle_data)

    return strategy


# ------------------------------------------------------------------ #
# Phase 1 diff tests (put signal detection)
# ------------------------------------------------------------------ #

class TestWheelV1V2PutSignalDiff:
    """Behavioral diff tests proving V1 and V2 produce identical put signal detection."""

    def test_no_direction_change_produces_no_put_signal(self):
        """Both V1 and V2 return None when no supertrend direction change."""
        from deprecated.wheel import WheelStrategy
        from strategies.wheel_v2 import WheelStrategyV2

        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (102.0, 1.0, 99.0),
            (103.0, 1.0, 99.5),
        ]

        strategy_v1 = _setup_strategy(WheelStrategy, candle_data)
        strategy_v2 = _setup_strategy(WheelStrategyV2, candle_data)

        new_bar = _make_candle_bar(close=104.0, superD=1.0, superT=100.0,
                                   dt="2025-06-15T14:00:00Z")

        result_v1 = strategy_v1.check_for_put_signal(new_bar)
        result_v2 = strategy_v2.check_for_put_signal(new_bar)

        assert result_v1 is None, f"V1 should return None, got {result_v1}"
        assert result_v2 is None, f"V2 should return None, got {result_v2}"

    def test_direction_change_produces_identical_put_signal(self):
        """Both V1 and V2 produce identical PutSellSignal on supertrend direction change."""
        from deprecated.wheel import WheelStrategy, PutSellSignal
        from strategies.wheel_v2 import WheelStrategyV2

        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),  # direction change
            (98.0, -1.0, 101.5),
        ]

        strategy_v1 = _setup_strategy(WheelStrategy, candle_data)
        strategy_v2 = _setup_strategy(WheelStrategyV2, candle_data)

        new_bar = _make_candle_bar(close=97.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T15:00:00Z")

        result_v1 = strategy_v1.check_for_put_signal(new_bar)
        result_v2 = strategy_v2.check_for_put_signal(new_bar)

        assert result_v1 is not None, "V1 should detect put signal"
        assert result_v2 is not None, "V2 should detect put signal"
        assert isinstance(result_v1, PutSellSignal)
        assert isinstance(result_v2, PutSellSignal)

        # Compare signal fields
        assert result_v1.symbol == result_v2.symbol
        assert result_v1.name == result_v2.name
        assert result_v1.price == result_v2.price
        assert result_v1.ltf_supertrend_count == result_v2.ltf_supertrend_count
        assert result_v1.ltf_supertrend_value == result_v2.ltf_supertrend_value
        assert result_v1.ltf_supertrend_direction == result_v2.ltf_supertrend_direction


# ------------------------------------------------------------------ #
# Phase 2 diff tests (covered call signal delegation)
# ------------------------------------------------------------------ #

class TestWheelV1V2CoveredCallDiff:
    """Verify V2 delegates Phase 2 to parent (OptionsStrategyBasicV2) correctly."""

    def test_phase2_delegates_to_parent_check_for_new_signal(self):
        """Phase 2 (SELL_CALLS) uses parent's check_for_new_signal in both versions."""
        from deprecated.wheel import WheelStrategy
        from strategies.wheel_v2 import WheelStrategyV2
        from deprecated.covered_call import OpenSignalV4

        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),  # direction change
            (98.0, -1.0, 101.5),
        ]

        strategy_v1 = _setup_strategy(WheelStrategy, candle_data)
        strategy_v2 = _setup_strategy(WheelStrategyV2, candle_data)

        new_bar = _make_candle_bar(close=97.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T15:00:00Z")

        # Both inherit check_for_new_signal from their respective parents
        result_v1 = strategy_v1.check_for_new_signal(new_bar)
        result_v2 = strategy_v2.check_for_new_signal(new_bar)

        assert result_v1 is not None, "V1 should detect call signal"
        assert result_v2 is not None, "V2 should detect call signal"
        assert isinstance(result_v1, OpenSignalV4)
        assert isinstance(result_v2, OpenSignalV4)

        # Compare signal fields
        assert result_v1.symbol == result_v2.symbol
        assert result_v1.price == result_v2.price
        assert result_v1.ltf_supertrend_count == result_v2.ltf_supertrend_count
        assert result_v1.ltf_supertrend_direction == result_v2.ltf_supertrend_direction

    def test_v2_inheritance_chain(self):
        """WheelStrategyV2 extends OptionsStrategyBasicV2 (not V1 parent)."""
        from strategies.wheel_v2 import WheelStrategyV2
        from strategies.covered_call_v2 import OptionsStrategyBasicV2
        from strategies.base_strategy import BaseStrategy

        assert issubclass(WheelStrategyV2, OptionsStrategyBasicV2), (
            "WheelStrategyV2 must extend OptionsStrategyBasicV2"
        )
        assert issubclass(WheelStrategyV2, BaseStrategy), (
            "WheelStrategyV2 must ultimately extend BaseStrategy"
        )

    def test_feature_vector_fn_callable_used_in_v2_put_signal(self):
        """V2 uses the produce_open_signals datasource for put signals."""
        from strategies.wheel_v2 import WheelStrategyV2

        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),
        ]

        strategy_v2 = _setup_strategy(WheelStrategyV2, candle_data)

        new_bar = _make_candle_bar(close=98.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T13:00:00Z")

        with patch("strategies.wheel_v2.produce_open_signals") as mock_produce:
            mock_produce.return_value = [{
                "signal_name": "SHORT_PUT_SIGNAL",
                "symbol": "AAPL",
                "timestamp": "2025-06-15T13:00:00Z",
                "price": 98.0,
                "ltf_supertrend_count": 1,
                "ltf_supertrend_value": 102.0,
                "ltf_supertrend_direction": -1.0,
                "expected_volatility": 0.0,
            }]

            result = strategy_v2.check_for_put_signal(new_bar)

            mock_produce.assert_called_once()
            assert result is not None
            assert result.name == "SHORT_PUT_SIGNAL"
