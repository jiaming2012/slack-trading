---
phase: 25-complete-strategy-migrations
plan: 01
subsystem: strategies
tags: [covered-call, wheel, datasource, v2-migration, supertrend, diff-test]

# Dependency graph
requires:
  - phase: 22-remaining-strategy-migrations
    provides: V2 datasource pattern (credit_spread_signals, credit_spread_v2)
provides:
  - covered_call_signals datasource (produce_open_signals with callable-based feature vector)
  - OptionsStrategyBasicV2 (covered call V2 strategy)
  - WheelStrategyV2 (wheel V2 extending OptionsStrategyBasicV2)
  - wheel_signals datasource (produce_open_signals for put sell signals)
  - Behavioral diff tests for both strategies (9 total)
affects: [25-02, 25-03, trading-engine]

# Tech tracking
tech-stack:
  added: []
  patterns: [callable-based datasource for stateful signal extraction]

key-files:
  created:
    - src/clients/python/datasources/covered_call_signals.py
    - src/clients/python/strategies/covered_call_v2.py
    - src/clients/python/tests/test_covered_call_diff.py
    - src/clients/python/datasources/wheel_signals.py
    - src/clients/python/strategies/wheel_v2.py
    - src/clients/python/tests/test_wheel_diff.py
  modified:
    - src/clients/python/datasources/__init__.py

key-decisions:
  - "Callable-based datasource pattern: feature_vector_fn callable encapsulates stateful supertrend lookback over candles_ltf DataFrame"
  - "V2 inherits from both BaseOpenStrategyV2 and BaseStrategy (same as V1) to preserve candle state management"
  - "WheelStrategyV2 extends OptionsStrategyBasicV2 (not V1 parent) per D-06"
  - "Shared types imported from V1 (OptionContract, CloseSignalV2, etc.) -- no duplication"

patterns-established:
  - "Callable-based datasource: pass feature_vector_fn for stateful signal extraction (covered call and wheel use same pattern)"
  - "V2 strategies copy full V1 logic, only replacing signal DETECTION with datasource delegation"

requirements-completed: [MIG-01]

# Metrics
duration: 8min
completed: 2026-03-31
---

# Phase 25 Plan 01: CoveredCall + Wheel V2 Strategies Summary

**Extracted supertrend signal detection into callable-based datasources for covered call and wheel strategies, created V2 classes delegating to datasources, and proved V1/V2 behavioral equivalence with 9 diff tests**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-31T11:18:41Z
- **Completed:** 2026-03-31T11:26:33Z
- **Tasks:** 2/2
- **Files created:** 6
- **Files modified:** 1

## Accomplishments

### Task 1: CoveredCall datasource + V2 strategy + diff test
- Created `covered_call_signals.py` with `produce_open_signals(feature_vector_fn, candle, symbol)` -- extracts the supertrend-based signal detection from OptionsStrategyBasic.check_for_new_signal()
- Created `OptionsStrategyBasicV2` in `covered_call_v2.py` -- full copy of V1 logic, only check_for_new_signal() delegates to datasource via feature_vector_fn callable
- 4 behavioral diff tests prove V1/V2 equivalence: no-signal, direction-change, multi-change, callable verification
- **Commit:** 207a70a

### Task 2: Wheel datasource + V2 strategy + diff test
- Created `wheel_signals.py` with `produce_open_signals(feature_vector_fn, candle, symbol)` -- extracts put sell signal detection from WheelStrategy.check_for_put_signal()
- Created `WheelStrategyV2(OptionsStrategyBasicV2)` in `wheel_v2.py` -- Phase 1 uses wheel_signals datasource, Phase 2 delegates to parent
- 5 behavioral diff tests: put no-signal, put direction-change, Phase 2 delegation, inheritance chain verification, callable verification
- **Commit:** f850928

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

All 9 diff tests pass across both test suites:
- `test_covered_call_diff.py`: 4 passed
- `test_wheel_diff.py`: 5 passed

V2 inheritance chain verified: WheelStrategyV2 -> OptionsStrategyBasicV2 -> BaseOpenStrategyV2 + BaseStrategy

## Known Stubs

None -- all datasources are fully wired and functional.
