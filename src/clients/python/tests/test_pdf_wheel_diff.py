"""
Behavioral diff test: PDFWheelStrategy (V1) vs PDFWheelStrategyV2 (V2).
Proves zero metric drift -- identical compound signal detection on identical inputs.

This test validates that the datasource extraction in V2 produces the same
check_for_pdf_put_signals behavior as V1's inline detect_atomic_signals_on_bar()
+ PDF lookup logic.

PDFWheel differs from plain Wheel in that signal detection uses compound signals
(candlestick + indicator combos) with PDF lookup, not just supertrend features.
The diff boundary is check_for_pdf_put_signals(): V1 computes inline, V2 delegates
to datasources.pdf_wheel_signals.produce_signals().

Pattern from test_credit_spread_diff.py / test_wheel_diff.py:
1. Import both V1 and V2 strategy classes
2. Patch detect_atomic_signals_on_bar in BOTH source modules
3. Create mock PDFDocument with test signal entries
4. Compare at the signal detection boundary
5. Assert identical signal detection results given identical inputs
"""

from datetime import datetime
from unittest.mock import MagicMock, patch

import pytest
import pandas as pd
import numpy as np

from lib.pdf_types import HorizonStats, PDFDocument, SignalPDF


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_mock_playground(symbol="AAPL", ltf_seconds=900, htf_seconds=86400):
    """Create a mock playground with minimal interface for PDF wheel strategy."""
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


def _make_pdf(
    signal_key="bullish_pin_bar|stochrsi_cross_above_20",
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
        percentiles={
            "5": -0.03, "10": -0.02, "25": -0.005, "50": 0.005,
            "60": 0.008, "70": 0.012, "80": 0.018, "90": 0.025,
        },
        forward_returns=forward_returns,
    )
    signal = SignalPDF(
        sample_size=sample_size,
        ci_95_width=0.008,
        sufficient_samples=True,
        horizons={"1h": horizon, "1d": horizon},
    )
    return PDFDocument(
        symbol="AAPL",
        signals={signal_key: signal},
        scan_start="2025-01-01",
        scan_end="2025-06-01",
        ltf_period_seconds=900,
        htf_period_seconds=86400,
    )


def _make_ltf_bar(close=100.0, dt_str="2025-06-15T10:00:00"):
    """Create a dict simulating an LTF bar."""
    return {
        "open": close - 0.5,
        "high": close + 0.5,
        "low": close - 1.0,
        "close": close,
        "datetime": dt_str,
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


def _setup_pdf_wheel_strategy(strategy_cls, symbol="AAPL", pdf=None):
    """Create a PDF wheel strategy instance with mock playground."""
    pg = _make_mock_playground(symbol)
    if pdf is None:
        pdf = _make_pdf()

    mock_candle_dict = {
        "close": 100.0, "open": 99.5, "high": 100.5, "low": 99.0,
        "datetime": "2025-06-15T10:00:00", "superD_50_3": 1.0,
        "superT_50_3": 98.0, "volume": 1000,
    }

    with patch("deprecated.base_open_strategy_v2.MessageToDict", return_value=mock_candle_dict):
        mock_candle = MagicMock()
        pg.fetch_candles_v2.return_value = [mock_candle]
        strategy = strategy_cls(
            pg, symbol, logger=MagicMock(),
            pdf=pdf,
            max_open_count=5,
            signal_ci_threshold=0.01,
        )

    # Initialize candles_ltf with a minimal DataFrame
    candle_data = [
        (100.0, 1.0, 98.0),
        (101.0, 1.0, 98.5),
        (99.0, -1.0, 101.0),
    ]
    rows = []
    for i, (c, superD, superT) in enumerate(candle_data):
        rows.append({
            "close": c, "open": c - 0.5, "high": c + 0.5, "low": c - 1.0,
            "datetime": f"2025-06-15T{10 + i}:00:00",
            "superD_50_3": superD, "superT_50_3": superT, "volume": 1000,
        })
    total = len(rows) * 2
    df = pd.DataFrame(index=range(total), columns=rows[0].keys())
    for i, row in enumerate(rows):
        df.iloc[i] = row
    strategy.candles_ltf = df
    strategy.candles_ltf_idx = len(candle_data)

    return strategy


# ------------------------------------------------------------------ #
# Diff tests
# ------------------------------------------------------------------ #

class TestPDFWheelV1V2SignalDetectionDiff:
    """Behavioral diff tests proving V1 and V2 produce identical compound signal detection."""

    @patch("datasources.pdf_wheel_signals.detect_atomic_signals_on_bar")
    @patch("strategies.pdf_wheel.detect_atomic_signals_on_bar")
    def test_no_signals_produces_no_pdf_put_signal(
        self, mock_v1_detect, mock_v2_detect,
    ):
        """Both V1 and V2 return None when no atomic signals are detected."""
        from deprecated.pdf_wheel import PDFWheelStrategy
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2

        mock_v1_detect.return_value = []
        mock_v2_detect.return_value = []

        pdf = _make_pdf()
        bar_dict = _make_ltf_bar(100.0)

        strategy_v1 = _setup_pdf_wheel_strategy(PDFWheelStrategy, pdf=pdf)
        strategy_v2 = _setup_pdf_wheel_strategy(PDFWheelStrategyV2, pdf=pdf)

        result_v1 = strategy_v1.check_for_pdf_put_signals(bar_dict)
        result_v2 = strategy_v2.check_for_pdf_put_signals(bar_dict)

        assert result_v1 is None, f"V1 should return None, got {result_v1}"
        assert result_v2 is None, f"V2 should return None, got {result_v2}"

    @patch("datasources.pdf_wheel_signals.detect_atomic_signals_on_bar")
    @patch("strategies.pdf_wheel.detect_atomic_signals_on_bar")
    def test_matching_signal_produces_identical_pdf_put_signal(
        self, mock_v1_detect, mock_v2_detect,
    ):
        """Both V1 and V2 produce identical PDFPutSignal when PDF has a match."""
        from deprecated.pdf_wheel import PDFWheelStrategy, PDFPutSignal
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2

        signal_key = "bullish_pin_bar|stochrsi_cross_above_20"
        pdf = _make_pdf(signal_key=signal_key)

        # Both detect the same atomic signals
        atomic_signals = ["bullish_pin_bar", "stochrsi_cross_above_20"]
        mock_v1_detect.return_value = list(atomic_signals)
        mock_v2_detect.return_value = list(atomic_signals)

        bar_dict = _make_ltf_bar(100.0)

        strategy_v1 = _setup_pdf_wheel_strategy(PDFWheelStrategy, pdf=pdf)
        strategy_v2 = _setup_pdf_wheel_strategy(PDFWheelStrategyV2, pdf=pdf)

        result_v1 = strategy_v1.check_for_pdf_put_signals(bar_dict)
        result_v2 = strategy_v2.check_for_pdf_put_signals(bar_dict)

        assert result_v1 is not None, "V1 should detect PDF put signal"
        assert result_v2 is not None, "V2 should detect PDF put signal"
        assert isinstance(result_v1, PDFPutSignal)
        assert isinstance(result_v2, PDFPutSignal)

        # Compare signal fields
        assert result_v1.symbol == result_v2.symbol
        assert result_v1.name == result_v2.name
        assert result_v1.price == result_v2.price
        assert result_v1.compound_key == result_v2.compound_key
        assert result_v1.pdf_entry == result_v2.pdf_entry

    @patch("datasources.pdf_wheel_signals.detect_atomic_signals_on_bar")
    @patch("strategies.pdf_wheel.detect_atomic_signals_on_bar")
    def test_signal_with_no_pdf_match_produces_no_signal(
        self, mock_v1_detect, mock_v2_detect,
    ):
        """Both V1 and V2 return None when PDF has no matching entry."""
        from deprecated.pdf_wheel import PDFWheelStrategy
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2

        # PDF has a different signal key than what will be detected
        pdf = _make_pdf(signal_key="hammer|supertrend_flip_up")

        # Detect signals that don't match the PDF
        mock_v1_detect.return_value = ["doji", "sma_50_cross_above"]
        mock_v2_detect.return_value = ["doji", "sma_50_cross_above"]

        bar_dict = _make_ltf_bar(100.0)

        strategy_v1 = _setup_pdf_wheel_strategy(PDFWheelStrategy, pdf=pdf)
        strategy_v2 = _setup_pdf_wheel_strategy(PDFWheelStrategyV2, pdf=pdf)

        result_v1 = strategy_v1.check_for_pdf_put_signals(bar_dict)
        result_v2 = strategy_v2.check_for_pdf_put_signals(bar_dict)

        assert result_v1 is None, f"V1 should return None (no PDF match), got {result_v1}"
        assert result_v2 is None, f"V2 should return None (no PDF match), got {result_v2}"

    @patch("datasources.pdf_wheel_signals.detect_atomic_signals_on_bar")
    @patch("strategies.pdf_wheel.detect_atomic_signals_on_bar")
    def test_daily_context_signals_included_in_compound_key(
        self, mock_v1_detect, mock_v2_detect,
    ):
        """Both V1 and V2 merge daily context signals into the compound key."""
        from deprecated.pdf_wheel import PDFWheelStrategy, PDFPutSignal
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2

        # Signal key includes both LTF and daily signals
        signal_key = "daily_supertrend_up|stochrsi_cross_above_20"
        pdf = _make_pdf(signal_key=signal_key)

        # LTF signals: only stochrsi
        mock_v1_detect.return_value = ["stochrsi_cross_above_20"]
        mock_v2_detect.return_value = ["stochrsi_cross_above_20"]

        bar_dict = _make_ltf_bar(100.0, dt_str="2025-06-15T10:00:00")

        strategy_v1 = _setup_pdf_wheel_strategy(PDFWheelStrategy, pdf=pdf)
        strategy_v2 = _setup_pdf_wheel_strategy(PDFWheelStrategyV2, pdf=pdf)

        # Simulate daily context: daily_supertrend_up was detected earlier
        strategy_v1._daily_signals["2025-06-15"] = ["daily_supertrend_up"]
        strategy_v2._daily_signals["2025-06-15"] = ["daily_supertrend_up"]

        result_v1 = strategy_v1.check_for_pdf_put_signals(bar_dict)
        result_v2 = strategy_v2.check_for_pdf_put_signals(bar_dict)

        assert result_v1 is not None, "V1 should detect signal with daily context"
        assert result_v2 is not None, "V2 should detect signal with daily context"

        assert result_v1.compound_key == result_v2.compound_key
        assert "daily_supertrend_up" in result_v1.compound_key
        assert "stochrsi_cross_above_20" in result_v1.compound_key


class TestPDFWheelV2InheritanceChain:
    """Verify PDFWheelStrategyV2 has the correct inheritance chain per D-06."""

    def test_inheritance_chain(self):
        """PDFWheelStrategyV2 -> WheelStrategyV2 -> OptionsStrategyBasicV2 -> BaseStrategy."""
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2
        from strategies.wheel_v2 import WheelStrategyV2
        from strategies.covered_call_v2 import OptionsStrategyBasicV2
        from strategies.base_strategy import BaseStrategy

        assert issubclass(PDFWheelStrategyV2, WheelStrategyV2), (
            "PDFWheelStrategyV2 must extend WheelStrategyV2"
        )
        assert issubclass(PDFWheelStrategyV2, OptionsStrategyBasicV2), (
            "PDFWheelStrategyV2 must ultimately extend OptionsStrategyBasicV2"
        )
        assert issubclass(PDFWheelStrategyV2, BaseStrategy), (
            "PDFWheelStrategyV2 must ultimately extend BaseStrategy"
        )

    def test_v2_uses_datasource_import(self):
        """V2 strategy imports produce_signals from pdf_wheel_signals datasource."""
        from strategies.pdf_wheel_v2 import PDFWheelStrategyV2

        signal_key = "bullish_pin_bar|stochrsi_cross_above_20"
        pdf = _make_pdf(signal_key=signal_key)

        strategy = _setup_pdf_wheel_strategy(PDFWheelStrategyV2, pdf=pdf)

        with patch("strategies.pdf_wheel_v2.produce_signals") as mock_produce:
            mock_produce.return_value = [{
                "compound_key": signal_key,
                "bar_dict": _make_ltf_bar(100.0),
                "pdf_entry": pdf.signals[signal_key],
                "price": 100.0,
                "timestamp": datetime(2025, 6, 15, 10, 0),
                "signal_name": "PDF_SHORT_PUT_SIGNAL",
            }]

            bar_dict = _make_ltf_bar(100.0)
            result = strategy.check_for_pdf_put_signals(bar_dict)

            mock_produce.assert_called_once()
            assert result is not None
            assert result.name == "PDF_SHORT_PUT_SIGNAL"
            assert result.compound_key == signal_key
