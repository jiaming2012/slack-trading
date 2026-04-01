---
phase: 18-signal-repository-sim-mode
plan: 01
subsystem: backtester
tags: [signal-repository, in-memory, clock-gated, tdd, interface]

# Dependency graph
requires:
  - phase: 17-signal-foundation
    provides: "TradeSignal struct, SignalName registry in eventmodels"
provides:
  - "ISignalRepository interface (Write, ReadPending, GetAll)"
  - "InMemorySignalRepository with sorted-slice + cursor clock-gated delivery"
affects: [18-02-PLAN, 19-signal-rpc, 20-esdb-persistence]

# Tech tracking
tech-stack:
  added: []
  patterns: ["sorted-slice with cursor for clock-gated delivery", "binary search insertion for timestamp ordering"]

key-files:
  created:
    - src/go/backtester-api/models/signal_repository_interface.go
    - src/go/backtester-api/models/signal_repository_memory.go
    - src/go/backtester-api/models/signal_repository_memory_test.go
  modified: []

key-decisions:
  - "Single global signal stream (no per-symbol partitioning) per D-01 design decision"
  - "Cursor adjustment on pre-cursor insertion to prevent skipping signals"

patterns-established:
  - "ISignalRepository: repository interface pattern for signal storage abstraction"
  - "Clock-gated delivery: ReadPending(upTo) prevents lookahead bias in backtesting"

requirements-completed: [REPO-01]

# Metrics
duration: 2min
completed: 2026-03-30
---

# Phase 18 Plan 01: Signal Repository & Sim Mode Summary

**ISignalRepository interface and InMemorySignalRepository with clock-gated delivery using sorted-slice cursor pattern, verified by 6 TDD test cases including race detection**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-30T21:55:30Z
- **Completed:** 2026-03-30T21:57:49Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- ISignalRepository interface with Write, ReadPending, GetAll methods following project I-prefix convention
- InMemorySignalRepository using binary-search insertion and cursor-based clock-gated delivery
- Comprehensive test suite: clock-gated delivery, out-of-order writes, partial consumption, nil guard, empty result, 100-goroutine concurrent writes

## Task Commits

Each task was committed atomically:

1. **Task 1: Define ISignalRepository interface** - `988e7b8` (feat)
2. **Task 2: Implement InMemorySignalRepository with tests** - `b08a723` (feat)

_TDD flow: RED (tests fail - undefined symbols) then GREEN (implementation passes all tests with race detector)_

## Files Created/Modified
- `src/go/backtester-api/models/signal_repository_interface.go` - ISignalRepository interface (Write, ReadPending, GetAll)
- `src/go/backtester-api/models/signal_repository_memory.go` - InMemorySignalRepository with sorted slice, cursor, sync.Mutex
- `src/go/backtester-api/models/signal_repository_memory_test.go` - 6 test functions covering all acceptance criteria

## Decisions Made
- Single global signal stream per D-01: no symbol parameter on ReadPending, symbol filtering is client-side
- Cursor adjustment when inserting before current position prevents signal skipping on out-of-order writes

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ISignalRepository interface ready for Playground integration in 18-02
- InMemorySignalRepository ready to be injected into Playground struct
- Interface supports future ESDB implementation (Phase 20) via same contract

## Self-Check: PASSED

- All 3 source files exist
- All 2 task commits verified (988e7b8, b08a723)
- Tests pass with race detector

---
*Phase: 18-signal-repository-sim-mode*
*Completed: 2026-03-30*
