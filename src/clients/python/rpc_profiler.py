"""Lightweight RPC profiler for the credit spread backtester.

Tracks wall-clock time per call type using time.perf_counter().
Minimal overhead — stores raw durations in lists, computes stats lazily.

Usage:
    profiler = RPCProfiler(enabled=True)
    with profiler.track("tick"):
        playground.tick(...)
    print(profiler.summary())
"""

from __future__ import annotations

from collections import defaultdict
from contextlib import contextmanager
from time import perf_counter
from typing import Dict, List


class RPCProfiler:
    """Tracks wall-clock timing per named call type."""

    def __init__(self, enabled: bool = True):
        self._enabled = enabled
        self._timings: Dict[str, List[float]] = defaultdict(list)
        self._wall_start = perf_counter()

    @contextmanager
    def track(self, name: str):
        """Context manager that records elapsed time for a named operation."""
        if not self._enabled:
            yield
            return
        start = perf_counter()
        try:
            yield
        finally:
            self._timings[name].append(perf_counter() - start)

    def record(self, name: str, duration: float):
        """Manually record a duration (seconds)."""
        if self._enabled:
            self._timings[name].append(duration)

    def summary(self) -> str:
        """Return a formatted summary table sorted by total time descending."""
        if not self._enabled or not self._timings:
            return ""

        wall = perf_counter() - self._wall_start
        lines = []
        lines.append("=" * 72)
        lines.append("RPC PROFILING SUMMARY")
        lines.append("=" * 72)
        lines.append(f"Wall clock: {wall:.1f}s")
        lines.append("")
        lines.append(
            f"{'Call Type':<25s} {'Count':>6s} {'Total(s)':>9s} "
            f"{'Mean(ms)':>9s} {'P50(ms)':>8s} {'P95(ms)':>8s} "
            f"{'% Wall':>7s}"
        )
        lines.append("-" * 72)

        # Sort by total time descending
        entries = []
        for name, durations in self._timings.items():
            durations_sorted = sorted(durations)
            count = len(durations_sorted)
            total = sum(durations_sorted)
            mean = total / count if count else 0
            p50 = _percentile(durations_sorted, 0.50)
            p95 = _percentile(durations_sorted, 0.95)
            pct = (total / wall * 100) if wall > 0 else 0
            entries.append((name, count, total, mean, p50, p95, pct))

        entries.sort(key=lambda e: e[2], reverse=True)

        for name, count, total, mean, p50, p95, pct in entries:
            lines.append(
                f"{name:<25s} {count:>6d} {total:>9.1f} "
                f"{mean * 1000:>9.1f} {p50 * 1000:>8.1f} {p95 * 1000:>8.1f} "
                f"{pct:>6.1f}%"
            )

        lines.append("=" * 72)
        return "\n".join(lines)

    def summary_dict(self) -> Dict[str, dict]:
        """Return machine-readable stats per call type."""
        result = {}
        for name, durations in self._timings.items():
            ds = sorted(durations)
            count = len(ds)
            total = sum(ds)
            result[name] = {
                "count": count,
                "total_s": total,
                "mean_ms": (total / count * 1000) if count else 0,
                "p50_ms": _percentile(ds, 0.50) * 1000,
                "p95_ms": _percentile(ds, 0.95) * 1000,
                "p99_ms": _percentile(ds, 0.99) * 1000,
            }
        return result


def _percentile(sorted_values: List[float], pct: float) -> float:
    """Compute percentile from a pre-sorted list."""
    if not sorted_values:
        return 0.0
    idx = int(pct * (len(sorted_values) - 1))
    return sorted_values[idx]
