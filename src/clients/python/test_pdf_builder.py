"""
Unit tests for Phase A: candlestick patterns, PDF types, and PDF builder.

All tests use contrived OHLC data — no server or Polygon API required.

Run::

    cd src/clients/python
    /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_pdf_builder.py -v
"""

import json
import math
import os
import tempfile
from datetime import datetime, timedelta

import pytest

from candlestick_patterns import is_pin_bar, is_engulfing, is_hammer, is_doji
from pdf_types import (
    CompoundSignal,
    HorizonStats,
    PDFDocument,
    SignalPDF,
    ci_95_width,
    compute_percentiles,
)
from pdf_builder import PDFBuilder, detect_atomic_signals_on_bar


# ================================================================== #
# Helpers — contrived bar builders
# ================================================================== #

def _bar(
    open=100, high=105, low=95, close=100,
    dt=None,
    superD=0,
    stochrsi_k=50,
    stochrsi_cross_above_20=False,
    stochrsi_cross_below_80=False,
    cdl_hammer=0,
    cdl_doji=0,
    sma_50=0, sma_100=0, sma_200=0,
):
    """Create a contrived bar dict with indicator fields."""
    return {
        "open": open, "high": high, "low": low, "close": close,
        "datetime": dt or datetime(2025, 1, 2, 10, 0),
        "superD_50_3": superD,
        "stochrsi_k_14_14_3_3": stochrsi_k,
        "stochrsi_cross_above_20": stochrsi_cross_above_20,
        "stochrsi_cross_below_80": stochrsi_cross_below_80,
        "cdl_hammer": cdl_hammer,
        "cdl_doji_10_0_1": cdl_doji,
        "sma_50": sma_50, "sma_100": sma_100, "sma_200": sma_200,
    }


# ================================================================== #
# 1. Candlestick pattern tests
# ================================================================== #

class TestPinBar:
    def test_bearish_pin_bar(self):
        # Long upper wick, small body near bottom
        detected, direction = is_pin_bar(open=100, high=120, low=99, close=101)
        assert detected is True
        assert direction == "bearish"

    def test_bullish_pin_bar(self):
        # Long lower wick, small body near top
        detected, direction = is_pin_bar(open=100, high=101, low=80, close=99)
        assert detected is True
        assert direction == "bullish"

    def test_not_a_pin_bar(self):
        # Normal candle, roughly equal wicks
        detected, _ = is_pin_bar(open=100, high=105, low=95, close=102)
        assert detected is False

    def test_doji_like_pin_bar(self):
        # Zero body but long upper wick should still detect
        detected, direction = is_pin_bar(open=100, high=120, low=99, close=100)
        assert detected is True
        assert direction == "bearish"


class TestEngulfing:
    def test_bullish_engulfing(self):
        # Prev red, curr green engulfs
        detected, direction = is_engulfing(
            prev_open=105, prev_close=100,  # red
            curr_open=99, curr_close=106,    # green, engulfs
        )
        assert detected is True
        assert direction == "bullish"

    def test_bearish_engulfing(self):
        # Prev green, curr red engulfs
        detected, direction = is_engulfing(
            prev_open=100, prev_close=105,  # green
            curr_open=106, curr_close=99,    # red, engulfs
        )
        assert detected is True
        assert direction == "bearish"

    def test_not_engulfing(self):
        # Curr body does not fully engulf prev
        detected, _ = is_engulfing(
            prev_open=100, prev_close=95,
            curr_open=96, curr_close=99,  # does not reach prev_open=100
        )
        assert detected is False

    def test_same_color_not_engulfing(self):
        # Both green — not an engulfing pattern
        detected, _ = is_engulfing(
            prev_open=100, prev_close=105,
            curr_open=104, curr_close=110,
        )
        assert detected is False


class TestHammer:
    def test_hammer(self):
        # body=0.5, lower_wick=10, upper_wick=0 → classic hammer
        assert is_hammer(open=100, high=100, low=90, close=100.5) is True

    def test_not_hammer_upper_wick_too_long(self):
        assert is_hammer(open=100, high=110, low=90, close=100.5) is False


class TestDoji:
    def test_doji(self):
        assert is_doji(open=100, close=100.005, high=105, low=95) is True

    def test_not_doji(self):
        assert is_doji(open=100, close=103, high=105, low=95) is False


# ================================================================== #
# 2. Atomic signal detection tests
# ================================================================== #

class TestAtomicSignalDetection:
    def test_stochrsi_above_80(self):
        bar = _bar(stochrsi_k=85)
        sigs = detect_atomic_signals_on_bar(bar)
        assert "stochrsi_above_80" in sigs

    def test_stochrsi_below_20(self):
        bar = _bar(stochrsi_k=15)
        sigs = detect_atomic_signals_on_bar(bar)
        assert "stochrsi_below_20" in sigs

    def test_supertrend_flip_up(self):
        prev = _bar(superD=-1)
        curr = _bar(superD=1)
        sigs = detect_atomic_signals_on_bar(curr, prev_bar=prev, timeframe="ltf")
        assert "supertrend_flip_up" in sigs

    def test_supertrend_flip_down(self):
        prev = _bar(superD=1)
        curr = _bar(superD=-1)
        sigs = detect_atomic_signals_on_bar(curr, prev_bar=prev, timeframe="ltf")
        assert "supertrend_flip_down" in sigs

    def test_daily_supertrend_up(self):
        bar = _bar(superD=1)
        sigs = detect_atomic_signals_on_bar(bar, timeframe="daily")
        assert "daily_supertrend_up" in sigs

    def test_daily_supertrend_down(self):
        bar = _bar(superD=-1)
        sigs = detect_atomic_signals_on_bar(bar, timeframe="daily")
        assert "daily_supertrend_down" in sigs

    def test_sma_cross_above(self):
        prev = _bar(close=99, sma_50=100)
        curr = _bar(close=101, sma_50=100)
        sigs = detect_atomic_signals_on_bar(curr, prev_bar=prev)
        assert "sma_50_cross_above" in sigs

    def test_sma_cross_below(self):
        prev = _bar(close=101, sma_50=100)
        curr = _bar(close=99, sma_50=100)
        sigs = detect_atomic_signals_on_bar(curr, prev_bar=prev)
        assert "sma_50_cross_below" in sigs

    def test_stochrsi_cross_above_20_flag(self):
        bar = _bar(stochrsi_cross_above_20=True)
        sigs = detect_atomic_signals_on_bar(bar)
        assert "stochrsi_cross_above_20" in sigs

    def test_pin_bar_detected_via_atomic(self):
        bar = _bar(open=100, high=120, low=99, close=101)
        sigs = detect_atomic_signals_on_bar(bar)
        assert "bearish_pin_bar" in sigs


# ================================================================== #
# 3. Compound signal tests
# ================================================================== #

class TestCompoundSignals:
    def _make_builder(self, ltf_bars, daily_bars=None):
        return PDFBuilder(
            symbol="TEST",
            ltf_bars=ltf_bars,
            daily_bars=daily_bars or [],
        )

    def test_single_atomic_becomes_compound(self):
        bars = [
            _bar(open=100, high=120, low=99, close=101,
                 dt=datetime(2025, 1, 2, 10, 0)),
        ]
        builder = self._make_builder(bars)
        ltf_sigs = builder._detect_ltf_signals()
        compounds = builder.build_compound_signals(ltf_sigs, {})
        assert len(compounds) >= 1
        assert "bearish_pin_bar" in compounds[0].components

    def test_cross_timeframe_compound(self):
        ltf_bars = [
            _bar(open=100, high=100, low=100, close=100,
                 dt=datetime(2025, 1, 2, 9, 45)),  # dummy prev bar
            _bar(open=100, high=120, low=99, close=101,
                 dt=datetime(2025, 1, 2, 10, 0)),   # bearish pin bar
        ]
        daily_bars = [
            _bar(superD=1, dt=datetime(2025, 1, 2, 16, 0)),  # daily supertrend up
        ]
        builder = self._make_builder(ltf_bars, daily_bars)
        daily_ctx = builder._build_daily_context()
        ltf_sigs = builder._detect_ltf_signals()
        compounds = builder.build_compound_signals(ltf_sigs, daily_ctx)

        # Should have at least one compound with both bearish_pin_bar and daily_supertrend_up
        keys = [c.key for c in compounds]
        found = any("bearish_pin_bar" in k and "daily_supertrend_up" in k for k in keys)
        assert found, f"Expected compound with pin_bar+daily_supertrend_up, got keys: {keys}"

    def test_compound_key_is_sorted(self):
        # Components must be passed pre-sorted (build_compound_signals does this)
        cs = CompoundSignal(
            components=("bearish_pin_bar", "daily_supertrend_up", "stochrsi_above_80"),
            timestamp=datetime(2025, 1, 2),
            bar_close=100,
        )
        parts = cs.key.split("|")
        assert parts == sorted(parts)
        assert cs.key == "bearish_pin_bar|daily_supertrend_up|stochrsi_above_80"


# ================================================================== #
# 4. Forward returns tests
# ================================================================== #

class TestForwardReturns:
    def test_known_forward_returns(self):
        base_dt = datetime(2025, 1, 2, 10, 0)
        bars = [
            _bar(close=100, dt=base_dt),                              # idx 0
            _bar(close=101, dt=base_dt + timedelta(minutes=15)),      # idx 1
            _bar(close=102, dt=base_dt + timedelta(minutes=30)),      # idx 2
            _bar(close=103, dt=base_dt + timedelta(minutes=45)),      # idx 3
            _bar(close=104, dt=base_dt + timedelta(minutes=60)),      # idx 4
        ]
        builder = PDFBuilder(
            symbol="TEST", ltf_bars=bars, daily_bars=[],
            horizons={"1h": 4, "30m": 2},
        )
        fwd = builder.compute_forward_returns(bar_index=0, bar_close=100)
        assert fwd["1h"] == pytest.approx(0.04)    # (104-100)/100
        assert fwd["30m"] == pytest.approx(0.02)   # (102-100)/100

    def test_insufficient_bars_for_horizon(self):
        bars = [_bar(close=100), _bar(close=101)]
        builder = PDFBuilder(
            symbol="TEST", ltf_bars=bars, daily_bars=[],
            horizons={"1h": 4},
        )
        fwd = builder.compute_forward_returns(bar_index=0, bar_close=100)
        assert fwd["1h"] is None


# ================================================================== #
# 5. CI width and percentile tests
# ================================================================== #

class TestCIWidth:
    def test_known_ci_width(self):
        returns = [0.01, 0.02, 0.03, 0.04, 0.05]
        n = 5
        mean = 0.03
        std = math.sqrt(sum((r - mean) ** 2 for r in returns) / (n - 1))
        expected = 2 * 1.96 * std / math.sqrt(n)
        assert ci_95_width(returns) == pytest.approx(expected)

    def test_single_return_is_inf(self):
        assert ci_95_width([0.01]) == float("inf")

    def test_empty_returns_is_inf(self):
        assert ci_95_width([]) == float("inf")

    def test_sufficient_samples(self):
        # Many identical returns → std=0 → CI=0 → sufficient
        returns = [0.01] * 100
        assert ci_95_width(returns) == pytest.approx(0.0)


class TestPercentiles:
    def test_known_percentiles(self):
        returns = list(range(1, 101))  # 1, 2, ..., 100
        pcts = compute_percentiles(returns, quantiles=(5, 10, 25, 50))
        assert pcts["50"] == pytest.approx(50.5)
        assert pcts["5"] == pytest.approx(5.95, abs=0.1)
        assert pcts["25"] == pytest.approx(25.75, abs=0.1)

    def test_empty_returns(self):
        pcts = compute_percentiles([], quantiles=(5, 50))
        assert pcts["5"] == 0.0
        assert pcts["50"] == 0.0


# ================================================================== #
# 6. JSON roundtrip
# ================================================================== #

class TestJSONRoundtrip:
    def test_save_and_load(self):
        pdf = PDFDocument(
            symbol="AAPL",
            scan_start="2024-01-01",
            scan_end="2024-12-31",
            signals={
                "bearish_pin_bar|daily_supertrend_up": SignalPDF(
                    sample_size=50,
                    ci_95_width=0.003,
                    sufficient_samples=True,
                    horizons={
                        "1h": HorizonStats(
                            mean=-0.001,
                            stddev=0.004,
                            percentiles={"5": -0.009, "10": -0.007, "25": -0.003, "50": -0.001},
                            forward_returns=[-0.009, -0.007, -0.003, -0.001, 0.001],
                        ),
                    },
                ),
            },
        )

        with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
            path = f.name

        try:
            pdf.save(path)
            loaded = PDFDocument.load(path)

            assert loaded.symbol == "AAPL"
            assert loaded.scan_start == "2024-01-01"
            assert "bearish_pin_bar|daily_supertrend_up" in loaded.signals

            sig = loaded.signals["bearish_pin_bar|daily_supertrend_up"]
            assert sig.sample_size == 50
            assert sig.sufficient_samples is True
            assert sig.ci_95_width == pytest.approx(0.003)

            h = sig.horizons["1h"]
            assert h.mean == pytest.approx(-0.001)
            assert h.percentiles["5"] == pytest.approx(-0.009)
            assert len(h.forward_returns) == 5
        finally:
            os.unlink(path)


# ================================================================== #
# 7. Full build pipeline test
# ================================================================== #

class TestPDFBuilderFullPipeline:
    def test_build_produces_signals(self):
        """
        Construct a contrived scenario:
        - 60 LTF bars with a bearish pin bar at indices 5, 15, 25, 35
        - Daily bar with supertrend UP
        - Verify the PDF has entries and forward returns are computed
        """
        base_dt = datetime(2025, 1, 2, 9, 30)
        ltf_bars = []
        for i in range(60):
            dt = base_dt + timedelta(minutes=15 * i)
            # Normal bar
            c = 100 + i * 0.1  # slowly rising price
            bar = _bar(open=c, high=c + 1, low=c - 1, close=c, dt=dt, superD=1)

            # Insert bearish pin bars at specific indices
            if i in (5, 15, 25, 35):
                bar = _bar(
                    open=c, high=c + 20, low=c - 1, close=c + 1,
                    dt=dt, superD=1, stochrsi_k=85,
                )
            ltf_bars.append(bar)

        daily_bars = [
            _bar(superD=1, dt=datetime(2025, 1, 2, 16, 0)),
        ]

        builder = PDFBuilder(
            symbol="TEST",
            ltf_bars=ltf_bars,
            daily_bars=daily_bars,
            horizons={"1h": 4, "4h": 16},
            min_ci_width=1.0,  # lenient threshold so signals pass
        )
        pdf = builder.build()

        assert pdf.symbol == "TEST"
        assert len(pdf.signals) > 0

        # At least one signal should have the bearish pin bar component
        pin_bar_keys = [k for k in pdf.signals if "bearish_pin_bar" in k]
        assert len(pin_bar_keys) > 0, f"No pin bar signals found. Keys: {list(pdf.signals.keys())}"

        # Check that forward returns were computed
        for key in pin_bar_keys:
            sig = pdf.signals[key]
            assert sig.sample_size > 0
            assert "1h" in sig.horizons or "4h" in sig.horizons


# ================================================================== #
# find_best_signal (subset matching)
# ================================================================== #

class TestFindBestSignal:
    """Test PDFDocument.find_best_signal subset matching logic."""

    def _make_pdf(self, signal_map):
        """Build a PDFDocument with given signal_key → (sample_size, ci_width) entries."""
        from pdf_types import HorizonStats, SignalPDF, PDFDocument, compute_percentiles
        signals = {}
        for key, (n, ci) in signal_map.items():
            returns = [0.01] * max(n, 2)
            signals[key] = SignalPDF(
                sample_size=n,
                ci_95_width=ci,
                sufficient_samples=(ci < 0.005),
                horizons={"2d": HorizonStats(
                    mean=0.01, stddev=0.02,
                    percentiles=compute_percentiles(returns),
                    forward_returns=returns,
                )},
            )
        return PDFDocument(
            symbol="TEST", signals=signals,
            scan_start="2024-01-01", scan_end="2024-12-31",
        )

    def test_exact_match_preferred(self):
        """Full compound key is preferred over subsets."""
        pdf = self._make_pdf({
            "a|b|c": (100, 0.003),
            "a|b": (200, 0.002),
            "a": (500, 0.001),
        })
        result = pdf.find_best_signal(["a", "b", "c"], min_ci_width=0.01)
        assert result is not None
        key, entry = result
        assert key == "a|b|c"

    def test_falls_back_to_subset(self):
        """When full key doesn't exist, falls back to largest matching subset."""
        pdf = self._make_pdf({
            "a|b": (200, 0.003),
            "a": (500, 0.001),
        })
        result = pdf.find_best_signal(["a", "b", "c"], min_ci_width=0.01)
        assert result is not None
        key, _ = result
        assert key == "a|b"

    def test_single_signal_fallback(self):
        """Falls back to single signal when no compound matches."""
        pdf = self._make_pdf({
            "b": (300, 0.004),
        })
        result = pdf.find_best_signal(["a", "b", "c"], min_ci_width=0.01)
        assert result is not None
        key, _ = result
        assert key == "b"

    def test_no_match_returns_none(self):
        """No matching signals at all → None."""
        pdf = self._make_pdf({
            "x|y": (100, 0.003),
        })
        result = pdf.find_best_signal(["a", "b"], min_ci_width=0.01)
        assert result is None

    def test_ci_threshold_filters(self):
        """Signals exceeding CI threshold are skipped."""
        pdf = self._make_pdf({
            "a|b": (10, 0.05),   # too wide
            "a": (500, 0.003),   # OK
        })
        result = pdf.find_best_signal(["a", "b"], min_ci_width=0.01)
        assert result is not None
        key, _ = result
        assert key == "a"  # fell through to single because a|b was filtered

    def test_ci_threshold_all_filtered(self):
        """All signals exceed CI threshold → None."""
        pdf = self._make_pdf({
            "a|b": (10, 0.05),
            "a": (20, 0.02),
        })
        result = pdf.find_best_signal(["a", "b"], min_ci_width=0.01)
        assert result is None

    def test_largest_sample_wins_among_ties(self):
        """Among subsets of equal size, largest sample_size wins."""
        pdf = self._make_pdf({
            "a|c": (300, 0.003),
            "a|b": (100, 0.004),
        })
        result = pdf.find_best_signal(["a", "b", "c"], min_ci_width=0.01)
        assert result is not None
        key, _ = result
        assert key == "a|c"

    def test_empty_components(self):
        """Empty component list → None."""
        pdf = self._make_pdf({"a": (100, 0.003)})
        assert pdf.find_best_signal([], min_ci_width=0.01) is None

    def test_single_component(self):
        """Single component matches directly."""
        pdf = self._make_pdf({"a": (100, 0.003)})
        result = pdf.find_best_signal(["a"], min_ci_width=0.01)
        assert result is not None
        assert result[0] == "a"
