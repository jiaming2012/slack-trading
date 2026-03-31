"""
Behavioral diff test: OptionsStrategyBasic (V1) vs OptionsStrategyBasicV2 (V2).
Proves zero metric drift -- identical signal detection on identical inputs.

This test validates that the datasource extraction in V2 produces the same
check_for_new_signal behavior as V1's inline _get_feature_vector() call.

Because covered call order placement depends on options ladder RPC calls
(complex external dependency), these tests compare at the check_for_new_signal
boundary -- the exact point where V1 and V2 diverge. Both versions share
identical code for everything after signal detection (tick, on_tick, orders).

Pattern from Phase 21 test_mean_reversion_diff.py / test_credit_spread_diff.py:
1. Import both V1 and V2 strategy classes
2. Mock the playground and candle state
3. Call check_for_new_signal on both with identical inputs
4. Assert identical signal detection results
"""

from datetime import datetime
from unittest.mock import MagicMock, patch, PropertyMock
from copy import deepcopy

import pytest
import numpy as np
import pandas as pd
from google.protobuf.json_format import MessageToDict


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(symbol="AAPL", ltf_seconds=3600, htf_seconds=86400):
    """Create a mock playground with minimal interface for covered call."""
    pg = MagicMock()
    pg.id = "test-pg-id"
    pg.ltf_seconds = ltf_seconds
    pg.htf_seconds = htf_seconds
    pg.timestamp = datetime(2025, 6, 15, 10, 0)
    pg.is_backtest_complete = MagicMock(return_value=False)

    # Account mock
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

    # fetch_candles_v2 for BaseOpenStrategyV2 init (returns protobuf-like candles)
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
    """Build a candles_ltf DataFrame from a list of (close, superD, superT) tuples.

    Returns a DataFrame matching the structure used by BaseOpenStrategyV2.
    """
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
    # Extend to double size (matching BaseOpenStrategyV2 init pattern)
    total = len(rows) * 2
    df = pd.DataFrame(index=range(total), columns=rows[0].keys() if rows else [])
    for i, row in enumerate(rows):
        df.iloc[i] = row
    return df


def _setup_strategy(strategy_cls, candle_data, symbol="AAPL"):
    """Create a strategy instance with pre-loaded candle state.

    Bypasses the BaseOpenStrategyV2.__init__ (which calls fetch_candles_v2)
    by directly setting the candle state after construction.
    """
    pg = _make_mock_playground(symbol)

    # Provide empty candles for BaseOpenStrategyV2 init
    # We'll override the candle state after
    mock_candle = MagicMock()
    mock_candle_dict = {
        "close": 100.0, "open": 99.5, "high": 100.5, "low": 99.0,
        "datetime": "2025-06-15T10:00:00Z",
        "superD_50_3": 1.0, "superT_50_3": 98.0, "volume": 1000,
    }

    # Mock MessageToDict to return our dict
    with patch("deprecated.base_open_strategy_v2.MessageToDict", return_value=mock_candle_dict):
        pg.fetch_candles_v2.return_value = [mock_candle]
        strategy = strategy_cls(pg, symbol, logger=MagicMock(), max_open_count=3)

    # Override candle state with our test data
    strategy.candles_ltf = _build_candles_ltf_df(candle_data)
    strategy.candles_ltf_idx = len(candle_data)

    return strategy


# ------------------------------------------------------------------ #
# Diff tests
# ------------------------------------------------------------------ #

class TestCoveredCallV1V2BehavioralDiff:
    """Behavioral diff tests proving V1 and V2 produce identical signal detection."""

    def test_no_direction_change_produces_no_signal(self):
        """Both V1 and V2 return None when no supertrend direction change."""
        from strategies.covered_call import OptionsStrategyBasic
        from strategies.covered_call_v2 import OptionsStrategyBasicV2

        # All candles have same supertrend direction (1.0) -- no change
        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (102.0, 1.0, 99.0),
            (103.0, 1.0, 99.5),
        ]

        strategy_v1 = _setup_strategy(OptionsStrategyBasic, candle_data)
        strategy_v2 = _setup_strategy(OptionsStrategyBasicV2, candle_data)

        # New candle with same direction
        new_bar = _make_candle_bar(close=104.0, superD=1.0, superT=100.0,
                                   dt="2025-06-15T14:00:00Z")

        result_v1 = strategy_v1.check_for_new_signal(new_bar)
        result_v2 = strategy_v2.check_for_new_signal(new_bar)

        assert result_v1 is None, f"V1 should return None, got {result_v1}"
        assert result_v2 is None, f"V2 should return None, got {result_v2}"

    def test_direction_change_produces_identical_signal(self):
        """Both V1 and V2 produce identical OpenSignalV4 on supertrend direction change."""
        from strategies.covered_call import OptionsStrategyBasic, OpenSignalV4
        from strategies.covered_call_v2 import OptionsStrategyBasicV2

        # Candles with direction 1.0, then changes at index 2 to -1.0
        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),  # direction change here
            (98.0, -1.0, 101.5),
        ]

        strategy_v1 = _setup_strategy(OptionsStrategyBasic, candle_data)
        strategy_v2 = _setup_strategy(OptionsStrategyBasicV2, candle_data)

        # New candle continues the new direction
        new_bar = _make_candle_bar(close=97.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T15:00:00Z")

        result_v1 = strategy_v1.check_for_new_signal(new_bar)
        result_v2 = strategy_v2.check_for_new_signal(new_bar)

        assert result_v1 is not None, "V1 should detect signal"
        assert result_v2 is not None, "V2 should detect signal"
        assert isinstance(result_v1, OpenSignalV4)
        assert isinstance(result_v2, OpenSignalV4)

        # Compare signal fields
        assert result_v1.symbol == result_v2.symbol
        assert result_v1.name == result_v2.name
        assert result_v1.price == result_v2.price
        assert result_v1.ltf_supertrend_count == result_v2.ltf_supertrend_count
        assert result_v1.ltf_supertrend_value == result_v2.ltf_supertrend_value
        assert result_v1.ltf_supertrend_direction == result_v2.ltf_supertrend_direction

    def test_multiple_direction_changes_identical_signals(self):
        """Both produce identical signals across multiple supertrend direction changes."""
        from strategies.covered_call import OptionsStrategyBasic
        from strategies.covered_call_v2 import OptionsStrategyBasicV2

        # Multiple direction changes
        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),   # change 1
            (98.0, -1.0, 101.5),
            (100.0, 1.0, 99.0),    # change 2
            (101.0, 1.0, 99.5),
        ]

        strategy_v1 = _setup_strategy(OptionsStrategyBasic, candle_data)
        strategy_v2 = _setup_strategy(OptionsStrategyBasicV2, candle_data)

        # New candle with yet another direction change
        new_bar = _make_candle_bar(close=99.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T16:00:00Z")

        result_v1 = strategy_v1.check_for_new_signal(new_bar)
        result_v2 = strategy_v2.check_for_new_signal(new_bar)

        assert result_v1 is not None, "V1 should detect signal"
        assert result_v2 is not None, "V2 should detect signal"

        # Both must agree on the supertrend count (bars since last change)
        assert result_v1.ltf_supertrend_count == result_v2.ltf_supertrend_count, (
            f"Count mismatch: V1={result_v1.ltf_supertrend_count} "
            f"V2={result_v2.ltf_supertrend_count}"
        )
        assert result_v1.ltf_supertrend_direction == result_v2.ltf_supertrend_direction
        assert result_v1.price == result_v2.price

    def test_feature_vector_fn_callable_used_in_v2(self):
        """V2 uses the produce_open_signals datasource (not inline detection)."""
        from strategies.covered_call_v2 import OptionsStrategyBasicV2

        candle_data = [
            (100.0, 1.0, 98.0),
            (101.0, 1.0, 98.5),
            (99.0, -1.0, 101.0),
        ]

        strategy_v2 = _setup_strategy(OptionsStrategyBasicV2, candle_data)

        new_bar = _make_candle_bar(close=98.0, superD=-1.0, superT=102.0,
                                   dt="2025-06-15T13:00:00Z")

        # Patch produce_open_signals to verify it's called
        with patch("strategies.covered_call_v2.produce_open_signals") as mock_produce:
            mock_produce.return_value = [{
                "signal_name": "SHORT_CALL_SIGNAL",
                "symbol": "AAPL",
                "timestamp": "2025-06-15T13:00:00Z",
                "price": 98.0,
                "ltf_supertrend_count": 1,
                "ltf_supertrend_value": 102.0,
                "ltf_supertrend_direction": -1.0,
                "expected_volatility": 0.0,
            }]

            result = strategy_v2.check_for_new_signal(new_bar)

            mock_produce.assert_called_once()
            assert result is not None
            assert result.name == "SHORT_CALL_SIGNAL"
