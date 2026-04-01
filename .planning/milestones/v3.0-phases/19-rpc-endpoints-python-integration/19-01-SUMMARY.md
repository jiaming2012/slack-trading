---
phase: 19-rpc-endpoints-python-integration
plan: 01
subsystem: api
tags: [twirp, rpc, protobuf, otel, signals]

requires:
  - phase: 18-signal-repository-sim-mode
    provides: ISignalRepository interface, InMemorySignalRepository, TradeSignal struct, SignalName registry
provides:
  - WriteSignal RPC endpoint for global signal production
  - GetSignals RPC endpoint with name/symbol/time range filters
  - GetProcessedSignals RPC endpoint for per-playground consumed signals
  - SignalsProduced OTel counter for signal production telemetry
  - Python proto stubs for all three new RPCs
affects: [19-02, 20-01, phase-20]

tech-stack:
  added: []
  patterns: [filterSignals helper for reusable signal filtering, twirp.InvalidArgumentError for validation]

key-files:
  created:
    - src/go/backtester-api/router/grpc_signal_handlers_test.go
  modified:
    - src/go/playground.proto
    - src/go/playground/playground.pb.go
    - src/go/playground/playground.twirp.go
    - src/clients/python/rpc/playground_pb2.py
    - src/clients/python/rpc/playground_twirp.py
    - src/go/backtester-api/router/grpc.go
    - src/go/telemetry/metrics.go

key-decisions:
  - "Used twirp.InvalidArgumentError for validation errors (cleaner than fmt.Errorf for client-facing errors)"
  - "Extracted filterSignals helper shared by GetSignals and GetProcessedSignals"
  - "globalSignalRepo initialized inline in NewServer (no constructor param change needed)"

patterns-established:
  - "filterSignals: reusable signal filtering by name, symbol, time range with proto conversion"
  - "twirp error codes: InvalidArgumentError for validation, NotFoundError for missing resources"

requirements-completed: [DS-01, DS-02, RPC-01]

duration: 5min
completed: 2026-03-30
---

# Phase 19 Plan 01: RPC Endpoints & Python Integration Summary

**WriteSignal/GetSignals/GetProcessedSignals Twirp RPCs with global signal repository, OTel counter, and 7 unit tests**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-30T22:58:23Z
- **Completed:** 2026-03-30T23:03:12Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Three new Twirp RPC endpoints added to PlaygroundService: WriteSignal (global signal production), GetSignals (filtered global query), GetProcessedSignals (per-playground consumed signals)
- Proto messages and stubs regenerated for both Go and Python
- Server struct extended with globalSignalRepo (InMemorySignalRepository) for global signal storage
- SignalsProduced OTel counter registered and incremented on each WriteSignal call
- 7 unit tests covering valid/invalid inputs, no-filter queries, name/symbol/time-range filtering, and error paths

## Task Commits

Each task was committed atomically:

1. **Task 1: Proto messages, codegen, Server struct, OTel counters** - `62ead2b` (feat)
2. **Task 2: Unit tests for signal RPC handlers** - `3053e15` (test)

## Files Created/Modified
- `src/go/playground.proto` - Added WriteSignal, GetSignals, GetProcessedSignals RPCs and 6 new messages
- `src/go/playground/playground.pb.go` - Regenerated Go protobuf stubs
- `src/go/playground/playground.twirp.go` - Regenerated Go Twirp service stubs
- `src/clients/python/rpc/playground_pb2.py` - Regenerated Python protobuf stubs
- `src/clients/python/rpc/playground_twirp.py` - Regenerated Python Twirp client stubs
- `src/go/backtester-api/router/grpc.go` - Added globalSignalRepo field, WriteSignal/GetSignals/GetProcessedSignals handlers, filterSignals helper
- `src/go/telemetry/metrics.go` - Added SignalsProduced counter
- `src/go/backtester-api/router/grpc_signal_handlers_test.go` - 7 unit tests for new handlers

## Decisions Made
- Used `twirp.InvalidArgumentError` for validation errors instead of generic `fmt.Errorf` -- provides proper Twirp error codes to clients
- Extracted `filterSignals` helper function shared by GetSignals and GetProcessedSignals to avoid code duplication
- Initialized `globalSignalRepo` inline in `NewServer` rather than adding a constructor parameter -- avoids breaking the existing call site in `rpc/twirp.go`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Proto generation via `task gen:proto` failed because the Python venv doesn't exist in the worktree. Ran protoc commands directly for both Go and Python stubs.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All three RPC endpoints compile and pass tests, ready for Python client wrappers in Plan 19-02
- Python proto stubs include WriteSignal/GetSignals/GetProcessedSignals for client integration
- globalSignalRepo is in-memory only; Phase 20 will add ESDB persistence

---
*Phase: 19-rpc-endpoints-python-integration*
*Completed: 2026-03-30*
