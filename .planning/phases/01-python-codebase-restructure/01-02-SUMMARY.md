---
phase: 01-python-codebase-restructure
plan: 02
subsystem: python-client
tags: [python, strategy, base-class, abc, trading-engine, consolidation]

requires:
  - phase: 01-01
    provides: "Structured Python package layout (strategies/, engine/, lib/) with absolute imports"
provides:
  - "BaseStrategy ABC with on_tick, get_next_tick_seconds, should_fetch_account (abstract) + on_retrain, on_complete, is_complete (concrete)"
  - "All 6 strategies inherit BaseStrategy and implement the required interface"
  - "Universal run_strategy() tick loop in engine/trading_engine.py"
affects: [01-03, 02-strategy-consolidation]

tech-stack:
  added: []
  patterns:
    - "BaseStrategy ABC as common interface for all trading strategies"
    - "Multiple inheritance (BaseOpenStrategyV2, BaseStrategy) for covered_call backward compat"
    - "Smart tick pattern: LTF when active trades, HTF when idle (D-03)"
    - "on_tick() wraps signal generation + order placement into single method"

key-files:
  created:
    - src/clients/python/strategies/base_strategy.py
  modified:
    - src/clients/python/strategies/covered_call.py
    - src/clients/python/strategies/wheel.py
    - src/clients/python/strategies/pdf_wheel.py
    - src/clients/python/strategies/mean_reversion.py
    - src/clients/python/strategies/options_mean_reversion.py
    - src/clients/python/strategies/credit_spread.py
    - src/clients/python/engine/trading_engine.py

key-decisions:
  - "Used multiple inheritance (BaseOpenStrategyV2, BaseStrategy) for OptionsStrategyBasic to preserve BaseOpenStrategyV2 methods (append_candle, candles_ltf, etc.)"
  - "Moved order placement logic from run_*_strategy() runner functions into on_tick() methods on each strategy class"
  - "Renamed old run_strategy() to _legacy_run_strategy() to avoid name collision with new universal run_strategy()"

patterns-established:
  - "Strategy interface: on_tick(tick_deltas), get_next_tick_seconds(), should_fetch_account() are the three abstract methods"
  - "Lifecycle hooks: on_retrain() (no-op default), on_complete() (no-op default), is_complete() (delegates to playground)"
  - "Engine loop: while not strategy.is_complete(): tick_deltas -> on_tick -> on_retrain -> tick"

requirements-completed: [CONS-01, CONS-02]

duration: 9min
completed: 2026-03-26
---

# Phase 01 Plan 02: Strategy Consolidation Summary

**BaseStrategy ABC with 3 abstract + 3 lifecycle methods, all 6 strategies adapted, and universal run_strategy() tick loop in trading engine**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-26T13:04:50Z
- **Completed:** 2026-03-26T13:13:50Z
- **Tasks:** 2
- **Files modified:** 8 (1 created, 7 modified)

## Accomplishments
- Created BaseStrategy ABC with on_tick, get_next_tick_seconds, should_fetch_account (abstract) plus on_retrain, on_complete, is_complete (concrete defaults)
- Adapted all 6 strategies to inherit from BaseStrategy with full interface implementation
- Added universal run_strategy() function to trading_engine.py with enable_retraining flag for optimizer compatibility (D-05)
- Preserved all existing strategy logic and backward compatibility

## Task Commits

Each task was committed atomically:

1. **Task 1: Create BaseStrategy and adapt all 6 strategies** - `d62457b` (feat)
2. **Task 2: Consolidate trading_engine.py into universal tick loop orchestrator** - `49b8ade` (feat)

## Files Created/Modified
- `src/clients/python/strategies/base_strategy.py` - BaseStrategy ABC with 3 abstract + 3 lifecycle methods
- `src/clients/python/strategies/covered_call.py` - OptionsStrategyBasic now inherits BaseStrategy; on_tick wraps tick() + order placement
- `src/clients/python/strategies/wheel.py` - WheelStrategy overrides on_tick for put/call signal handling
- `src/clients/python/strategies/pdf_wheel.py` - PDFWheelStrategy overrides on_tick for PDF-guided put signals + Kelly sizing
- `src/clients/python/strategies/mean_reversion.py` - MeanReversionStrategy inherits BaseStrategy; on_tick delegates to process_candles
- `src/clients/python/strategies/options_mean_reversion.py` - OptionsMeanReversionStrategy inherits BaseStrategy with same pattern
- `src/clients/python/strategies/credit_spread.py` - CreditSpreadStrategy inherits BaseStrategy with same pattern
- `src/clients/python/engine/trading_engine.py` - New run_strategy() universal loop; old run_strategy renamed to _legacy_run_strategy

## Decisions Made
- Used multiple inheritance (BaseOpenStrategyV2, BaseStrategy) for OptionsStrategyBasic to preserve existing candle management methods without copying them
- Each strategy's on_tick() encapsulates both signal generation (tick()) and order placement (from runner functions), making strategies self-contained
- Renamed old run_strategy() to _legacy_run_strategy() rather than removing it, since objective() still depends on the legacy pattern

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None.

## Next Phase Readiness
- BaseStrategy interface and universal run_strategy() are ready for Plan 03 (optimizer refactoring)
- All strategies can now be run through either the legacy runner functions or the new engine loop
- Demo scripts still use the per-strategy runner functions; Plan 03 will migrate them to run_strategy()

---
*Phase: 01-python-codebase-restructure*
*Completed: 2026-03-26*
