# Phase 23: Replay & Telemetry - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Add signal replay from ESDB for reproducible simulations, OTel signal telemetry with Grafana panels, datasource heartbeat monitoring, and alerting on datasource liveness. Final phase of v3.0.

Requirements: REPLAY-01, REPLAY-02, OBS-01, OBS-02

</domain>

<decisions>
## Implementation Decisions

### D-01: Replay mode via CLI flag
- `--replay-signals trade-signals` flag on demo scripts
- Preloads signals from ESDB into InMemorySignalRepository (sorted by timestamp)
- Clock-gated delivery works as normal — no strategy code changes
- Strategy doesn't know whether signals came from live datasource or ESDB replay

### D-02: Alerting — datasource liveness heartbeat
- Alert is about **datasource script liveness**, NOT signal production frequency
- If the datasource script stops running (heartbeat goes stale), fire alert
- Same pattern as existing Python strategy heartbeat stale alert
- It doesn't matter when signals are produced, as long as the script is checking for new signals

### D-03: Replay integration test — dual-run comparison
- Run same strategy twice on identical playground data:
  1. Once with in-memory datasource (normal sim mode)
  2. Once with ESDB replay (--replay-signals mode)
- Compare final metrics (P&L, trades, win rate) — must match exactly
- Reuses Phase 21 behavioral diff test pattern
- Requires ESDB TestContainer

### D-04: Grafana signal panels — add to existing dashboard
- Add 2-3 panels to `grodt-live-simulation.json`:
  - Signal Production Count (by name)
  - Signal Consumption Count (by strategy)
  - Datasource Heartbeat status
- No new dashboard — keep everything in one place

### D-05: Datasource heartbeat — new DatasourceHeartbeat class
- New class `DatasourceHeartbeat` (not reusing StrategyHeartbeat)
- Datasource-specific metrics: signals_checked, last_check_time, datasource_name
- OTel gauge for heartbeat + structured log
- Grafana alert: heartbeat stale for X minutes → fire

### Claude's Discretion
- DatasourceHeartbeat class internals and OTel metric names
- Exact Grafana panel queries and layout
- How ESDB replay preloads signals (batch read on startup vs streaming)
- Alert threshold timing (how many minutes of silence before alerting)

</decisions>

<canonical_refs>
## Canonical References

### Signal Infrastructure (Phases 17-22)
- `src/go/backtester-api/models/signal_repository_esdb.go` — ESDBSignalRepository (read from ESDB)
- `src/go/backtester-api/models/signal_repository_memory.go` — InMemorySignalRepository (preload target)
- `src/go/backtester-api/router/grpc.go` — WriteSignal/GetSignals RPCs, signal delivery in NextTick

### Existing Heartbeat Pattern
- `src/clients/python/engine/heartbeat.py` — StrategyHeartbeat (OTel gauge + structured log pattern)

### Grafana Dashboards
- `observability/dashboards/grodt-live-simulation.json` — add signal panels here
- `observability/alerting/alerting.yaml` — existing alert rules (heartbeat stale pattern)

### Testing
- `src/clients/python/tests/test_mean_reversion_diff.py` — behavioral diff test pattern for replay comparison
- `integration_testing/esdb_signal_repository_test.go` — ESDB TestContainer setup

### Demo Scripts
- `src/clients/python/demos/demo_mean_reversion_v2.py` — add --replay-signals flag here

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- ESDBSignalRepository.ReadPending() already reads from ESDB with time filtering
- InMemorySignalRepository can be preloaded with signals from any source
- Grafana dashboard provisioning via docker-compose volume mounts
- Alerting YAML follows existing heartbeat stale pattern

### Integration Points
- Demo scripts: add `--replay-signals` argparse flag
- `DatasourceHeartbeat` in new file: `src/clients/python/engine/datasource_heartbeat.py`
- `grodt-live-simulation.json`: add signal panels after existing panels
- `alerting.yaml`: add datasource heartbeat stale rule

</code_context>

<specifics>
## Specific Ideas

- Replay preload: read all signals from ESDB, filter by time window matching playground start/end dates, bulk-load into InMemorySignalRepository
- DatasourceHeartbeat should emit `grodt.datasource.heartbeat` gauge (distinct from `grodt.strategy.heartbeat`)
- Signal production panel: `rate(grodt_signals_produced_total[5m])` by signal_name

</specifics>

<deferred>
## Deferred Ideas

- Composite signal framework — v3.1
- Signal backtesting optimizer — future
- Per-signal-type alerting thresholds — future enhancement

</deferred>

---

*Phase: 23-replay-telemetry*
*Context gathered: 2026-03-31*
