"""Tests for OTel instrumentation wiring in trading_engine.run_strategy().

Verifies:
- OTel setup is called on entry
- Heartbeat lifecycle (start → active → idle → stop)
- Live mode creates spans, simulator mode does not
- _flush_decisions() called after on_tick()
- Heartbeat.record_tick() called each iteration
- Cleanup in finally block (heartbeat stop, on_complete, otel_shutdown)
- Exception handling preserves cleanup
"""
import unittest
from unittest.mock import MagicMock, patch, call
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor


class MockStrategy:
    """Minimal strategy mock that runs for N ticks then completes."""

    def __init__(self, ticks_before_complete=2):
        self._ticks_remaining = ticks_before_complete
        self._decisions = []
        self.symbol = "AAPL"
        self.on_tick_calls = 0
        self.on_retrain_calls = 0
        self.on_complete_calls = 0

    def is_complete(self):
        return self._ticks_remaining <= 0

    def on_tick(self, tick_deltas):
        self.on_tick_calls += 1
        self._ticks_remaining -= 1

    def get_next_tick_seconds(self):
        return 60

    def should_fetch_account(self):
        return True

    def on_retrain(self):
        self.on_retrain_calls += 1

    def on_complete(self):
        self.on_complete_calls += 1

    def _flush_decisions(self):
        self._decisions.clear()

    def record_decision(self, decision):
        self._decisions.append(decision)


class MockPlayground:
    """Minimal playground mock."""

    def __init__(self, environment="simulator"):
        self.id = "test-playground-123"
        self.environment = environment
        self.tick_calls = 0

    def flush_new_state_buffer(self):
        return []

    def tick(self, seconds, fetch_account=True):
        self.tick_calls += 1


class TestRunStrategyOtelSetup(unittest.TestCase):
    """Test that run_strategy() calls setup_otel() on entry."""

    @patch("engine.trading_engine.setup_otel")
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_calls_setup_otel(self, mock_hb_cls, mock_setup):
        from engine.trading_engine import run_strategy

        mock_setup.return_value = MagicMock()
        mock_hb = MagicMock()
        mock_hb_cls.return_value = mock_hb

        strategy = MockStrategy(ticks_before_complete=1)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        mock_setup.assert_called_once()

    @patch("engine.trading_engine.setup_otel")
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_calls_otel_shutdown_in_finally(self, mock_hb_cls, mock_setup):
        from engine.trading_engine import run_strategy

        mock_shutdown = MagicMock()
        mock_setup.return_value = mock_shutdown
        mock_hb_cls.return_value = MagicMock()

        strategy = MockStrategy(ticks_before_complete=1)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        mock_shutdown.assert_called_once()


class TestRunStrategyHeartbeatLifecycle(unittest.TestCase):
    """Test heartbeat start/stop lifecycle around run_strategy()."""

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_heartbeat_started_and_stopped(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb = MagicMock()
        mock_hb_cls.return_value = mock_hb

        strategy = MockStrategy(ticks_before_complete=2)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        mock_hb.start.assert_called_once()
        mock_hb.stop.assert_called_once()

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_heartbeat_state_transitions(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb = MagicMock()
        mock_hb_cls.return_value = mock_hb

        strategy = MockStrategy(ticks_before_complete=1)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        # Should set "active" at start and "idle" in finally
        mock_hb.set_state.assert_any_call("active")
        mock_hb.set_state.assert_any_call("idle")

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_heartbeat_record_tick_called_each_iteration(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb = MagicMock()
        mock_hb_cls.return_value = mock_hb

        strategy = MockStrategy(ticks_before_complete=3)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        assert mock_hb.record_tick.call_count == 3


class _ListExporter:
    """Simple exporter that collects spans in a list."""

    def __init__(self):
        self.spans = []

    def export(self, spans):
        self.spans.extend(spans)
        from opentelemetry.sdk.trace.export import SpanExportResult
        return SpanExportResult.SUCCESS

    def shutdown(self):
        pass

    def force_flush(self, timeout_millis=None):
        return True


class TestRunStrategyLiveVsSimulator(unittest.TestCase):
    """Test span creation differs between live and simulator modes."""

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_live_mode_creates_spans(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb_cls.return_value = MagicMock()

        # Create isolated provider + exporter, patch trace.get_tracer to use it
        exporter = _ListExporter()
        provider = TracerProvider()
        provider.add_span_processor(SimpleSpanProcessor(exporter))

        with patch("engine.trading_engine.trace") as mock_trace:
            mock_trace.get_tracer.return_value = provider.get_tracer("test")

            strategy = MockStrategy(ticks_before_complete=2)
            playground = MockPlayground(environment="live")

            run_strategy(strategy, playground, MagicMock())

        provider.force_flush()
        tick_spans = [s for s in exporter.spans if s.name == "strategy.tick"]
        assert len(tick_spans) == 2, f"Expected 2 tick spans, got {len(tick_spans)}"

        # Verify span attributes
        attrs = dict(tick_spans[0].attributes)
        assert attrs["playground_id"] == "test-playground-123"
        assert attrs["symbol"] == "AAPL"
        assert attrs["strategy_name"] == "MockStrategy"
        assert attrs["tick_number"] == 1

        provider.shutdown()

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_simulator_mode_no_spans(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb_cls.return_value = MagicMock()

        exporter = _ListExporter()
        provider = TracerProvider()
        provider.add_span_processor(SimpleSpanProcessor(exporter))

        with patch("engine.trading_engine.trace") as mock_trace:
            mock_trace.get_tracer.return_value = provider.get_tracer("test")

            strategy = MockStrategy(ticks_before_complete=2)
            playground = MockPlayground(environment="simulator")

            run_strategy(strategy, playground, MagicMock())

        provider.force_flush()
        tick_spans = [s for s in exporter.spans if s.name == "strategy.tick"]
        assert len(tick_spans) == 0, f"Expected 0 tick spans in simulator mode, got {len(tick_spans)}"

        provider.shutdown()


class TestRunStrategyFlushDecisions(unittest.TestCase):
    """Test that _flush_decisions() is called after each on_tick()."""

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_flush_decisions_called_after_tick(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb_cls.return_value = MagicMock()

        strategy = MockStrategy(ticks_before_complete=2)
        strategy._flush_decisions = MagicMock()
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        assert strategy._flush_decisions.call_count == 2


class TestRunStrategyCleanup(unittest.TestCase):
    """Test cleanup in finally block."""

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_on_complete_called(self, mock_hb_cls, _):
        from engine.trading_engine import run_strategy

        mock_hb_cls.return_value = MagicMock()

        strategy = MockStrategy(ticks_before_complete=1)
        playground = MockPlayground()

        run_strategy(strategy, playground, MagicMock())

        assert strategy.on_complete_calls == 1

    @patch("engine.trading_engine.setup_otel", return_value=MagicMock())
    @patch("engine.trading_engine.StrategyHeartbeat")
    def test_cleanup_on_exception(self, mock_hb_cls, mock_setup):
        from engine.trading_engine import run_strategy

        mock_shutdown = MagicMock()
        mock_setup.return_value = mock_shutdown
        mock_hb = MagicMock()
        mock_hb_cls.return_value = mock_hb

        strategy = MockStrategy(ticks_before_complete=3)
        strategy.on_tick = MagicMock(side_effect=RuntimeError("boom"))
        playground = MockPlayground()

        with self.assertRaises(RuntimeError):
            run_strategy(strategy, playground, MagicMock())

        # Cleanup should still happen
        mock_hb.set_state.assert_any_call("idle")
        mock_hb.stop.assert_called_once()
        mock_shutdown.assert_called_once()


if __name__ == "__main__":
    unittest.main()
