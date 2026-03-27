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
            mock_meter.create_gauge.assert_called_once_with(
                "grodt.strategy.heartbeat",
                unit="1",
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
    def test_emits_structured_log(self, caplog):
        with patch("engine.heartbeat.metrics") as mock_metrics:
            mock_metrics.get_meter.return_value = MagicMock()
            from engine.heartbeat import StrategyHeartbeat
            hb = StrategyHeartbeat("test-strategy")
            hb.set_state("active")
            hb.record_tick()

            # Patch the wait timeout to fire immediately
            with caplog.at_level(logging.INFO, logger="grodt.strategy.heartbeat"):
                hb._emit_heartbeat()

            # Check structured log fields
            assert len(caplog.records) >= 1
            record = caplog.records[-1]
            assert record.event == "heartbeat"
            assert record.strategy_name == "test-strategy"
            assert record.state == "active"
            assert record.tick_count == 1
            assert hasattr(record, "last_tick_time")
            assert hasattr(record, "uptime_seconds")
