"""
Tests for signal decision logging in BaseStrategy.

Covers:
- SignalDecision dataclass fields and construction
- BaseStrategy.record_decision() auto-fills trace_id, playground_id, symbol
- BaseStrategy._log_decision() verbose vs non-verbose mode
- BaseStrategy._flush_decisions() logs all and clears list
- "skip" and "place" decisions produce correct structured logs
"""

import logging
import os
from unittest.mock import MagicMock, patch

import pytest

from engine.types import SignalDecision
from strategies.base_strategy import BaseStrategy


class ConcreteTestStrategy(BaseStrategy):
    """Minimal concrete strategy for testing."""

    def __init__(self, playground, symbol, **kwargs):
        super().__init__(playground, symbol, **kwargs)

    def on_tick(self, tick_deltas):
        pass

    def get_next_tick_seconds(self):
        return 60

    def should_fetch_account(self):
        return False


@pytest.fixture
def mock_playground():
    pg = MagicMock()
    pg.id = "test-playground-123"
    pg.is_backtest_complete.return_value = False
    return pg


@pytest.fixture
def strategy(mock_playground):
    return ConcreteTestStrategy(mock_playground, "AAPL")


class TestSignalDecisionDataclass:
    """Test SignalDecision dataclass has all required fields."""

    def test_signal_decision_has_required_fields(self):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="pg-123",
        )
        assert decision.signal_type == "covered_call"
        assert decision.direction == "long"
        assert decision.decision == "place"
        assert decision.reason == "signal triggered"
        assert decision.symbol == "AAPL"
        assert decision.playground_id == "pg-123"
        assert decision.trace_id == ""
        assert decision.indicators is None

    def test_signal_decision_with_indicators(self):
        indicators = {"rsi": 72.5, "macd": 1.2, "sma_20": 150.0}
        decision = SignalDecision(
            signal_type="mean_reversion_dip",
            direction="short",
            decision="skip",
            reason="below threshold",
            symbol="MSFT",
            playground_id="pg-456",
            trace_id="abc123",
            indicators=indicators,
        )
        assert decision.indicators == indicators
        assert decision.trace_id == "abc123"


class TestRecordDecision:
    """Test BaseStrategy.record_decision() auto-fill behavior."""

    def test_record_decision_appends_to_list(self, strategy):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="test-playground-123",
        )
        strategy.record_decision(decision)
        assert len(strategy._decisions) == 1
        assert strategy._decisions[0].signal_type == "covered_call"

    def test_record_decision_auto_fills_playground_id(self, strategy):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="",
            playground_id="",
        )
        strategy.record_decision(decision)
        assert strategy._decisions[0].playground_id == "test-playground-123"

    def test_record_decision_auto_fills_symbol(self, strategy):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="",
            playground_id="",
        )
        strategy.record_decision(decision)
        assert strategy._decisions[0].symbol == "AAPL"

    @patch("strategies.base_strategy.trace")
    def test_record_decision_auto_fills_trace_id_from_otel(self, mock_trace, strategy):
        mock_span = MagicMock()
        mock_ctx = MagicMock()
        mock_ctx.trace_id = 0x0123456789ABCDEF0123456789ABCDEF
        mock_span.get_span_context.return_value = mock_ctx
        mock_trace.get_current_span.return_value = mock_span

        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="pg-123",
        )
        strategy.record_decision(decision)
        assert strategy._decisions[0].trace_id == "0123456789abcdef0123456789abcdef"

    @patch("strategies.base_strategy.trace")
    def test_record_decision_empty_trace_id_when_no_span(self, mock_trace, strategy):
        mock_span = MagicMock()
        mock_ctx = MagicMock()
        mock_ctx.trace_id = 0  # no active trace
        mock_span.get_span_context.return_value = mock_ctx
        mock_trace.get_current_span.return_value = mock_span

        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="skip",
            reason="no signal",
            symbol="AAPL",
            playground_id="pg-123",
        )
        strategy.record_decision(decision)
        assert strategy._decisions[0].trace_id == ""


class TestLogDecision:
    """Test BaseStrategy._log_decision() structured logging behavior."""

    def test_log_decision_excludes_indicators_when_not_verbose(self, strategy, caplog):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="pg-123",
            indicators={"rsi": 72.5},
        )
        with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
            strategy._log_decision(decision)

        assert len(caplog.records) == 1
        record = caplog.records[0]
        assert record.signal_type == "covered_call"
        assert record.direction == "long"
        assert record.decision == "place"
        assert record.reason == "signal triggered"
        assert not hasattr(record, "indicators")

    def test_log_decision_includes_indicators_when_verbose(self, strategy, caplog):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="pg-123",
            indicators={"rsi": 72.5, "macd": 1.2},
        )
        with patch.dict(os.environ, {"STRATEGY_LOG_VERBOSE": "true"}):
            with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
                strategy._log_decision(decision)

        assert len(caplog.records) == 1
        record = caplog.records[0]
        assert record.indicators == {"rsi": 72.5, "macd": 1.2}

    def test_skip_decision_with_reason(self, strategy, caplog):
        decision = SignalDecision(
            signal_type="mean_reversion_dip",
            direction="neutral",
            decision="skip",
            reason="below threshold",
            symbol="MSFT",
            playground_id="pg-789",
        )
        with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
            strategy._log_decision(decision)

        assert len(caplog.records) == 1
        record = caplog.records[0]
        assert record.decision == "skip"
        assert record.reason == "below threshold"
        assert record.signal_type == "mean_reversion_dip"

    def test_place_decision_produces_correct_log(self, strategy, caplog):
        decision = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="signal triggered",
            symbol="AAPL",
            playground_id="pg-123",
            trace_id="abc123def456",
        )
        with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
            strategy._log_decision(decision)

        assert len(caplog.records) == 1
        record = caplog.records[0]
        assert record.decision == "place"
        assert record.symbol == "AAPL"
        assert record.playground_id == "pg-123"
        assert record.trace_id == "abc123def456"


class TestFlushDecisions:
    """Test BaseStrategy._flush_decisions() logs all and clears."""

    def test_flush_decisions_logs_all_and_clears(self, strategy, caplog):
        d1 = SignalDecision(
            signal_type="covered_call",
            direction="long",
            decision="place",
            reason="triggered",
            symbol="AAPL",
            playground_id="pg-1",
        )
        d2 = SignalDecision(
            signal_type="mean_reversion",
            direction="neutral",
            decision="skip",
            reason="below threshold",
            symbol="MSFT",
            playground_id="pg-2",
        )
        strategy._decisions = [d1, d2]

        with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
            strategy._flush_decisions()

        assert len(caplog.records) == 2
        assert strategy._decisions == []

    def test_flush_decisions_noop_when_empty(self, strategy, caplog):
        with caplog.at_level(logging.INFO, logger="grodt.strategy.signal"):
            strategy._flush_decisions()

        assert len(caplog.records) == 0
        assert strategy._decisions == []
