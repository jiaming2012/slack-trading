---
phase: 20-esdb-persistence
plan: 01
subsystem: database
tags: [esdb, eventstore, signal-repository, trade-signals]

requires:
  - phase: 18-signal-repository-sim-mode
    provides: ISignalRepository interface, InMemorySignalRepository, TradeSignal struct
provides:
  - ESDBSignalRepository implementing ISignalRepository with ESDB write-through
  - Global trade-signals stream naming (single stream, not per-symbol)
affects: [20-02-PLAN, server-wiring, live-mode]

tech-stack:
  added: []
  patterns: [ESDB write-through repository, Go-side filtering of ESDB events]

key-files:
  created:
    - src/go/backtester-api/models/signal_repository_esdb.go
    - src/go/backtester-api/models/signal_repository_esdb_test.go
  modified:
    - src/go/eventmodels/trade_signal.go
    - src/go/eventmodels/stream_names.go

key-decisions:
  - "Removed NewTradeSignalStreamName per D-01 single global stream decision"
  - "ESDBSignalRepository reads full stream on each call (acceptable at <1000 signals/day volume)"

patterns-established:
  - "ESDB repository pattern: write-through via EsdbProducer.Save, read via eventservices.FetchAll"

requirements-completed: [REPO-02, QUERY-01]

duration: 2min
completed: 2026-03-30
---

# Phase 20 Plan 01: ESDB Signal Persistence Summary

**ESDBSignalRepository with write-through ESDB persistence and global trade-signals stream naming fix**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-30T23:54:51Z
- **Completed:** 2026-03-30T23:57:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Fixed TradeSignal to use single global "trade-signals" stream instead of per-symbol streams (D-01 compliance)
- Removed NewTradeSignalStreamName function that created per-symbol streams
- Created ESDBSignalRepository implementing ISignalRepository with ESDB write-through and filtered reads
- Added unit tests verifying interface compliance, nil-safety, and global stream naming

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix TradeSignal stream name to global and implement ESDBSignalRepository** - `16fba80` (feat)
2. **Task 2: Unit tests for ESDBSignalRepository** - `0381d2d` (test)

## Files Created/Modified
- `src/go/backtester-api/models/signal_repository_esdb.go` - ESDBSignalRepository with Write, ReadPending, GetAll via ESDB
- `src/go/backtester-api/models/signal_repository_esdb_test.go` - Interface check, nil-signal guard, global stream name tests
- `src/go/eventmodels/trade_signal.go` - Changed to use TradeSignalStream constant directly
- `src/go/eventmodels/stream_names.go` - Removed NewTradeSignalStreamName function

## Decisions Made
- Removed NewTradeSignalStreamName per D-01 single global stream decision -- all signals share one stream
- ESDBSignalRepository reads full stream on each call -- acceptable at expected volume (<1000 signals/day)
- ReadPending logs errors and returns empty slice rather than propagating errors (matches ISignalRepository contract)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ESDBSignalRepository ready for server wiring in Plan 02
- All unit tests pass, full project builds cleanly
- Integration tests with live ESDB belong in Plan 02

## Self-Check: PASSED

- [x] signal_repository_esdb.go exists
- [x] signal_repository_esdb_test.go exists
- [x] Commit 16fba80 found
- [x] Commit 0381d2d found

---
*Phase: 20-esdb-persistence*
*Completed: 2026-03-30*
