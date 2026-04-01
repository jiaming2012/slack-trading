---
phase: 24-signal-rpc-endpoints
plan: 01
subsystem: api
tags: [twirp, protobuf, rpc, signals, otel]

# Dependency graph
requires:
  - phase: 20-esdb-signal-repository
    provides: ISignalRepository, ESDBSignalRepository, InMemorySignalRepository, TradeSignal
  - phase: 17-trade-signal-struct
    provides: TradeSignal struct, SignalName type with Validate()
provides:
  - WriteSignal RPC endpoint for producing signals via Twirp
  - GetSignals RPC endpoint for querying global signal repository
  - GetProcessedSignals RPC endpoint for querying playground-consumed signals
  - globalSignalRepo wiring in Server struct and cmd/main.go
affects: [24-02-PLAN, python-client-integration, grafana-dashboards]

# Tech tracking
tech-stack:
  added: []
  patterns: [filterSignals shared helper for signal query endpoints, globalSignalRepo DI through Server struct]

key-files:
  created:
    - src/go/backtester-api/router/grpc_signal_test.go
  modified:
    - src/go/playground.proto
    - src/go/backtester-api/router/grpc.go
    - src/go/backtester-api/rpc/twirp.go
    - cmd/main.go
    - src/go/playground/playground.pb.go
    - src/go/playground/playground.twirp.go
    - src/clients/python/rpc/playground_pb2.py
    - src/clients/python/rpc/playground_twirp.py

key-decisions:
  - "Used FetchPlayground (not GetPlayground) for playground lookup in GetProcessedSignals -- matches existing DatabaseService API"
  - "ESDBSignalRepository selected when esdbProducer != nil in cmd/main.go, preserving DS-02 stream topology"

patterns-established:
  - "filterSignals helper: shared signal filtering logic for GetSignals and GetProcessedSignals"
  - "globalSignalRepo DI: passed through SetupTwirpServer -> NewServer -> Server struct"

requirements-completed: [DS-01, DS-02, RPC-01]

# Metrics
duration: 6min
completed: 2026-03-31
---

# Phase 24 Plan 01: Signal RPC Endpoints Summary

**WriteSignal, GetSignals, GetProcessedSignals Twirp RPCs with proto stubs, OTel telemetry, globalSignalRepo wiring, and 10 unit tests**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-31T10:50:36Z
- **Completed:** 2026-03-31T10:56:30Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Three new Twirp RPC endpoints defined in playground.proto and implemented in grpc.go
- Server struct extended with globalSignalRepo (ISignalRepository) with full DI wiring
- OTel SignalsGenerated counter incremented on each WriteSignal call with signal_name and symbol attributes
- cmd/main.go creates ESDBSignalRepository for live mode, InMemorySignalRepository otherwise
- 10 unit tests covering happy paths, validation errors, and filter combinations
- Go and Python proto stubs regenerated

## Task Commits

Each task was committed atomically:

1. **Task 1: Proto definitions and Go handler implementations** - `22e0b17` (feat)
2. **Task 2: Unit tests for signal RPC handlers** - `f536a7e` (test)

## Files Created/Modified
- `src/go/playground.proto` - Added WriteSignal, GetSignals, GetProcessedSignals RPCs and request/response messages
- `src/go/backtester-api/router/grpc.go` - Added globalSignalRepo to Server, filterSignals helper, WriteSignal/GetSignals/GetProcessedSignals handlers
- `src/go/backtester-api/rpc/twirp.go` - Added globalSignalRepo parameter to SetupTwirpServer
- `cmd/main.go` - Creates ESDBSignalRepository or InMemorySignalRepository, passes to SetupTwirpServer
- `src/go/backtester-api/router/grpc_signal_test.go` - 10 unit tests for signal RPC handlers
- `src/go/playground/playground.pb.go` - Regenerated proto stubs
- `src/go/playground/playground.twirp.go` - Regenerated Twirp server stubs
- `src/clients/python/rpc/playground_pb2.py` - Regenerated Python proto stubs
- `src/clients/python/rpc/playground_twirp.py` - Regenerated Python Twirp client stubs

## Decisions Made
- Used `FetchPlayground` (not `GetPlayground`) for playground lookup in GetProcessedSignals -- matches existing DatabaseService API
- ESDBSignalRepository selected when esdbProducer != nil in cmd/main.go, preserving DS-02 stream topology from Phase 20

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Merged feature/trade-signals branch into worktree**
- **Found during:** Task 1 (reading source files)
- **Issue:** Worktree was based on an older branch without Phase 17-23 prerequisite code (TradeSignal, ISignalRepository, telemetry metrics)
- **Fix:** Merged feature/trade-signals into the worktree to get prerequisite types and interfaces
- **Files modified:** Entire working tree updated
- **Verification:** All referenced types and interfaces available after merge

**2. [Rule 3 - Blocking] Ran protoc directly instead of task gen:proto**
- **Found during:** Task 1 (proto stub generation)
- **Issue:** `task gen:proto` requires Python venv activation which was missing in worktree
- **Fix:** Ran protoc command directly with correct paths and manually moved output files
- **Files modified:** Generated proto stubs
- **Verification:** `go build ./cmd/main.go` and `go build ./src/go/...` both succeed

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both auto-fixes necessary to unblock execution. No scope creep.

## Issues Encountered
None beyond the deviations noted above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three signal RPCs are functional and tested
- Python proto stubs regenerated with new RPC methods
- Ready for Plan 02 (Python client integration or additional signal features)

## Self-Check: PASSED

---
*Phase: 24-signal-rpc-endpoints*
*Completed: 2026-03-31*
