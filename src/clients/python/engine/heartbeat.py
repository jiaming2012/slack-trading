"""Strategy heartbeat daemon thread.

Emits a metric gauge and structured log every 30 seconds to indicate
the strategy is alive. Mirrors the Go server heartbeat pattern.
"""
import logging
import threading
import time
from datetime import datetime, timezone

from opentelemetry import metrics

logger = logging.getLogger("grodt.strategy.heartbeat")

HEARTBEAT_INTERVAL_SECONDS = 30


class StrategyHeartbeat:
    """Background daemon thread that emits heartbeat gauge + structured log.

    Usage:
        hb = StrategyHeartbeat("covered-call")
        hb.start()
        # ... in tick loop ...
        hb.record_tick()
        hb.set_state("active")
        # ... on shutdown ...
        hb.stop()
    """

    def __init__(self, strategy_name: str):
        self.strategy_name = strategy_name
        self.state = "idle"
        self.tick_count = 0
        self.last_tick_time = None
        self._stop_event = threading.Event()
        self._start_time = time.time()
        self._thread = None

        meter = metrics.get_meter("grodt-strategy")
        self._heartbeat_gauge = meter.create_gauge(
            "grodt.strategy.heartbeat",
            unit="1",
            description="Strategy heartbeat (1=alive)",
        )

    def start(self):
        """Start the heartbeat daemon thread."""
        self._thread = threading.Thread(
            target=self._run,
            name=f"heartbeat-{self.strategy_name}",
            daemon=True,
        )
        self._thread.start()

    def stop(self):
        """Stop the heartbeat thread (waits up to 2 seconds)."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=2)

    def record_tick(self):
        """Record that a tick was processed."""
        self.tick_count += 1
        self.last_tick_time = datetime.now(timezone.utc).isoformat()

    def set_state(self, state: str):
        """Update the strategy state (e.g. 'active', 'idle')."""
        self.state = state

    def _run(self):
        """Main heartbeat loop -- runs in daemon thread."""
        while not self._stop_event.wait(timeout=HEARTBEAT_INTERVAL_SECONDS):
            self._emit_heartbeat()

    def _emit_heartbeat(self):
        """Emit gauge metric and structured log."""
        uptime_seconds = round(time.time() - self._start_time, 1)

        # Set gauge metric
        self._heartbeat_gauge.set(
            1,
            attributes={
                "strategy_name": self.strategy_name,
                "state": self.state,
            },
        )

        # Emit structured log
        logger.info(
            "heartbeat",
            extra={
                "event": "heartbeat",
                "strategy_name": self.strategy_name,
                "state": self.state,
                "tick_count": self.tick_count,
                "last_tick_time": self.last_tick_time,
                "uptime_seconds": uptime_seconds,
            },
        )
