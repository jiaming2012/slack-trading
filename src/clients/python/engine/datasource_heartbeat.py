"""Datasource heartbeat daemon thread.

Emits a metric gauge and structured log every 30 seconds to indicate
a datasource script is alive. Follows the same pattern as StrategyHeartbeat
but with datasource-specific fields.
"""
import threading
import time
from datetime import datetime, timezone

from loguru import logger
from opentelemetry import metrics

HEARTBEAT_INTERVAL_SECONDS = 30


class DatasourceHeartbeat:
    """Background daemon thread that emits heartbeat gauge + structured log.

    Usage:
        hb = DatasourceHeartbeat("polygon-options", symbol="AAPL")
        hb.start()
        # ... in datasource check loop ...
        hb.record_check()
        # ... on shutdown ...
        hb.stop()
    """

    def __init__(self, datasource_name: str, symbol: str = ""):
        self.datasource_name = datasource_name
        self.symbol = symbol
        self.check_count = 0
        self.last_check_time = None
        self._stop_event = threading.Event()
        self._start_time = time.time()
        self._thread = None

        meter = metrics.get_meter("grodt-datasource")
        self._heartbeat_gauge = meter.create_gauge(
            "grodt.datasource.heartbeat",
            description="Datasource heartbeat (1=alive)",
        )

    def start(self):
        """Start the heartbeat daemon thread."""
        self._thread = threading.Thread(
            target=self._run,
            name=f"heartbeat-ds-{self.datasource_name}",
            daemon=True,
        )
        self._thread.start()

    def stop(self):
        """Stop the heartbeat thread (waits up to 2 seconds)."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=2)

    def record_check(self):
        """Record that a datasource check was performed."""
        self.check_count += 1
        self.last_check_time = datetime.now(timezone.utc).isoformat()

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
                "datasource_name": self.datasource_name,
                "symbol": self.symbol,
            },
        )

        # Emit structured log
        logger.info(
            "heartbeat | datasource={} checks={} uptime={}s symbol={}",
            self.datasource_name, self.check_count, uptime_seconds,
            self.symbol,
        )
