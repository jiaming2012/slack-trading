# Phase 23: Replay & Telemetry - Discussion Log

> **Audit trail only.**

**Date:** 2026-03-31
**Phase:** 23-replay-telemetry
**Areas discussed:** Replay mode, Signal alerting, Integration test, Grafana dashboard, Datasource heartbeat

---

## Replay Mode

| Option | Description | Selected |
|--------|-------------|----------|
| CLI flag --replay-signals | Preload from ESDB into InMemory | ✓ |
| Config-based mode | Env var SIGNAL_SOURCE | |

**User's choice:** CLI flag (Recommended)

## Signal Alerting

**User's key insight:** "The SignalsProduced counter should just send a heartbeat message to grafana with a timestamp so that we can check that the script is running. It doesn't matter when the signal is produced, as long as the script is checking for new signals."
→ Alert on **datasource script liveness**, not signal frequency.

## Replay Integration Test

| Option | Description | Selected |
|--------|-------------|----------|
| Dual-run comparison | Run in-memory vs ESDB replay, compare metrics | ✓ |
| Snapshot comparison | Compare against fixture | |

## Grafana Dashboard

| Option | Description | Selected |
|--------|-------------|----------|
| Add to existing dashboard | 2-3 panels on grodt-live-simulation | ✓ |
| New dedicated dashboard | Separate grodt-signals | |

## Datasource Heartbeat

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse StrategyHeartbeat | Same class, different name | |
| New DatasourceHeartbeat | Datasource-specific metrics | ✓ |

## Claude's Discretion
- DatasourceHeartbeat internals, OTel metrics, Grafana queries, ESDB preload mechanism, alert thresholds
