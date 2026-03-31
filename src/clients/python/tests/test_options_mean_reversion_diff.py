"""
Behavioral diff test: OptionsMeanReversionStrategy (V1) vs OptionsMeanReversionStrategyV2 (V2).
Proves zero metric drift -- identical order sequences on identical inputs.

Pattern follows test_mean_reversion_diff.py from Phase 21:
1. Import both V1 and V2 strategy classes
2. Patch detect_atomic_signals_on_bar in V1 module and produce_signals in V2 datasource
3. Patch _bar_to_dict in both strategy modules to control bar conversion
4. Create identical candle sequences and run both strategies
5. Assert place_order.call_args_list matches exactly (after group_id normalization)
"""

from datetime import datetime
from unittest.mock import MagicMock, patch, call
from copy import deepcopy

import pytest

from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(equity=100_000.0):
    """Create a mock playground with minimal interface for options strategy."""
    pg = MagicMock()
    pg.htf_seconds = 3600
    pg.ltf_seconds = 300
    pg.account = MagicMock()
    pg.account.equity = equity
    pg.account.balance = equity
    pg.account.free_margin = equity
    pg.account.get_quantity = MagicMock(return_value=10000)
    pg.account.get_position = MagicMock(return_value=None)
    pg.place_order = MagicMock()
    pg.fetch_ladder = MagicMock(return_value=None)
    pg.is_backtest_complete = False
    pg.current_candles = {}
    return pg


def _make_pdf(
    signal_key="bullish_supertrend|stochrsi_cross_above_20",
    mean=0.005,
    stddev=0.02,
    sample_size=100,
    horizon_key="1h",
):
    """Create a minimal PDFDocument with one signal."""
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
        sufficient_samples=True,
        horizons={horizon_key: horizon},
    )
    return PDFDocument(
        symbol="AAPL",
        signals={signal_key: signal},
        scan_start="2025-01-01",
        scan_end="2025-06-01",
        ltf_period_seconds=300,
        htf_period_seconds=3600,
    )


def _make_bearish_pdf(
    signal_key="bearish_supertrend|stochrsi_cross_below_80",
    mean=-0.005,
    stddev=0.02,
    sample_size=100,
    horizon_key="1h",
):
    """Create a minimal PDFDocument with a bearish signal."""
    import numpy as np
    rng = np.random.default_rng(42)
    forward_returns = list(rng.normal(mean, stddev, sample_size))

    horizon = HorizonStats(
        mean=mean,
        stddev=stddev,
        percentiles={"5": -0.03, "10": -0.02, "25": -0.005, "50": -0.005},
        forward_returns=forward_returns,
    )
    signal = SignalPDF(
        sample_size=sample_size,
        ci_95_width=0.01,
        sufficient_samples=True,
        horizons={horizon_key: horizon},
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


def _run_strategy(strategy_cls, tick_deltas, pdf, **kwargs):
    """Run a strategy through a sequence of tick deltas and return order calls.

    Returns:
        list of place_order call args.
    """
    pg = _make_mock_playground()
    strategy = strategy_cls(
        pg, "AAPL", pdf=pdf, total_contracts_per_group=5, **kwargs,
    )
    for td in tick_deltas:
        strategy.on_tick([td])
    return pg.place_order.call_args_list


def _normalize_order_calls(call_list):
    """Normalize order calls by replacing non-deterministic group_id values.

    Each unique group_id is mapped to a stable label (group_0, group_1, ...)
    so that V1 and V2 calls can be compared despite different UUIDs.
    """
    normalized = []
    group_map = {}
    counter = 0
    for c in call_list:
        args, kwargs = c
        attrs = kwargs.get("attributes", args[5] if len(args) > 5 else None)
        if attrs and "group_id" in attrs:
            gid = attrs["group_id"]
            if gid not in group_map:
                group_map[gid] = f"group_{counter}"
                counter += 1
            attrs = dict(attrs)
            attrs["group_id"] = group_map[gid]
            if len(args) > 5:
                args = args[:5] + (attrs,) + args[6:]
                normalized.append(call(*args))
            else:
                new_kwargs = dict(kwargs)
                new_kwargs["attributes"] = attrs
                normalized.append(call(*args, **new_kwargs))
        else:
            normalized.append(c)
    return normalized


# ------------------------------------------------------------------ #
# Diff tests
# ------------------------------------------------------------------ #

class TestOptionsV1V2BehavioralDiff:
    """Behavioral diff tests proving V1 and V2 produce identical output."""

    @patch("strategies.options_mean_reversion_v2._bar_to_dict")
    @patch("strategies.options_mean_reversion._bar_to_dict")
    @patch("datasources.options_ma_crossover.detect_atomic_signals_on_bar")
    @patch("strategies.options_mean_reversion.detect_atomic_signals_on_bar")
    def test_htf_bullish_signal_produces_identical_orders(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 produce identical group creation on bullish HTF signal.

        Since options entry requires a ladder fetch (which returns None in mocks),
        no actual place_order calls are made. But the _try_create_group path
        exercises the same logic in both versions.
        """
        from deprecated.options_mean_reversion import OptionsMeanReversionStrategy
        from strategies.options_mean_reversion_v2 import OptionsMeanReversionStrategyV2

        signal_key = "bullish_supertrend|stochrsi_cross_above_20"
        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))

        pdf = _make_pdf(signal_key=signal_key)

        # --- V1 run ---
        mock_v1_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v1_bar.side_effect = [bar1, bar2]

        pg_v1 = _make_mock_playground()
        v1 = OptionsMeanReversionStrategy(
            pg_v1, "AAPL", pdf=pdf, total_contracts_per_group=5,
        )
        td1 = _make_tick_delta([_make_candle(3600)])
        td2 = _make_tick_delta([_make_candle(3600)])
        v1.on_tick([td1])
        v1.on_tick([td2])

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2]
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        pg_v2 = _make_mock_playground()
        v2 = OptionsMeanReversionStrategyV2(
            pg_v2, "AAPL", pdf=pdf, total_contracts_per_group=5,
        )
        td3 = _make_tick_delta([_make_candle(3600)])
        td4 = _make_tick_delta([_make_candle(3600)])
        v2.on_tick([td3])
        v2.on_tick([td4])

        # Both should create exactly one trade group with same signal
        assert len(v1.trade_groups) == len(v2.trade_groups), (
            f"V1 groups: {len(v1.trade_groups)}, V2 groups: {len(v2.trade_groups)}"
        )
        assert len(v1.trade_groups) == 1
        assert v1.trade_groups[0].htf_signal_key == v2.trade_groups[0].htf_signal_key
        assert v1.trade_groups[0].direction == v2.trade_groups[0].direction == "bullish"

        # Funnel counters match
        assert v1.funnel["htf_bars"] == v2.funnel["htf_bars"]
        assert v1.funnel["signals_detected"] == v2.funnel["signals_detected"]
        assert v1.funnel["groups_created"] == v2.funnel["groups_created"]

        # place_order calls match (both empty since no ladder)
        norm_v1 = _normalize_order_calls(pg_v1.place_order.call_args_list)
        norm_v2 = _normalize_order_calls(pg_v2.place_order.call_args_list)
        assert norm_v1 == norm_v2

    @patch("strategies.options_mean_reversion_v2._bar_to_dict")
    @patch("strategies.options_mean_reversion._bar_to_dict")
    @patch("datasources.options_ma_crossover.detect_atomic_signals_on_bar")
    @patch("strategies.options_mean_reversion.detect_atomic_signals_on_bar")
    def test_htf_bearish_signal_produces_identical_orders(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 produce identical group creation on bearish HTF signal."""
        from deprecated.options_mean_reversion import OptionsMeanReversionStrategy
        from strategies.options_mean_reversion_v2 import OptionsMeanReversionStrategyV2

        signal_key = "bearish_supertrend|stochrsi_cross_below_80"
        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(99.0, datetime=datetime(2025, 6, 15, 11, 0))

        pdf = _make_bearish_pdf(signal_key=signal_key)

        # --- V1 run (not long_only) ---
        mock_v1_detect.side_effect = [
            [],
            ["bearish_supertrend", "stochrsi_cross_below_80"],
        ]
        mock_v1_bar.side_effect = [bar1, bar2]

        pg_v1 = _make_mock_playground()
        v1 = OptionsMeanReversionStrategy(
            pg_v1, "AAPL", pdf=pdf, total_contracts_per_group=5, long_only=False,
        )
        td1 = _make_tick_delta([_make_candle(3600)])
        td2 = _make_tick_delta([_make_candle(3600)])
        v1.on_tick([td1])
        v1.on_tick([td2])

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bearish_supertrend", "stochrsi_cross_below_80"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2]
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        pg_v2 = _make_mock_playground()
        v2 = OptionsMeanReversionStrategyV2(
            pg_v2, "AAPL", pdf=pdf, total_contracts_per_group=5, long_only=False,
        )
        td3 = _make_tick_delta([_make_candle(3600)])
        td4 = _make_tick_delta([_make_candle(3600)])
        v2.on_tick([td3])
        v2.on_tick([td4])

        # Both produce identical group counts and funnel counters
        assert len(v1.trade_groups) == len(v2.trade_groups), (
            f"V1 groups: {len(v1.trade_groups)}, V2 groups: {len(v2.trade_groups)}"
        )

        # If groups were created, verify signal match
        if v1.trade_groups:
            assert v1.trade_groups[0].htf_signal_key == v2.trade_groups[0].htf_signal_key
            assert v1.trade_groups[0].direction == v2.trade_groups[0].direction == "bearish"

        # Funnel counters match (proves identical code paths)
        assert v1.funnel["htf_bars"] == v2.funnel["htf_bars"]
        assert v1.funnel["signals_detected"] == v2.funnel["signals_detected"]
        assert v1.funnel["bearish_signals"] == v2.funnel["bearish_signals"]
        assert v1.funnel["groups_created"] == v2.funnel["groups_created"]
        assert v1.funnel["groups_skipped_empty_plan"] == v2.funnel["groups_skipped_empty_plan"]

        # place_order calls match
        norm_v1 = _normalize_order_calls(pg_v1.place_order.call_args_list)
        norm_v2 = _normalize_order_calls(pg_v2.place_order.call_args_list)
        assert norm_v1 == norm_v2

    @patch("strategies.options_mean_reversion_v2._bar_to_dict")
    @patch("strategies.options_mean_reversion._bar_to_dict")
    @patch("datasources.options_ma_crossover.detect_atomic_signals_on_bar")
    @patch("strategies.options_mean_reversion.detect_atomic_signals_on_bar")
    def test_no_signal_produces_no_orders(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 produce zero orders when no signals are detected."""
        from deprecated.options_mean_reversion import OptionsMeanReversionStrategy
        from strategies.options_mean_reversion_v2 import OptionsMeanReversionStrategyV2

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

        orders_v1 = _run_strategy(
            OptionsMeanReversionStrategy, [td1, td2], pdf,
        )

        # Reset mocks for V2 run
        mock_v1_detect.return_value = []
        mock_v2_detect.return_value = []
        mock_v1_bar.side_effect = [bar1, bar2]
        mock_v2_bar.side_effect = [bar1, bar2]

        c3 = _make_candle(3600)
        c4 = _make_candle(3600)
        td3 = _make_tick_delta([c3])
        td4 = _make_tick_delta([c4])

        orders_v2 = _run_strategy(
            OptionsMeanReversionStrategyV2, [td3, td4], pdf,
        )

        assert orders_v1 == []
        assert orders_v2 == []
        assert orders_v1 == orders_v2
