"""Unit tests for datasources.ma_crossover.produce_signals().

Validates the datasource module in isolation using mocked signal detection.
Tests cover: no-signal, None PDF, insufficient samples, and valid signal cases.
"""

from datetime import datetime
from unittest.mock import patch, MagicMock

import pytest

from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_bar(close=100.0):
    """Create a minimal HTF bar dict."""
    return {
        "open": close - 0.5,
        "high": close + 0.5,
        "low": close - 1.0,
        "close": close,
        "datetime": datetime(2025, 6, 15, 10, 0),
        "superD_50_3": 1.0,
        "stochrsi_k_14_14_3_3": 25.0,
        "stochrsi_cross_above_20": 1.0,
        "stochrsi_cross_below_80": 0.0,
        "sma_50": close - 2.0,
        "sma_100": close - 3.0,
        "sma_200": close - 5.0,
    }


def _make_pdf(signal_key="bullish_supertrend|stochrsi_cross_above_20",
              sufficient=True, sample_size=100):
    """Create a minimal PDFDocument with one signal."""
    horizon = HorizonStats(
        mean=0.005,
        stddev=0.02,
        percentiles={"5": -0.03, "10": -0.02, "25": -0.005, "50": 0.005},
        forward_returns=[0.001] * sample_size,
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


# ------------------------------------------------------------------ #
# Tests
# ------------------------------------------------------------------ #

class TestProduceSignals:

    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    def test_no_signals_returns_empty(self, mock_detect):
        """No atomic signals detected -> empty list."""
        mock_detect.return_value = []
        from datasources.ma_crossover import produce_signals

        bar = _make_bar()
        prev_bar = _make_bar(99.0)
        pdf = _make_pdf()

        result = produce_signals(bar, prev_bar, pdf)
        assert result == []

    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    def test_none_pdf_returns_empty(self, mock_detect):
        """Signals detected but pdf is None -> empty list."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        from datasources.ma_crossover import produce_signals

        bar = _make_bar()
        prev_bar = _make_bar(99.0)

        result = produce_signals(bar, prev_bar, None)
        assert result == []

    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    def test_insufficient_samples_returns_empty(self, mock_detect):
        """Signal key exists in PDF but has insufficient samples -> empty list."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        from datasources.ma_crossover import produce_signals

        bar = _make_bar()
        prev_bar = _make_bar(99.0)
        pdf = _make_pdf(sufficient=False)

        result = produce_signals(bar, prev_bar, pdf)
        assert result == []

    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    def test_valid_signal_returns_dict(self, mock_detect):
        """Valid signal with sufficient samples -> list with one signal dict."""
        mock_detect.return_value = [
            "bullish_supertrend", "stochrsi_cross_above_20",
        ]
        from datasources.ma_crossover import produce_signals

        bar = _make_bar()
        prev_bar = _make_bar(99.0)
        pdf = _make_pdf()

        result = produce_signals(bar, prev_bar, pdf)

        assert len(result) == 1
        sig = result[0]
        assert "signal_key" in sig
        assert "bar_dict" in sig
        assert "pdf_entry" in sig
        assert sig["signal_key"] == "bullish_supertrend|stochrsi_cross_above_20"
        assert sig["bar_dict"] is bar
        assert sig["pdf_entry"].sufficient_samples is True

    @patch("datasources.ma_crossover.detect_atomic_signals_on_bar")
    def test_signal_key_not_in_pdf_returns_empty(self, mock_detect):
        """Signals detected but compound key not in PDF -> empty list."""
        mock_detect.return_value = ["some_unknown_signal"]
        from datasources.ma_crossover import produce_signals

        bar = _make_bar()
        prev_bar = _make_bar(99.0)
        pdf = _make_pdf()  # has "bullish_supertrend|stochrsi_cross_above_20" only

        result = produce_signals(bar, prev_bar, pdf)
        assert result == []
