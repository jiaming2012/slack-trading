---
phase: 22-remaining-strategy-migrations
plan: 02
subsystem: strategies
tags: [python, credit-spread, datasource, diff-test, trade-signal-framework]

# Dependency graph
requires:
  - phase: 21-first-strategy-migration-validation
    provides: "Datasource pattern (ma_crossover.py) and diff test pattern (test_mean_reversion_diff.py)"
provides:
  - "credit_spread_signals datasource with direction and horizon output"
  - "CreditSpreadStrategyV2 consuming datasource signals"
  - "5 behavioral diff tests proving V1/V2 equivalence"
affects: [22-remaining-strategy-migrations]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Bidirectional datasource pattern (produces direction field for bullish/bearish strategies)"

key-files:
  created:
    - src/clients/python/datasources/credit_spread_signals.py
    - src/clients/python/strategies/credit_spread_v2.py
    - src/clients/python/tests/test_credit_spread_diff.py
  modified:
    - src/clients/python/datasources/__init__.py

key-decisions:
  - "Compare at _try_create_group boundary (not place_order) because order placement depends on options ladder RPC which is identical in both V1 and V2"
  - "Added direction and horizon fields to credit_spread_signals output to support bidirectional trading"

patterns-established:
  - "Bidirectional datasource: produce_signals returns direction field (bullish/bearish) for strategies that trade both directions"
  - "Group-level diff testing: compare _try_create_group calls when downstream logic (entries/exits) is identical between V1 and V2"

requirements-completed: [MIG-01]

# Metrics
duration: 4min
completed: 2026-03-31
---

# Phase 22 Plan 02: Credit Spread Strategy Migration Summary

**CreditSpreadStrategyV2 consuming credit_spread_signals datasource with bidirectional signal output, validated by 5 behavioral diff tests**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-31T02:05:32Z
- **Completed:** 2026-03-31T02:09:52Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Extracted signal detection from CreditSpreadStrategy._process_htf_candle() into credit_spread_signals.produce_signals()
- Extended datasource pattern with direction and horizon fields for bidirectional credit spread trading
- Created CreditSpreadStrategyV2 (2020-line full copy) consuming datasource signals
- All 5 diff tests pass proving zero metric drift between V1 and V2

## Task Commits

Each task was committed atomically:

1. **Task 1: Create credit_spread_signals datasource and CreditSpreadStrategyV2** - `2143c19` (feat)
2. **Task 2: Create CreditSpread behavioral diff test** - `2da93c9` (test)

## Files Created/Modified
- `src/clients/python/datasources/credit_spread_signals.py` - Stateless signal producer with direction + horizon output
- `src/clients/python/strategies/credit_spread_v2.py` - V2 strategy consuming datasource signals
- `src/clients/python/tests/test_credit_spread_diff.py` - 5 behavioral diff tests (no signal, bullish, bearish, long_only, multi-signal)
- `src/clients/python/datasources/__init__.py` - Added credit_spread_signals export

## Decisions Made
- Compare at _try_create_group boundary rather than place_order because order placement requires options ladder RPC (identical between V1 and V2)
- Extended datasource output format with direction and horizon fields for bidirectional strategies

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed call object unpacking in _normalize_group_calls**
- **Found during:** Task 2 (diff test creation)
- **Issue:** unittest.mock.call objects require `.args` attribute access, not tuple unpacking
- **Fix:** Changed `args, kwargs = c` to `args = c.args`
- **Files modified:** src/clients/python/tests/test_credit_spread_diff.py
- **Verification:** All 5 tests pass
- **Committed in:** 2da93c9 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor test helper fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all code is fully wired and functional.

## Next Phase Readiness
- Credit spread migration complete, ready for next strategy migration (Plan 03)
- Bidirectional datasource pattern established for other strategies that trade both directions

---
*Phase: 22-remaining-strategy-migrations*
*Completed: 2026-03-31*
