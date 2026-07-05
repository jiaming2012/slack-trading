"""Tests for the server-reporting heartbeat daemons.

StrategyHeartbeat and DatasourceHeartbeat now POST to the trading server's
/telemetry/heartbeat endpoint (ADR-0005) instead of recording OTel gauges.
Delivery failures must warn and never disturb the strategy.
"""
from unittest.mock import MagicMock, patch

import requests

from engine.heartbeat import StrategyHeartbeat, post_heartbeat, telemetry_host
from engine.datasource_heartbeat import DatasourceHeartbeat


class TestTelemetryHost:
    def test_default_host(self, monkeypatch):
        monkeypatch.delenv("TELEMETRY_HOST", raising=False)
        assert telemetry_host() == "http://localhost:8080"

    def test_env_override(self, monkeypatch):
        monkeypatch.setenv("TELEMETRY_HOST", "http://grodt.example:8080")
        assert telemetry_host() == "http://grodt.example:8080"


class TestPostHeartbeat:
    def test_posts_kind_name_meta(self):
        with patch("engine.heartbeat.requests.post") as mock_post:
            mock_post.return_value = MagicMock(status_code=200)
            ok = post_heartbeat("strategy", "covered-call", {"state": "active"})

        assert ok is True
        args, kwargs = mock_post.call_args
        assert args[0].endswith("/telemetry/heartbeat")
        assert kwargs["json"] == {
            "kind": "strategy",
            "name": "covered-call",
            "meta": {"state": "active"},
        }

    def test_connection_failure_warns_and_returns_false(self):
        with patch("engine.heartbeat.requests.post", side_effect=requests.ConnectionError("refused")):
            ok = post_heartbeat("strategy", "covered-call", {})
        assert ok is False

    def test_non_2xx_returns_false(self):
        with patch("engine.heartbeat.requests.post") as mock_post:
            mock_post.return_value = MagicMock(status_code=400, text="bad kind")
            ok = post_heartbeat("bogus", "covered-call", {})
        assert ok is False


class TestStrategyHeartbeatLifecycle:
    def test_start_creates_daemon_thread(self):
        hb = StrategyHeartbeat("test-strategy")
        hb.start()
        try:
            assert hb._thread is not None
            assert hb._thread.is_alive()
            assert hb._thread.daemon is True
        finally:
            hb.stop()

    def test_stop_terminates_thread(self):
        hb = StrategyHeartbeat("test-strategy")
        hb.start()
        hb.stop()
        assert not hb._thread.is_alive()

    def test_record_tick_and_state(self):
        hb = StrategyHeartbeat("test-strategy")
        hb.record_tick()
        hb.record_tick()
        hb.set_state("active")
        assert hb.tick_count == 2
        assert hb.state == "active"
        assert hb.last_tick_time is not None


class TestStrategyHeartbeatEmission:
    def test_emit_posts_strategy_beat(self):
        hb = StrategyHeartbeat("covered-call", playground_id="pg-1", client_id="cc-v7")
        hb.set_state("active")
        hb.record_tick()

        with patch("engine.heartbeat.post_heartbeat") as mock_beat:
            hb._emit_heartbeat()

        mock_beat.assert_called_once_with(
            "strategy",
            "covered-call",
            {
                "state": "active",
                "tick_count": "1",
                "playground_id": "pg-1",
                "client_id": "cc-v7",
            },
        )

    def test_delivery_failure_does_not_raise(self):
        hb = StrategyHeartbeat("covered-call")
        with patch("engine.heartbeat.requests.post", side_effect=requests.ConnectionError("down")):
            hb._emit_heartbeat()  # must not raise -- strategy is undisturbed


class TestDatasourceHeartbeat:
    def test_emit_posts_datasource_beat(self):
        hb = DatasourceHeartbeat("polygon-options", symbol="AAPL")
        hb.record_check()

        with patch("engine.datasource_heartbeat.post_heartbeat") as mock_beat:
            hb._emit_heartbeat()

        mock_beat.assert_called_once_with(
            "datasource",
            "polygon-options",
            {"symbol": "AAPL", "check_count": "1"},
        )

    def test_thread_lifecycle(self):
        hb = DatasourceHeartbeat("polygon-options")
        hb.start()
        try:
            assert hb._thread.is_alive()
            assert hb._thread.daemon is True
        finally:
            hb.stop()
        assert not hb._thread.is_alive()
