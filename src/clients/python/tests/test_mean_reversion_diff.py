"""
Behavioral diff test: MeanReversionStrategy (V1) vs MeanReversionStrategyV2 (V2).
Proves zero metric drift -- identical order sequences on identical inputs.

This test pattern is reusable for Phase 22 remaining strategy migrations.
The key idea: run both strategy versions through the same mocked candle sequence
with identical signal detection, then compare all place_order calls exactly.

Pattern for Phase 22:
1. Import both V1 and V2 strategy classes
2. Patch detect_atomic_signals_on_bar in BOTH source modules
3. Patch _bar_to_dict in BOTH strategy modules to control bar conversion
4. Create identical candle sequences and run both strategies
5. Assert place_order.call_args_list matches exactly
"""

from datetime import datetime
from unittest.mock import MagicMock, patch, call
from copy import deepcopy

import pytest

from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers (adapted from test_mean_reversion_strategy.py)
# ------------------------------------------------------------------ #

def _make_mock_playground(equity=100_000.0, free_margin=None):
    """Create a mock playground with minimal interface."""
    pg = MagicMock()
    pg.htf_seconds = 3600
    pg.ltf_seconds = 300
    pg.account = MagicMock()
    pg.account.equity = equity
    pg.account.free_margin = free_margin if free_margin is not None else equity
    pg.account.get_quantity = MagicMock(return_value=0)
    pg.place_order = MagicMock()
    pg.is_backtest_complete = False
    return pg


def _make_pdf(
    signal_key="bullish_supertrend|stochrsi_cross_above_20",
    mean=0.005,
    stddev=0.02,
    sample_size=100,
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


def _run_strategy(strategy_cls, tick_deltas, pdf, bar_to_dict_fn=None,
                  detect_patch_target=None):
    """Run a strategy through a sequence of tick deltas and return order calls.

    Args:
        strategy_cls: The strategy class to instantiate.
        tick_deltas: List of tick delta objects.
        pdf: PDFDocument for the strategy.
        bar_to_dict_fn: (unused, kept for API compat).
        detect_patch_target: (unused, kept for API compat).

    Returns:
        list of place_order call args.
    """
    pg = _make_mock_playground()
    strategy = strategy_cls(
        pg, "AAPL", pdf=pdf, total_shares_per_group=100,
    )
    for td in tick_deltas:
        strategy.on_tick([td])
    return pg.place_order.call_args_list


def _normalize_order_calls(call_list):
    """Normalize order calls for behavioral (trading) comparison.

    - Each unique group_id is mapped to a stable label (group_0, group_1, ...)
      so V1 and V2 calls compare despite different UUIDs.
    - The V2-only ``signal_id`` kwarg is dropped: it carries the SignalDecision
      RPC handle (telemetry), not trading behavior, and V1 never emits it. This
      keeps the diff scoped to the order semantics both versions must share.
    """
    normalized = []
    group_map = {}
    counter = 0
    for c in call_list:
        args, kwargs = c
        kwargs = {k: v for k, v in kwargs.items() if k != "signal_id"}
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

class TestV1V2BehavioralDiff:
    """Behavioral diff tests proving V1 and V2 produce identical output."""

    @patch("strategies.mean_reversion_v2._bar_to_dict")
    @patch("deprecated.mean_reversion._bar_to_dict")
    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    @patch("deprecated.mean_reversion.detect_atomic_signals_on_bar")
    def test_v1_v2_no_orders_on_no_signal(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both V1 and V2 produce zero orders when no signals are detected."""
        from deprecated.mean_reversion import MeanReversionStrategy
        from strategies.mean_reversion_v2 import MeanReversionStrategyV2

        # No signals detected
        mock_v1_detect.return_value = []
        mock_v2_detect.return_value = []

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))

        # Both _bar_to_dict mocks return bars in sequence
        mock_v1_bar.side_effect = [bar1, bar2]
        mock_v2_bar.side_effect = [bar1, bar2]

        pdf = _make_pdf()

        # Create two HTF candles
        c1 = _make_candle(3600)
        c2 = _make_candle(3600)
        td1 = _make_tick_delta([c1])
        td2 = _make_tick_delta([c2])

        orders_v1 = _run_strategy(
            MeanReversionStrategy, [td1, td2], pdf,
            bar_to_dict_fn=None,
            detect_patch_target="strategies.mean_reversion",
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
            MeanReversionStrategyV2, [td3, td4], pdf,
            bar_to_dict_fn=None,
            detect_patch_target="datasources.ma_crossover",
        )

        assert orders_v1 == []
        assert orders_v2 == []
        assert orders_v1 == orders_v2

    @patch("strategies.mean_reversion_v2._bar_to_dict")
    @patch("deprecated.mean_reversion._bar_to_dict")
    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    @patch("deprecated.mean_reversion.detect_atomic_signals_on_bar")
    def test_v1_v2_identical_orders_on_bullish_signal(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Both produce identical place_order sequences on bullish HTF signal."""
        from deprecated.mean_reversion import MeanReversionStrategy
        from strategies.mean_reversion_v2 import MeanReversionStrategyV2

        bar1 = _make_htf_bar(100.0)
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))
        ltf_bar = {
            "open": 99.5, "high": 100.0, "low": 98.5, "close": 99.0,
            "datetime": datetime(2025, 6, 15, 11, 5),
        }

        pdf = _make_pdf()

        # --- V1 run ---
        # First HTF bar: no signal; Second HTF bar: bullish signal
        mock_v1_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v1_bar.side_effect = [bar1, bar2, ltf_bar]

        htf_c1 = _make_candle(3600)
        htf_c2 = _make_candle(3600)
        ltf_c1 = _make_candle(300)
        td1 = _make_tick_delta([htf_c1])
        td2 = _make_tick_delta([htf_c2])
        td3 = _make_tick_delta([ltf_c1])

        orders_v1 = _run_strategy(
            MeanReversionStrategy, [td1, td2, td3], pdf,
            bar_to_dict_fn=None,
            detect_patch_target="strategies.mean_reversion",
        )

        # --- V2 run ---
        mock_v2_detect.side_effect = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        mock_v2_bar.side_effect = [bar1, bar2, ltf_bar]
        # V1 detect must also be reset (won't be called but just in case)
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        htf_c3 = _make_candle(3600)
        htf_c4 = _make_candle(3600)
        ltf_c2 = _make_candle(300)
        td4 = _make_tick_delta([htf_c3])
        td5 = _make_tick_delta([htf_c4])
        td6 = _make_tick_delta([ltf_c2])

        orders_v2 = _run_strategy(
            MeanReversionStrategyV2, [td4, td5, td6], pdf,
            bar_to_dict_fn=None,
            detect_patch_target="datasources.ma_crossover",
        )

        # Both must produce the exact same order calls (normalized for group_id)
        norm_v1 = _normalize_order_calls(orders_v1)
        norm_v2 = _normalize_order_calls(orders_v2)
        assert len(norm_v1) == len(norm_v2), (
            f"V1 produced {len(norm_v1)} orders, V2 produced {len(norm_v2)}"
        )
        for i, (v1_call, v2_call) in enumerate(zip(norm_v1, norm_v2)):
            assert v1_call == v2_call, (
                f"Order {i} differs:\n  V1={v1_call}\n  V2={v2_call}"
            )

    @patch("strategies.mean_reversion_v2._bar_to_dict")
    @patch("deprecated.mean_reversion._bar_to_dict")
    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    @patch("deprecated.mean_reversion.detect_atomic_signals_on_bar")
    def test_v1_v2_identical_on_multi_group_sequence(
        self, mock_v1_detect, mock_v2_detect,
        mock_v1_bar, mock_v2_bar,
    ):
        """Multiple HTF signals produce identical order sequences in V1 and V2."""
        from deprecated.mean_reversion import MeanReversionStrategy
        from strategies.mean_reversion_v2 import MeanReversionStrategyV2

        bar1 = _make_htf_bar(100.0, datetime=datetime(2025, 6, 15, 10, 0))
        bar2 = _make_htf_bar(101.0, datetime=datetime(2025, 6, 15, 11, 0))
        bar3 = _make_htf_bar(102.0, datetime=datetime(2025, 6, 15, 12, 0))
        ltf1 = {
            "open": 99.5, "high": 100.0, "low": 98.5, "close": 99.0,
            "datetime": datetime(2025, 6, 15, 11, 5),
        }
        ltf2 = {
            "open": 100.5, "high": 101.0, "low": 99.5, "close": 100.0,
            "datetime": datetime(2025, 6, 15, 12, 5),
        }

        pdf = _make_pdf()

        # Signal sequence: no signal, signal, signal
        signal_sequence = [
            [],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
            ["bullish_supertrend", "stochrsi_cross_above_20"],
        ]
        bar_sequence = [bar1, bar2, ltf1, bar3, ltf2]

        # --- V1 ---
        mock_v1_detect.side_effect = list(signal_sequence)
        mock_v1_bar.side_effect = list(bar_sequence)

        tds_v1 = [
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(300)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(300)]),
        ]

        orders_v1 = _run_strategy(
            MeanReversionStrategy, tds_v1, pdf,
            bar_to_dict_fn=None,
            detect_patch_target="strategies.mean_reversion",
        )

        # --- V2 ---
        mock_v2_detect.side_effect = list(signal_sequence)
        mock_v2_bar.side_effect = list(bar_sequence)
        mock_v1_detect.side_effect = None
        mock_v1_detect.return_value = []

        tds_v2 = [
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(300)]),
            _make_tick_delta([_make_candle(3600)]),
            _make_tick_delta([_make_candle(300)]),
        ]

        orders_v2 = _run_strategy(
            MeanReversionStrategyV2, tds_v2, pdf,
            bar_to_dict_fn=None,
            detect_patch_target="datasources.ma_crossover",
        )

        norm_v1 = _normalize_order_calls(orders_v1)
        norm_v2 = _normalize_order_calls(orders_v2)
        assert len(norm_v1) == len(norm_v2), (
            f"V1 produced {len(norm_v1)} orders, V2 produced {len(norm_v2)}"
        )
        for i, (v1_call, v2_call) in enumerate(zip(norm_v1, norm_v2)):
            assert v1_call == v2_call, (
                f"Order {i} differs:\n  V1={v1_call}\n  V2={v2_call}"
            )
