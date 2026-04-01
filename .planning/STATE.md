---
gsd_state_version: 1.0
milestone: v3.0
milestone_name: TradeSignal Framework
status: executing
stopped_at: Completed quick task 260401-fij
last_updated: "2026-04-01T15:20:05.776Z"
last_activity: 2026-04-01
progress:
  total_phases: 10
  completed_phases: 8
  total_plans: 24
  completed_plans: 21
  percent: 29
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-30)

**Core value:** When a live simulation is running, the operator can always tell whether the system is alive and what it's doing -- even when no trades are being placed.
**Current focus:** Phase 26 — python-datasource-wiring-observability

## Current Position

Phase: 26
Plan: Not started
Status: Executing Phase 26
Last activity: 2026-04-01

Progress: [███░░░░░░░] 29%

## Performance Metrics

**Velocity:**

- Total plans completed: 18 (v1.0) + 4 (v1.1) + 8 (v2.0) = 30
- Average duration: ~4min
- Total execution time: ~2 hours

**Recent Trend (v2.0):**

- Last 3 plans: Phase 15 P01 (4m4s), Phase 16 P01 (4min), Phase 16 P02 (3min)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap v3.0]: 7 phases derived from 18 requirements following research-recommended build order
- [Roadmap v3.0]: Per-symbol ESDB streams (signals-AAPL) over per-signal-name streams (research recommendation)
- [Roadmap v3.0]: MIG-03 (diff testing) in Phase 21 to prove pattern before bulk migration in Phase 22
- [Roadmap v3.0]: signal_id optional during migration phases, required after Phase 22 completion
- [Research]: WriteSignal via Twirp RPC (single-writer principle) over direct Python-to-ESDB writes
- [Phase 17]: Preserved existing SignalName constants and appended new ones for backward compatibility
- [Phase 17]: TradeSignal Attributes: map[string]interface{} in Go, map<string,string> in proto per D-02 design
- [Phase 17]: SignalID set post-construction in commitOrderRecord to avoid modifying PopulateOrderRecord 20+ param signature
- [Phase 18-01]: Single global signal stream (no per-symbol partitioning) per D-01 design
- [Phase 18-01]: Cursor adjustment on pre-cursor insertion prevents signal skipping
- [Phase 18-02]: Attributes map[string]interface{} converted to map[string]string via fmt.Sprintf for proto
- [Phase 18-02]: ESDB persistence uses existing pub/sub pattern (PublishAndSaveEvent) for architectural consistency
- [Phase 20]: Removed NewTradeSignalStreamName per D-01 single global stream decision
- [Phase 20]: ESDBSignalRepository reads full ESDB stream on each call (acceptable at <1000 signals/day)
- [Phase 20]: Live and reconcile playgrounds both get ESDBSignalRepository; only simulator gets InMemory
- [Phase 20]: ESDB integration test uses TestContainers with EsdbProducer.Start() for full stack validation
- [Phase 21]: produce_signals() returns list of dicts for flexible consumption by multiple strategies
- [Phase 21]: Normalized group_id in order call comparisons for deterministic behavioral diff testing
- [Phase 22]: on_signal() added as default no-op (not abstract) for backward compat
- [Phase 22]: Compare at _try_create_group boundary for credit spread diff tests (order placement depends on identical options ladder RPC)
- [Phase 23]: 5m staleness threshold for datasource heartbeat (10 missed 30s emissions)
- [Phase 23]: Signal date filtering uses inclusive start, exclusive stop+1day for full coverage
- [Phase 23]: Replay mode requires ESDB producer; returns error if not configured
- [Phase 23]: 10 signals across 45min window for dual-run replay test coverage
- [Phase 24]: Used google.protobuf.timestamp_pb2.Timestamp.FromDatetime for datetime conversion in Python signal RPC wrappers
- [Phase 25]: Callable-based datasource pattern: feature_vector_fn callable encapsulates stateful supertrend lookback for covered call and wheel signals
- [Phase 25]: Signal callback pattern: _signal_callback on playground set by trading engine for decoupled on_signal() dispatch
- [Phase 25]: All 6 V1 strategy files moved to deprecated/, V2-only codebase with updated imports

### Pending Todos

None yet.

### Blockers/Concerns

- [Research]: Verify esdbclient + grpcio version compatibility in grodt conda env before Phase 20
- [Research]: Stream naming decision (per-symbol vs per-signal-name) needs spike validation in Phase 17
- [Research]: Attributes type -- map[string]string for proto compat vs map[string]interface{} for flexibility

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260401-cgf | Add self-contained OTel collector to E2E test harness for metrics/traces verification | 2026-04-01 | 480bfea | [260401-cgf-add-self-contained-otel-collector-to-e2e](./quick/260401-cgf-add-self-contained-otel-collector-to-e2e/) |
| 260401-fij | Fix 3 v3.0 audit gaps: signal_id param, Grafana label mismatch, diff test failure | 2026-04-01 | 28981cb | [260401-fij-fix-3-v3-0-audit-gaps-signal-id-param-gr](./quick/260401-fij-fix-3-v3-0-audit-gaps-signal-id-param-gr/) |
| 260401-gig | Share globalSignalRepo with sim playgrounds and wire write_signal() in MeanReversionV2 | 2026-04-01 | 4754944 | [260401-gig-share-globalsignalrepo-with-sim-playgrou](./quick/260401-gig-share-globalsignalrepo-with-sim-playgrou/) |

## Session Continuity

Last session: 2026-04-01T15:57:04Z
Stopped at: Completed quick task 260401-gig
Resume file: .planning/phases/26-python-datasource-wiring-observability/26-CONTEXT.md
