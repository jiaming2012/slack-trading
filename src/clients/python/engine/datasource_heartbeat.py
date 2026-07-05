"""Datasource heartbeat daemon thread.

Reports a liveness signal to the trading server every 30 seconds via
POST /telemetry/heartbeat, following the same pattern as StrategyHeartbeat
but with datasource-specific fields. Delivery failures warn and never
disturb the datasource loop.
"""
import threading
import time
from datetime import datetime, timezone

from loguru import logger

from engine.heartbeat import HEARTBEAT_INTERVAL_SECONDS, post_heartbeat


class DatasourceHeartbeat:
    """Background daemon thread that reports datasource liveness to the server.

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
        """Report liveness to the server and emit a structured log."""
        uptime_seconds = round(time.time() - self._start_time, 1)

        post_heartbeat(
            "datasource",
            self.datasource_name,
            {
                "symbol": self.symbol,
                "check_count": str(self.check_count),
            },
        )

        logger.info(
            "heartbeat | datasource={} checks={} uptime={}s symbol={}",
            self.datasource_name, self.check_count, uptime_seconds,
            self.symbol,
        )
