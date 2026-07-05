"""Strategy heartbeat daemon thread.

Reports a liveness signal to the trading server every 30 seconds via
POST /telemetry/heartbeat (ADR-0005: the in-process Telemetry module
replaces the OTel pipeline). A delivery failure logs a warning and never
disturbs the strategy -- the thread keeps trying on subsequent beats.
"""
import os
import threading
import time
from datetime import datetime, timezone

import requests
from loguru import logger

HEARTBEAT_INTERVAL_SECONDS = 30
HEARTBEAT_TIMEOUT_SECONDS = 5


def telemetry_host() -> str:
    """Base URL of the trading server's REST surface."""
    return os.getenv("TELEMETRY_HOST", "http://localhost:8080")


def post_heartbeat(kind: str, name: str, meta: dict) -> bool:
    """POST one heartbeat; returns True on success, warns and returns False otherwise."""
    url = f"{telemetry_host()}/telemetry/heartbeat"
    try:
        resp = requests.post(
            url,
            json={"kind": kind, "name": name, "meta": meta},
            timeout=HEARTBEAT_TIMEOUT_SECONDS,
        )
        if resp.status_code >= 300:
            logger.warning("heartbeat post to {} returned {}: {}", url, resp.status_code, resp.text)
            return False
        return True
    except requests.RequestException as e:
        logger.warning("heartbeat post to {} failed (will retry next beat): {}", url, e)
        return False


class StrategyHeartbeat:
    """Background daemon thread that reports strategy liveness to the server.

    Usage:
        hb = StrategyHeartbeat("covered-call")
        hb.start()
        # ... in tick loop ...
        hb.record_tick()
        hb.set_state("active")
        # ... on shutdown ...
        hb.stop()
    """

    def __init__(self, strategy_name: str, playground_id: str = "", client_id: str = ""):
        self.strategy_name = strategy_name
        self.playground_id = playground_id
        self.client_id = client_id
        self.state = "idle"
        self.tick_count = 0
        self.last_tick_time = None
        self._stop_event = threading.Event()
        self._start_time = time.time()
        self._thread = None

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
        """Report liveness to the server and emit a structured log."""
        uptime_seconds = round(time.time() - self._start_time, 1)

        post_heartbeat(
            "strategy",
            self.strategy_name,
            {
                "state": self.state,
                "tick_count": str(self.tick_count),
                "playground_id": self.playground_id,
                "client_id": self.client_id,
            },
        )

        logger.info(
            "heartbeat | strategy={} state={} ticks={} uptime={}s playground_id={} client_id={}",
            self.strategy_name, self.state, self.tick_count, uptime_seconds,
            self.playground_id, self.client_id,
        )
