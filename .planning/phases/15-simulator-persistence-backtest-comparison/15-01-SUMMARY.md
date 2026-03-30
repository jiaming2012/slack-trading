---
phase: 15-simulator-persistence-backtest-comparison
plan: 01
subsystem: database
tags: [postgres, psycopg2, backtest, persistence, argparse]

requires:
  - phase: 13-analytics-schema-indexes
    provides: v_playground_stats view for authoritative metrics
  - phase: 14-core-performance-dashboards
    provides: analytics-schema.sql with indexes and views
provides:
  - backtest_runs table in analytics-schema.sql
  - save_backtest_run() Python function for persisting backtest results
  - get_parameters() method on BaseStrategy and MeanReversionStrategy
  - --save-to-db CLI flag on demo_mean_reversion.py
affects: [15-02-strategy-comparison-dashboard, 16-spread-analytics]

tech-stack:
  added: [psycopg2-binary]
  patterns: [opt-in persistence via CLI flag, v_playground_stats as single source of truth for metrics]

key-files:
  created:
    - src/clients/python/engine/persistence.py
    - src/clients/python/tests/test_persistence.py
  modified:
    - infra/analytics-schema.sql
    - src/clients/python/strategies/base_strategy.py
    - src/clients/python/strategies/mean_reversion.py
    - src/clients/python/demos/demo_mean_reversion.py

key-decisions:
  - "Metrics sourced from v_playground_stats view (not computed in Python) for dashboard consistency"
  - "psycopg2-binary used directly (not SQLAlchemy) to keep persistence module lightweight"
  - "get_parameters() returns empty dict by default so existing strategies are unaffected"

patterns-established:
  - "Opt-in persistence: --save-to-db flag only persists when explicitly passed"
  - "Strategy parameter capture: get_parameters() on BaseStrategy for serialization"

requirements-completed: [PERSIST-01, PERSIST-02]

duration: 4min
completed: 2026-03-30
---

# Phase 15 Plan 01: Simulator Persistence & Backtest Runs Summary

**backtest_runs SQL table with psycopg2 persistence module, strategy parameter capture via get_parameters(), and --save-to-db CLI flag on demo script**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T16:07:05Z
- **Completed:** 2026-03-30T16:11:09Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- backtest_runs table with FK to playground_sessions, JSONB parameters, and 3 indexes
- save_backtest_run() queries v_playground_stats for authoritative metrics before INSERT
- BaseStrategy.get_parameters() and MeanReversionStrategy override with all 9 tuning params
- --save-to-db flag on demo_mean_reversion.py calls SavePlayground RPC then persists summary row
- 4 unit tests covering insert, stats query ordering, JSON serialization, and missing stats

## Task Commits

Each task was committed atomically:

1. **Task 1: Add backtest_runs table and get_parameters()** - `d981c48` (feat)
2. **Task 2: Create persistence module and unit tests** - `79b7beb` (test: RED), `75f4e8a` (feat: GREEN)
3. **Task 3: Add --save-to-db flag to demo** - `73c1cbe` (feat)

## Files Created/Modified
- `infra/analytics-schema.sql` - Added backtest_runs table, indexes, and GRANT
- `src/clients/python/engine/persistence.py` - save_backtest_run() function
- `src/clients/python/tests/test_persistence.py` - 4 unit tests for persistence module
- `src/clients/python/strategies/base_strategy.py` - Added get_parameters() method
- `src/clients/python/strategies/mean_reversion.py` - Override get_parameters() with tuning params
- `src/clients/python/demos/demo_mean_reversion.py` - --save-to-db flag and persistence block

## Decisions Made
- Metrics sourced from v_playground_stats view (not computed in Python) for dashboard consistency
- psycopg2-binary used directly (not SQLAlchemy) to keep persistence module lightweight
- get_parameters() returns empty dict by default so existing strategies are unaffected
- Removed _return_model attribute from get_parameters() since model is a demo-level param passed separately

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Copied analytics-schema.sql from main repo**
- **Found during:** Task 1
- **Issue:** analytics-schema.sql existed on other worktree branches but not on this worktree's branch
- **Fix:** Copied the file from the main repo to establish the baseline before adding backtest_runs
- **Files modified:** infra/analytics-schema.sql
- **Verification:** File exists and contains all prior views plus new table

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to establish baseline SQL file. No scope creep.

## Issues Encountered
None beyond the file copy deviation noted above.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all data paths are wired (save_backtest_run queries real v_playground_stats and inserts to real table).

## Next Phase Readiness
- backtest_runs table ready for Phase 15-02 to build Strategy Comparison dashboard
- Metabase can query backtest_runs via metabase_ro GRANT
- Schema must be applied to production DB before dashboard creation

---
*Phase: 15-simulator-persistence-backtest-comparison*
*Completed: 2026-03-30*
