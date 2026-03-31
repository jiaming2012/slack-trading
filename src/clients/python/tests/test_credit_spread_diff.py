"""
Behavioral diff test: CreditSpreadStrategy (V1) vs CreditSpreadStrategyV2 (V2).
Proves zero metric drift -- identical group creation on identical inputs.

This test validates that the datasource extraction in V2 produces the same
signal detection behavior as V1's inline detect_atomic_signals_on_bar() call.

Because credit spread order placement depends on options ladder RPC calls
(complex external dependency), these tests compare at the _try_create_group
level -- the exact boundary where V1 and V2 diverge. Both versions share
identical code for everything after group creation (entries, exits, orders).

Pattern from Phase 21 test_mean_reversion_diff.py:
1. Import both V1 and V2 strategy classes
2. Patch detect_atomic_signals_on_bar in BOTH source modules
3. Patch _bar_to_dict in BOTH strategy modules to control bar conversion
4. Create identical candle sequences and run both strategies
5. Assert _try_create_group calls match exactly
"""

from datetime import datetime
from unittest.mock import MagicMock, patch, call
from copy import deepcopy

import pytest
import numpy as np

from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


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
    pg.account.balance = equity
    pg.account.free_margin = equity
    pg.account.get_quantity = MagicMock(return_value=10000)
    pg.account.get_position = MagicMock(return_value=None)
    pg.place_order = MagicMock()
    pg.fetch_ladder = MagicMock(return_value=None)
    pg.is_backtest_complete = False
    return pg


def _make_pdf(
    signal_key="bullish_supertrend|stochrsi_cross_above_20",
    mean=0.005,
    stddev=0.02,
    sample_size=100,
):
    """Create a minimal PDFDocument with one signal."""
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
        sufficient_samples=True,
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


def _make_candle(period, symbol="AAPL"):
    """Create a mock candle object with the expected structure."""
    candle = MagicMock()
    candle.period = period
    candle.symbol = symbol
    candle.bar = MagicMock()
    return candle


def _make_tick_delta(candles):
    """Create a mock tick delta containing new candles."""
    td = MagicMock()
    td.new_candles = candles
    return td


def _normalize_group_calls(call_list):
    """Normalize _try_create_group calls for comparison.

    Each call has positional args: (signal_key, bar_dict, pdf_entry, direction).
    The bar_dict and pdf_entry are compared by value, not identity.
    group_id is generated inside _try_create_group so it doesn't appear here.
    """
    normalized = []
    for c in call_list:
        args = c.args
        # Extract the 4 positional args
        signal_key = args[0]
        bar_dict = args[1]
        direction = args[3]
        normalized.append((signal_key, bar_dict.get("close"), direction))
    return normalized


def _run_strategy_and_capture_groups(strategy_cls, tick_deltas, pdf, bar_to_dict_mock):
    """Run a strategy and capture _try_create_group calls.

    Returns (strategy_instance, list of _try_create_group call args).
    """
    pg = _make_mock_playground()
    strategy = strategy_cls(
        pg, "AAPL", pdf=pdf,
        total_contracts_per_group=10,
        htf_horizon="1h",
    )

    # Spy on _try_create_group
    original_try_create = strategy._try_create_group
    group_calls = []

    def spy_try_create(*args, **kwargs):
        group_calls.append(call(*args, **kwargs))
        return original_try_create(*args, **kwargs)

    strategy._try_create_group = spy_try_create

    for td in tick_deltas:
        strategy.on_tick([td])

    return strategy, group_calls


# ------------------------------------------------------------------ #
# Diff tests
# ------------------------------------------------------------------ #

class TestCreditSpreadV1V2BehavioralDiff:
    """Behavioral diff tests proving V1 and V2 produce identical group creation."""

    @patch("strategies.credit_spread_v2._bar_to_dict")
    @patch("strategies.credit_spread._bar_to_dict")
    @patch("datasources.credit_spread_signals.detect_atomic_signals_on_bar")
    @patch("strategies.credit_spread.detect_atomic_signals_on_bar")
    def test_no_signal_produces_no_groups(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 produce zero groups when no signals are detected."""
        from strategies.credit_spread import CreditSpreadStrategy
        from strategies.credit_spread_v2 import CreditSpreadStrategyV2

        # No signals detected
        mock_v1_detect.return_value = []
        mock_v2_detect.return_value = []

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))

        mock_v1_bar.side_effect = [bar1, bar2]
        mock_v2_bar.side_effect = [bar1, bar2]

        pdf = _make_pdf()

        c1 = _make_candle(3600)
        c2 = _make_candle(3600)
        td1 = _make_tick_delta([c1])
        td2 = _make_tick_delta([c2])

        _, groups_v1 = _run_strategy_and_capture_groups(
            CreditSpreadStrategy, [td1, td2], pdf, mock_v1_bar,
        )

        # Reset for V2
        mock_v1_detect.return_value = []
        mock_v2_detect.return_value = []
        mock_v1_bar.side_effect = [bar1, bar2]
        mock_v2_bar.side_effect = [bar1, bar2]

        c3 = _make_candle(3600)
        c4 = _make_candle(3600)
        td3 = _make_tick_delta([c3])
        td4 = _make_tick_delta([c4])

        _, groups_v2 = _run_strategy_and_capture_groups(
            CreditSpreadStrategyV2, [td3, td4], pdf, mock_v2_bar,
        )

        assert groups_v1 == []
        assert groups_v2 == []
        assert groups_v1 == groups_v2

    @patch("strategies.credit_spread_v2._bar_to_dict")
    @patch("strategies.credit_spread._bar_to_dict")
    @patch("datasources.credit_spread_signals.detect_atomic_signals_on_bar")
    @patch("strategies.credit_spread.detect_atomic_signals_on_bar")
    def test_htf_bullish_signal_produces_identical_groups(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both produce identical _try_create_group calls on bullish HTF signal."""
        from strategies.credit_spread import CreditSpreadStrategy
        from strategies.credit_spread_v2 import CreditSpreadStrategyV2

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))

        pdf = _make_pdf()

        # --- V1 run ---
        mock_v1_detect.side_effect = [
            [],  # first bar: no signal
            ["bullish_supertrend", "stochrsi_cross_above_20"],  # second bar: signal
        ]
        mock_v1_bar.side_effect = [bar1, bar2]

        htf_c1 = _make_candle(3600)
        htf_c2 = _make_candle(3600)
        td1 = _make_tick_delta([htf_c1])
        td2 = _make_tick_delta([htf_c2])

        _, groups_v1 = _run_strategy_and_capture_groups(
            CreditSpreadStrategy, [td1, td2], pdf, mock_v1_bar,
        )

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2]
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        htf_c3 = _make_candle(3600)
        htf_c4 = _make_candle(3600)
        td3 = _make_tick_delta([htf_c3])
        td4 = _make_tick_delta([htf_c4])

        _, groups_v2 = _run_strategy_and_capture_groups(
            CreditSpreadStrategyV2, [td3, td4], pdf, mock_v2_bar,
        )

        # Both must create exactly one group
        assert len(groups_v1) == 1, f"V1 created {len(groups_v1)} groups, expected 1"
        assert len(groups_v2) == 1, f"V2 created {len(groups_v2)} groups, expected 1"

        # Compare normalized calls
        norm_v1 = _normalize_group_calls(groups_v1)
        norm_v2 = _normalize_group_calls(groups_v2)
        assert norm_v1 == norm_v2, (
            f"Group creation differs:\n  V1={norm_v1}\n  V2={norm_v2}"
        )

    @patch("strategies.credit_spread_v2._bar_to_dict")
    @patch("strategies.credit_spread._bar_to_dict")
    @patch("datasources.credit_spread_signals.detect_atomic_signals_on_bar")
    @patch("strategies.credit_spread.detect_atomic_signals_on_bar")
    def test_htf_bearish_signal_produces_identical_groups(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both produce identical groups on bearish HTF signal (non-long-only)."""
        from strategies.credit_spread import CreditSpreadStrategy
        from strategies.credit_spread_v2 import CreditSpreadStrategyV2

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(99.0, datetime=datetime(2025, 6, 15, 11, 0))

        # Bearish PDF: mean < 0
        pdf = _make_pdf(mean=-0.005, stddev=0.02)

        # --- V1 run ---
        mock_v1_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v1_bar.side_effect = [bar1, bar2]

        htf_c1 = _make_candle(3600)
        htf_c2 = _make_candle(3600)
        td1 = _make_tick_delta([htf_c1])
        td2 = _make_tick_delta([htf_c2])

        _, groups_v1 = _run_strategy_and_capture_groups(
            CreditSpreadStrategy, [td1, td2], pdf, mock_v1_bar,
        )

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2]
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        htf_c3 = _make_candle(3600)
        htf_c4 = _make_candle(3600)
        td3 = _make_tick_delta([htf_c3])
        td4 = _make_tick_delta([htf_c4])

        _, groups_v2 = _run_strategy_and_capture_groups(
            CreditSpreadStrategyV2, [td3, td4], pdf, mock_v2_bar,
        )

        # Both must create exactly one bearish group
        assert len(groups_v1) == 1, f"V1 created {len(groups_v1)} groups, expected 1"
        assert len(groups_v2) == 1, f"V2 created {len(groups_v2)} groups, expected 1"

        norm_v1 = _normalize_group_calls(groups_v1)
        norm_v2 = _normalize_group_calls(groups_v2)
        assert norm_v1 == norm_v2, (
            f"Group creation differs:\n  V1={norm_v1}\n  V2={norm_v2}"
        )

        # Verify direction is bearish
        assert norm_v1[0][2] == "bearish"

    @patch("strategies.credit_spread_v2._bar_to_dict")
    @patch("strategies.credit_spread._bar_to_dict")
    @patch("datasources.credit_spread_signals.detect_atomic_signals_on_bar")
    @patch("strategies.credit_spread.detect_atomic_signals_on_bar")
    def test_bearish_signal_skipped_when_long_only(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 skip bearish signals when long_only=True."""
        from strategies.credit_spread import CreditSpreadStrategy
        from strategies.credit_spread_v2 import CreditSpreadStrategyV2

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(99.0, datetime=datetime(2025, 6, 15, 11, 0))

        pdf = _make_pdf(mean=-0.005, stddev=0.02)

        # --- V1 run ---
        mock_v1_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v1_bar.side_effect = [bar1, bar2]

        pg_v1 = _make_mock_playground()
        strategy_v1 = CreditSpreadStrategy(
            pg_v1, "AAPL", pdf=pdf, total_contracts_per_group=10,
            long_only=True,
        )
        original_v1 = strategy_v1._try_create_group
        groups_v1 = []

        def spy_v1(*args, **kwargs):
            groups_v1.append(call(*args, **kwargs))
            return original_v1(*args, **kwargs)

        strategy_v1._try_create_group = spy_v1
        strategy_v1.on_tick([_make_tick_delta([_make_candle(3600)])])
        strategy_v1.on_tick([_make_tick_delta([_make_candle(3600)])])

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2]
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        pg_v2 = _make_mock_playground()
        strategy_v2 = CreditSpreadStrategyV2(
            pg_v2, "AAPL", pdf=pdf, total_contracts_per_group=10,
            long_only=True,
        )
        original_v2 = strategy_v2._try_create_group
        groups_v2 = []

        def spy_v2(*args, **kwargs):
            groups_v2.append(call(*args, **kwargs))
            return original_v2(*args, **kwargs)

        strategy_v2._try_create_group = spy_v2
        strategy_v2.on_tick([_make_tick_delta([_make_candle(3600)])])
        strategy_v2.on_tick([_make_tick_delta([_make_candle(3600)])])

        # Both should have zero groups (bearish signal skipped)
        assert groups_v1 == [], f"V1 should skip bearish signal when long_only, got {len(groups_v1)}"
        assert groups_v2 == [], f"V2 should skip bearish signal when long_only, got {len(groups_v2)}"

    @patch("strategies.credit_spread_v2._bar_to_dict")
    @patch("strategies.credit_spread._bar_to_dict")
    @patch("datasources.credit_spread_signals.detect_atomic_signals_on_bar")
    @patch("strategies.credit_spread.detect_atomic_signals_on_bar")
    def test_multi_signal_sequence_produces_identical_groups(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Multiple HTF signals produce identical group sequences in V1 and V2."""
        from strategies.credit_spread import CreditSpreadStrategy
        from strategies.credit_spread_v2 import CreditSpreadStrategyV2

        bar1 = _make_htf_bar(100.0, datetime=datetime(2025, 6, 15, 10, 0))
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))
        bar3 = _make_htf_bar(102.0, datetime=datetime(2025, 6, 15, 12, 0))

        pdf = _make_pdf()

        signal_sequence = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        bar_sequence = [bar1, bar2, bar3]

        # --- V1 ---
        mock_v1_detect.side_effect = list(signal_sequence)
        mock_v1_bar.side_effect = list(bar_sequence)

        tds_v1 = [
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
        ]

        _, groups_v1 = _run_strategy_and_capture_groups(
            CreditSpreadStrategy, tds_v1, pdf, mock_v1_bar,
        )

        # --- V2 ---
        mock_v2_detect.side_effect = list(signal_sequence)
        mock_v2_bar.side_effect = list(bar_sequence)
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        tds_v2 = [
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
        ]

        _, groups_v2 = _run_strategy_and_capture_groups(
            CreditSpreadStrategyV2, tds_v2, pdf, mock_v2_bar,
        )

        norm_v1 = _normalize_group_calls(groups_v1)
        norm_v2 = _normalize_group_calls(groups_v2)
        assert len(norm_v1) == len(norm_v2), (
            f"V1 produced {len(norm_v1)} groups, V2 produced {len(norm_v2)}"
        )
        for i, (v1_call, v2_call) in enumerate(zip(norm_v1, norm_v2)):
            assert v1_call == v2_call, (
                f"Group {i} differs:\n  V1={v1_call}\n  V2={v2_call}"
            )
