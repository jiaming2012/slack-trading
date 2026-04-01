---
phase: 25-complete-strategy-migrations
plan: 03
subsystem: strategy
tags: [python, migration, signals, callback, deprecated]

# Dependency graph
requires:
  - phase: 25-01
    provides: CoveredCall V2, Wheel V2 strategies
  - phase: 25-02
    provides: PDFWheel V2 strategy
provides:
  - on_signal() callback wiring in client.py tick() for TradeSignal delivery
  - All V1 strategy files archived in deprecated/
  - V2-only strategy codebase with clean imports
affects: [26-loki-grafana, future-strategy-development]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Signal callback pattern: playground._signal_callback = strategy.on_signal wired by trading engine"
    - "V1 files in deprecated/ with imports updated across entire codebase"

key-files:
  created: []
  modified:
    - src/clients/python/engine/client.py
    - src/clients/python/engine/trading_engine.py
    - src/clients/python/deprecated/covered_call.py
    - src/clients/python/deprecated/wheel.py
    - src/clients/python/deprecated/pdf_wheel.py
    - src/clients/python/deprecated/mean_reversion.py
    - src/clients/python/deprecated/options_mean_reversion.py
    - src/clients/python/deprecated/credit_spread.py

key-decisions:
  - "Signal callback pattern: _signal_callback attribute on playground set by trading engine, keeping client.py decoupled from strategy type"
  - "Signals dispatched DURING tick processing (before buffer append) for time-synchronization"

patterns-established:
  - "Signal callback: playground._signal_callback = strategy.on_signal set in run_strategy() before tick loop"
  - "V1 strategies live in deprecated/ package, V2 strategies in strategies/ package"

requirements-completed: [MIG-01, MIG-02]

# Metrics
duration: 3min
completed: 2026-03-31
---

# Phase 25 Plan 03: Wire on_signal() and Deprecate V1 Strategies Summary

**on_signal() callback wired in client.py tick() for TradeSignal delivery; all 6 V1 strategy files moved to deprecated/ with imports updated across 25 files**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-31T11:36:50Z
- **Completed:** 2026-03-31T11:39:45Z
- **Tasks:** 2
- **Files modified:** 27

## Accomplishments
- Wired on_signal() delivery in client.py tick() using callback pattern -- signals dispatched during tick processing for time-synchronization
- Trading engine sets playground._signal_callback = strategy.on_signal before tick loop starts
- Moved all 6 V1 strategy files (covered_call, wheel, pdf_wheel, mean_reversion, options_mean_reversion, credit_spread) from strategies/ to deprecated/
- Updated imports in 20 files across V2 strategies, tests, and demos

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire on_signal() in client.py tick()** - `f1f06f3` (feat)
2. **Task 2: Move V1 strategies to deprecated/ and verify V2-only imports** - `8d201cf` (refactor)

## Files Created/Modified
- `src/clients/python/engine/client.py` - Added _signal_callback attribute and signal dispatch in tick()
- `src/clients/python/engine/trading_engine.py` - Wire callback before tick loop
- `src/clients/python/deprecated/covered_call.py` - Moved from strategies/
- `src/clients/python/deprecated/wheel.py` - Moved from strategies/, updated internal imports
- `src/clients/python/deprecated/pdf_wheel.py` - Moved from strategies/, updated internal imports
- `src/clients/python/deprecated/mean_reversion.py` - Moved from strategies/
- `src/clients/python/deprecated/options_mean_reversion.py` - Moved from strategies/
- `src/clients/python/deprecated/credit_spread.py` - Moved from strategies/
- `src/clients/python/strategies/covered_call_v2.py` - Updated V1 imports to deprecated.*
- `src/clients/python/strategies/wheel_v2.py` - Updated V1 imports to deprecated.*
- `src/clients/python/strategies/pdf_wheel_v2.py` - Updated V1 imports to deprecated.*
- `src/clients/python/tests/*_diff.py` - Updated V1 imports to deprecated.* (6 diff test files)
- `src/clients/python/tests/*_strategy.py` - Updated V1 imports to deprecated.* (4 unit test files)
- `src/clients/python/demos/*.py` - Updated V1 imports to deprecated.* (6 demo files)

## Decisions Made
- Signal callback pattern: _signal_callback attribute on playground set by trading engine keeps client.py decoupled from strategy type
- Signals dispatched DURING tick processing (before buffer append) for time-synchronization with the tick that delivered them

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All V2 strategies are the primary strategy implementations
- on_signal() callback is wired and ready for TradeSignal consumption
- MIG-01 (all strategies migrated) and MIG-02 (originals deprecated) are complete
- Phase 25 is fully complete

---
*Phase: 25-complete-strategy-migrations*
*Completed: 2026-03-31*
