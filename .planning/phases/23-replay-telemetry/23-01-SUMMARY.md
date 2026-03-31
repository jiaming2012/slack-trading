---
phase: 23-replay-telemetry
plan: 01
subsystem: api
tags: [protobuf, esdb, replay, telemetry, otel, signals]

# Dependency graph
requires:
  - phase: 20-signal-repositories
    provides: "ISignalRepository interface, ESDBSignalRepository, InMemorySignalRepository"
  - phase: 18-signal-plumbing
    provides: "TradeSignal struct, signal delivery in simulateTick via ReadPending"
provides:
  - "replay_signal_stream proto field on CreatePolygonPlaygroundRequest"
  - "ESDB signal preload into InMemorySignalRepository for replay mode"
  - "SignalsConsumed OTel counter (grodt.signals.consumed)"
  - "--replay-signals CLI flag on demo_mean_reversion_v2.py"
affects: [23-02, 23-03, grafana-dashboards]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replay pattern: ESDB read-all -> date filter -> InMemory bulk load"
    - "Telemetry emission after ReadPending for signal consumption tracking"

key-files:
  created: []
  modified:
    - src/go/playground.proto
    - src/go/playground/playground.pb.go
    - src/go/playground/playground.twirp.go
    - src/clients/python/rpc/playground_pb2.py
    - src/go/backtester-api/router/grpc.go
    - src/go/telemetry/metrics.go
    - src/go/backtester-api/models/playground.go
    - src/clients/python/demos/demo_mean_reversion_v2.py

key-decisions:
  - "Signal date filtering uses inclusive start, exclusive stop+1day for full coverage"
  - "Replay requires ESDB producer -- returns error if not configured"

patterns-established:
  - "Replay preload: temporary ESDBSignalRepository.GetAll() -> filter -> InMemorySignalRepository.Write() loop"

requirements-completed: [REPLAY-01, OBS-01]

# Metrics
duration: 3min
completed: 2026-03-31
---

# Phase 23 Plan 01: ESDB Signal Replay and Consumption Telemetry Summary

**ESDB signal replay via proto field with date-filtered preload into InMemorySignalRepository, plus grodt.signals.consumed OTel counter**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-31T03:22:49Z
- **Completed:** 2026-03-31T03:25:49Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Added `replay_signal_stream` optional field to CreatePolygonPlaygroundRequest proto (field 8)
- Implemented ESDB signal preload in CreatePlayground: reads all signals, filters by playground date range, loads into InMemorySignalRepository
- Added SignalsConsumed OTel counter emitted after each ReadPending delivery in simulateTick
- Added `--replay-signals` CLI flag to demo_mean_reversion_v2.py passing stream name through to RPC

## Task Commits

Each task was committed atomically:

1. **Task 1: Proto field + Go replay preload + signal consumption counter** - `f2a8e8a` (feat)
2. **Task 2: Python CLI flag and client wiring** - `76c5075` (feat)

## Files Created/Modified
- `src/go/playground.proto` - Added optional string replay_signal_stream = 8 to CreatePolygonPlaygroundRequest
- `src/go/playground/playground.pb.go` - Regenerated Go protobuf stubs
- `src/go/playground/playground.twirp.go` - Regenerated Go Twirp stubs
- `src/clients/python/rpc/playground_pb2.py` - Regenerated Python protobuf stubs
- `src/go/backtester-api/router/grpc.go` - Replay preload logic in CreatePlayground handler
- `src/go/telemetry/metrics.go` - SignalsConsumed counter (grodt.signals.consumed)
- `src/go/backtester-api/models/playground.go` - Signal consumption telemetry emission after ReadPending
- `src/clients/python/demos/demo_mean_reversion_v2.py` - --replay-signals argparse flag

## Decisions Made
- Signal date filtering uses `[startDate, stopDate+1day)` range to include all signals on the stop date
- Replay mode returns an error if ESDB producer is nil (replay requires ESDB connectivity)
- SignalsConsumed telemetry includes signal_name and symbol attributes for per-signal-type filtering

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `task gen:proto` failed due to missing Python virtualenv at `src/clients/python/env/` -- used direct `protoc` commands instead (Go stubs from `src/go/`, Python stubs via `protoc --python_out --twirpy_out`)

## Known Stubs

None -- all data paths are fully wired.

## Next Phase Readiness
- Replay proto field and preload logic ready for integration testing (Phase 23 Plan 02)
- SignalsConsumed counter ready for Grafana dashboard integration
- Python demo can be invoked with `--replay-signals trade-signals` once ESDB has persisted signals

---
*Phase: 23-replay-telemetry*
*Completed: 2026-03-31*
