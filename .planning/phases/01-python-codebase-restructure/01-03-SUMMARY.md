---
phase: 01-python-codebase-restructure
plan: 03
subsystem: python-client
tags: [python, strategy, demos, optimizer, run_strategy, trading-engine]

requires:
  - phase: 01-02
    provides: "BaseStrategy ABC and universal run_strategy() tick loop in engine/trading_engine.py"
provides:
  - "All 6 demo scripts use engine.trading_engine.run_strategy() instead of per-strategy runner functions"
  - "trading_engine_optimizer.py supports new strategy interface via strategy_factory parameter"
  - "run_strategy() supports on_tick callback for PDF retraining and tracking"
  - "344 tests collect and 296 pass (40 pre-existing failures, none caused by restructure)"
affects: [02-strategy-consolidation]

tech-stack:
  added: []
  patterns:
    - "on_tick callback parameter in run_strategy() for external callbacks (retrain, spread tracking)"
    - "Strategy factory pattern in optimizer: (kwargs) -> (strategy, playground, metric_fn)"

key-files:
  created: []
  modified:
    - src/clients/python/demos/demo_covered_call.py
    - src/clients/python/demos/demo_wheel_strategy.py
    - src/clients/python/demos/demo_pdf_wheel_strategy.py
    - src/clients/python/demos/demo_mean_reversion.py
    - src/clients/python/demos/demo_options_mean_reversion.py
    - src/clients/python/demos/demo_credit_spread.py
    - src/clients/python/engine/trading_engine.py
    - src/clients/python/engine/trading_engine_optimizer.py

key-decisions:
  - "Added on_tick callback to run_strategy() to support demo retrain callbacks without modifying strategy classes"
  - "Used strategy_factory pattern for optimizer (D-06) with enable_retraining=False (D-05)"
  - "Preserved legacy objective() path in optimizer as fallback for deprecated strategy classes"

patterns-established:
  - "Demo pattern: create strategy instance, call run_strategy(strategy, playground, logger, on_tick=callback)"
  - "Optimizer factory pattern: strategy_factory(kwargs) returns (strategy, playground, metric_fn)"

requirements-completed: [CONS-03, CONS-04]

duration: 11min
completed: 2026-03-26
---

# Phase 01 Plan 03: Demo and Optimizer Refactoring Summary

**All 6 demo scripts wired through run_strategy(), optimizer updated with strategy factory pattern (D-06) and enable_retraining=False (D-05), 344 tests collect with 296 passing**

## Performance

- **Duration:** 11 min
- **Started:** 2026-03-26T13:17:55Z
- **Completed:** 2026-03-26T13:28:45Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Refactored all 6 demo scripts to use engine.trading_engine.run_strategy() instead of per-strategy runner functions
- Added on_tick callback parameter to run_strategy() for PDF retrain and spread tracking callbacks
- Refactored trading_engine_optimizer.py with strategy_factory parameter using run_strategy() with enable_retraining=False
- Validated all 344 tests collect and 296 pass (40 failures are pre-existing, not caused by restructure)

## Task Commits

Each task was committed atomically:

1. **Task 1: Refactor all 6 demo scripts to use run_strategy()** - `7fdba94` (feat)
2. **Task 2: Refactor trading_engine_optimizer.py for new interface (D-06)** - `d79cb30` (feat)
3. **Task 3: Validate all tests pass** - No commit (no files modified; all test imports already correct from Plan 01)

## Files Created/Modified
- `src/clients/python/engine/trading_engine.py` - Added on_tick callback parameter to run_strategy()
- `src/clients/python/engine/trading_engine_optimizer.py` - Added strategy_factory parameter, imports run_strategy, uses enable_retraining=False
- `src/clients/python/demos/demo_covered_call.py` - Uses run_strategy() with OptionsStrategyBasic
- `src/clients/python/demos/demo_wheel_strategy.py` - Uses run_strategy() with WheelStrategy
- `src/clients/python/demos/demo_pdf_wheel_strategy.py` - Uses run_strategy() with PDFWheelStrategy and on_tick callback
- `src/clients/python/demos/demo_mean_reversion.py` - Uses run_strategy() with MeanReversionStrategy and on_tick callback
- `src/clients/python/demos/demo_options_mean_reversion.py` - Uses run_strategy() with OptionsMeanReversionStrategy and on_tick callback
- `src/clients/python/demos/demo_credit_spread.py` - Uses run_strategy() with CreditSpreadStrategy and on_tick callback

## Decisions Made
- Added on_tick callback to run_strategy() rather than requiring strategies to override on_retrain() -- this preserves the existing demo retrain callback pattern without modifying strategy classes
- Used strategy_factory pattern for optimizer (D-06) where factory returns (strategy, playground, metric_fn) tuple, keeping the gp_minimize integration clean
- Preserved legacy objective() path in optimizer since deprecated strategy classes don't implement BaseStrategy

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added on_tick callback to run_strategy()**
- **Found during:** Task 1 (demo refactoring)
- **Issue:** Demo scripts use on_tick callbacks for PDF retraining, but run_strategy() didn't support this
- **Fix:** Added optional on_tick parameter to run_strategy() that's called after each tick batch
- **Files modified:** src/clients/python/engine/trading_engine.py
- **Verification:** All 6 demos run --help without errors
- **Committed in:** 7fdba94

---

**Total deviations:** 1 auto-fixed (1 missing critical functionality)
**Impact on plan:** Essential for demo-engine integration. No scope creep.

## Issues Encountered
- 40 test failures in test_credit_spread_strategy.py, test_mean_reversion_strategy.py, test_options_mean_reversion_strategy.py, and test_mean_reversion_report.py are pre-existing (80 failures on dev branch before Plan 03 changes, reduced to 40 after our changes). These are test logic/assertion issues, not import problems. Per CONS-04, no test logic was modified.
- scikit-optimize (skopt) was not installed in the grodt conda env; installed it for import verification.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None.

## Next Phase Readiness
- Phase 01 (python-codebase-restructure) is complete:
  1. Directory structure established (Plan 01)
  2. BaseStrategy ABC and universal run_strategy() created (Plan 02)
  3. All demos and optimizer wired through new interface (Plan 03)
- Ready for Phase 02 and beyond
- Pre-existing test failures in 4 test files should be investigated in a future plan

## Self-Check: PASSED

---
*Phase: 01-python-codebase-restructure*
*Completed: 2026-03-26*
