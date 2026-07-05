"""Tests for engine/heartbeat.py StrategyHeartbeat daemon thread."""
import logging
import time
import pytest
from unittest.mock import MagicMock, patch


class TestStrategyHeartbeatConstruction:
    def test_creates_gauge_metric(self):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_meter = MagicMock()
            mock_metrics.get_meter.return_value = mock_meter
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            # unit="1" was intentionally dropped in commit 72da6a35 ("Remove
            # unit=\"1\" from gauge to avoid _ratio suffix in Prometheus").
            mock_meter.create_gauge.assert_called_once_with(
                "grodt.strategy.heartbeat",
                description="Strategy heartbeat (1=alive)",
            )


class TestStrategyHeartbeatLifecycle:
    def test_start_creates_daemon_thread(self):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_metrics.get_meter.return_value = MagicMock()
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            hb.start()
            assert hb._thread is not None
            assert hb._thread.is_alive()
            assert hb._thread.daemon is True
            hb.stop()

    def test_stop_terminates_thread(self):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_metrics.get_meter.return_value = MagicMock()
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            hb.start()
            assert hb._thread.is_alive()
            hb.stop()
            time.sleep(0.2)
            assert not hb._thread.is_alive()


class TestStrategyHeartbeatState:
    def test_record_tick_increments_count(self):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_metrics.get_meter.return_value = MagicMock()
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            assert hb.tick_count == 0
            assert hb.last_tick_time is None
            hb.record_tick()
            assert hb.tick_count == 1
            assert hb.last_tick_time is not None
            hb.record_tick()
            assert hb.tick_count == 2

    def test_set_state_updates_state(self):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_metrics.get_meter.return_value = MagicMock()
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            assert hb.state == "idle"
            hb.set_state("active")
            assert hb.state == "active"


class TestStrategyHeartbeatLogging:
    def test_emits_structured_log(self):
        # Commit 72da6a35 switched the heartbeat logger from stdlib logging to
        # loguru, so caplog (which captures stdlib records) no longer sees it.
        # Capture the loguru record directly and assert the emitted message
        # carries the strategy/state/tick fields.
        from loguru import logger as loguru_logger

        captured = []
        sink_id = loguru_logger.add(lambda m: captured.append(m.record), level="INFO")
        try:
            with patch("engine.heartbeat.metrics") as mock_metrics:
                mock_metrics.get_meter.return_value = MagicMock()
                from engine.heartbeat import StrategyHeartbeat
                hb = StrategyHeartbeat("test-strategy")
                hb.set_state("active")
                hb.record_tick()

                hb._emit_heartbeat()
        finally:
            loguru_logger.remove(sink_id)

        heartbeat_msgs = [r["message"] for r in captured if r["message"].startswith("heartbeat")]
        assert len(heartbeat_msgs) >= 1
        msg = heartbeat_msgs[-1]
        assert "strategy=test-strategy" in msg
        assert "state=active" in msg
        assert "ticks=1" in msg
        assert "uptime=" in msg
