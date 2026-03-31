---
phase: 22-remaining-strategy-migrations
plan: 03
subsystem: strategies
tags: [covered-call, wheel, supertrend, datasource, v2-migration, diff-test]

# Dependency graph
requires:
  - phase: 21-first-strategy-migration-validation
    provides: "Proven datasource extraction + V2 + diff test pattern"
provides:
  - "OptionsStrategyBasicV2 (covered call V2) with datasource delegation"
  - "WheelStrategyV2 extending CoveredCallV2 with put signal datasource"
  - "covered_call_signals and wheel_signals datasource modules"
  - "8 behavioral diff tests proving V1/V2 equivalence"
affects: [22-04-PLAN, 22-05-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Callable-based datasource for state-dependent signal extraction (feature_vector_fn)"
    - "Shared type imports from V1 to avoid duplication (OptionContract, CloseSignalV2, etc.)"

key-files:
  created:
    - src/clients/python/datasources/covered_call_signals.py
    - src/clients/python/datasources/wheel_signals.py
    - src/clients/python/strategies/covered_call_v2.py
    - src/clients/python/strategies/wheel_v2.py
    - src/clients/python/tests/test_covered_call_diff.py
    - src/clients/python/tests/test_wheel_diff.py
  modified:
    - src/clients/python/datasources/__init__.py

key-decisions:
  - "Datasource functions accept callables (feature_vector_fn, volatility_fn) to encapsulate state-dependent operations rather than raw data"
  - "V2 imports shared types (OptionContract, CloseSignalV2, etc.) from V1 to avoid duplication"
  - "WheelStrategyV2 extends OptionsStrategyBasicV2 (not V1 parent) maintaining clean V2 inheritance chain"

patterns-established:
  - "Callable-parameter datasource: when signal detection depends on stateful class data (candles_ltf DataFrame), pass the method as a callable rather than extracting raw data"
  - "Shared type imports: V2 modules import dataclass types from V1 instead of duplicating definitions"

requirements-completed: [MIG-01]

# Metrics
duration: 7min
completed: 2026-03-31
---

# Phase 22 Plan 03: CoveredCall and Wheel V2 Migration Summary

**CoveredCall and Wheel strategies migrated to V2 with supertrend-based datasource extraction and 8 behavioral diff tests proving zero metric drift**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-31T02:04:55Z
- **Completed:** 2026-03-31T02:11:58Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Extracted supertrend-based signal detection (open signals, roll signals) into covered_call_signals datasource using callable parameters for state-dependent operations
- Extracted put sell and put close signal detection into wheel_signals datasource
- Created OptionsStrategyBasicV2 delegating check_for_new_signal and check_for_roll_signal to datasource
- Created WheelStrategyV2 extending OptionsStrategyBasicV2 (clean V2 chain) with put signal delegation
- 8 behavioral diff tests all pass: no signal, open signal, roll signal, on_tick orders, put sell, put close, inherited calls, phase1 tick

## Task Commits

Each task was committed atomically:

1. **Task 1: Create CoveredCall datasource, V2 strategy, and diff test** - `0a8d01e` (feat)
2. **Task 2: Create Wheel datasource, V2 strategy, and diff test** - `168b1ce` (feat)

## Files Created/Modified
- `src/clients/python/datasources/covered_call_signals.py` - produce_open_signals and produce_roll_signals functions
- `src/clients/python/datasources/wheel_signals.py` - produce_put_sell_signals and produce_put_close_signals functions
- `src/clients/python/strategies/covered_call_v2.py` - OptionsStrategyBasicV2 class with datasource delegation
- `src/clients/python/strategies/wheel_v2.py` - WheelStrategyV2 extending OptionsStrategyBasicV2
- `src/clients/python/tests/test_covered_call_diff.py` - 4 behavioral diff tests for covered call V1 vs V2
- `src/clients/python/tests/test_wheel_diff.py` - 4 behavioral diff tests for wheel V1 vs V2
- `src/clients/python/datasources/__init__.py` - Updated exports for all datasource modules

## Decisions Made
- Used callable-parameter pattern for datasource functions (feature_vector_fn, volatility_fn) because CoveredCall's signal detection depends on candles_ltf DataFrame state that cannot be easily extracted into a pure function
- Imported shared types (OptionContract, CloseSignalV2, OpenSignalV4, RollSignalV1, etc.) from V1 modules instead of duplicating definitions
- Bypassed BaseOpenStrategyV2.__init__ in tests via __new__ + manual attribute setup to avoid complex candle fetching dependencies

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all datasource functions are fully wired and tested.

## Next Phase Readiness
- CoveredCall V2 and Wheel V2 are ready
- Plan 04 (CreditSpread, OptionsMeanReversion) can proceed independently
- Plan 05 (PDFWheel) depends on WheelStrategyV2 from this plan -- now available

## Self-Check: PASSED

---
*Phase: 22-remaining-strategy-migrations*
*Completed: 2026-03-31*
