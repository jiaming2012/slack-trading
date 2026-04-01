---
phase: 21-first-strategy-migration-validation
plan: 01
subsystem: strategy
tags: [python, datasource, mean-reversion, signal-extraction, migration]

# Dependency graph
requires:
  - phase: 20-esdb-persistence
    provides: Signal repository infrastructure for live mode
provides:
  - datasources/ma_crossover.py with stateless produce_signals() function
  - MeanReversionStrategyV2 consuming signals from datasource module
  - Datasource-consumption pattern for all future strategy migrations
affects: [21-02, 22-remaining-strategy-migrations]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Datasource module: stateless produce_signals(bar_dict, prev_bar, pdf) -> list[dict]"
    - "V2 strategy: full copy of V1 with only _process_htf_candle() consuming datasource"
    - "Caller manages prev_bar state (datasource is stateless)"

key-files:
  created:
    - src/clients/python/datasources/__init__.py
    - src/clients/python/datasources/ma_crossover.py
    - src/clients/python/strategies/mean_reversion_v2.py
  modified: []

key-decisions:
  - "produce_signals() returns list of dicts (signal_key, bar_dict, pdf_entry) for flexible consumption"
  - "V2 runner function renamed to run_mean_reversion_v2 to avoid collision with V1"

patterns-established:
  - "Datasource extraction: extract signal detection into datasources/<name>.py with produce_signals() function"
  - "Strategy V2 pattern: full copy with only _process_htf_candle() modified to consume datasource"

requirements-completed: [MIG-03]

# Metrics
duration: 2min
completed: 2026-03-31
---

# Phase 21 Plan 01: MA Crossover Datasource & MeanReversionStrategyV2 Summary

**Stateless MA crossover datasource module extracting signal detection from V1, consumed by MeanReversionStrategyV2 with identical behavior**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-31T00:32:00Z
- **Completed:** 2026-03-31T00:34:22Z
- **Tasks:** 2
- **Files created:** 3

## Accomplishments
- Created datasources package with ma_crossover.py extracting signal detection from _process_htf_candle() lines 241-257
- Created MeanReversionStrategyV2 as full copy of V1 with only _process_htf_candle() consuming datasource signals
- Verified prev_bar timing invariant: state update happens AFTER produce_signals() call, matching V1

## Task Commits

Each task was committed atomically:

1. **Task 1: Create datasources/ma_crossover.py module** - `4d76339` (feat)
2. **Task 2: Create MeanReversionStrategyV2 consuming datasource signals** - `747d746` (feat)

## Files Created/Modified
- `src/clients/python/datasources/__init__.py` - Package init for datasources
- `src/clients/python/datasources/ma_crossover.py` - Stateless produce_signals() extracting V1 signal detection
- `src/clients/python/strategies/mean_reversion_v2.py` - V2 strategy consuming datasource signals

## Decisions Made
- produce_signals() returns list of dicts with signal_key, bar_dict, pdf_entry keys for flexible consumption by multiple strategies
- Runner function renamed to run_mean_reversion_v2() to coexist with V1 during migration period
- V2 label set to "mean_reversion_v2" for differentiation in metrics/logging

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is wired and operational.

## Next Phase Readiness
- Datasource module and V2 strategy ready for behavioral diff testing (Plan 21-02)
- Plan 21-02 will create diff test comparing V1 and V2 output on same data
- Pattern established for Phase 22 remaining strategy migrations

## Self-Check: PASSED

All 3 created files verified present. Both task commit hashes (4d76339, 747d746) verified in git log.

---
*Phase: 21-first-strategy-migration-validation*
*Completed: 2026-03-31*
