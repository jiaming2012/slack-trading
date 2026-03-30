---
phase: 16-spread-analytics
plan: 01
subsystem: api, database
tags: [spread, multi-leg, uuid, jsonb, gin-index, sql-views, twirp]

# Dependency graph
requires:
  - phase: 13-analytics-schema-indexes
    provides: v_order_pnl view for per-order realized P&L
provides:
  - PlaceMultiLegOrder spread_group_key + leg_role attribute injection
  - v_spread_pnl SQL view for per-spread-group P&L aggregation
  - v_spread_stats SQL view for per-playground spread performance
  - GIN index on order_records.attributes for JSONB query performance
affects: [16-02, metabase-dashboards, spread-strategy]

# Tech tracking
tech-stack:
  added: []
  patterns: [spread-group-key-injection, jsonb-attribute-analytics, helper-function-extraction-for-testability]

key-files:
  created:
    - src/go/backtester-api/router/grpc_spread_test.go
  modified:
    - src/go/backtester-api/router/grpc.go
    - infra/analytics-schema.sql

key-decisions:
  - "Extracted buildMultiLegRequests helper from PlaceMultiLegOrder for testability instead of mocking Server dependencies"
  - "COALESCE(spread_group_key, group_id) in v_spread_pnl ensures backward compatibility with existing credit_spread.py data"
  - "Only opening-side orders in v_spread_pnl to avoid double-counting P&L (matches v_playground_stats pattern)"

patterns-established:
  - "Spread grouping via JSONB attributes: shared UUID key + per-leg role"
  - "Helper function extraction for RPC handler unit testing"

requirements-completed: [SPREAD-01, SCHEMA-03, SCHEMA-04, SPREAD-02]

# Metrics
duration: 4min
completed: 2026-03-30
---

# Phase 16 Plan 01: Spread Grouping & Analytics Views Summary

**PlaceMultiLegOrder injects shared spread_group_key UUID + leg_role into each leg's attributes; SQL views aggregate per-spread P&L with GIN index**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T18:49:19Z
- **Completed:** 2026-03-30T18:53:25Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- PlaceMultiLegOrder handler injects spread_group_key UUID and leg_role (short/long/unknown) into every leg's Attributes
- v_spread_pnl view aggregates per-leg P&L by spread group, supporting both new spread_group_key and legacy group_id
- v_spread_stats view provides win/loss/win_rate/avg_pnl per playground from completed spreads
- GIN index on order_records.attributes for efficient JSONB key extraction queries
- Unit tests validate attribute injection with 3 test cases covering all leg_role mappings

## Task Commits

Each task was committed atomically:

1. **Task 1: Inject spread_group_key and leg_role in PlaceMultiLegOrder handler** - `7d1830b` (feat)
2. **Task 2: Add v_spread_pnl, v_spread_stats views and GIN index** - `52355c4` (feat)
3. **Task 3: Unit test for PlaceMultiLegOrder spread attribute injection** - `4ea8741` (test)

## Files Created/Modified
- `src/go/backtester-api/router/grpc.go` - PlaceMultiLegOrder handler with spread attribute injection + extracted buildMultiLegRequests helper
- `src/go/backtester-api/router/grpc_spread_test.go` - Unit tests for spread attribute injection (3 test functions)
- `infra/analytics-schema.sql` - v_spread_pnl view, v_spread_stats view, GIN index, metabase_ro grants

## Decisions Made
- Extracted `buildMultiLegRequests` as a package-level helper function for direct unit testing instead of mocking the full Server struct with its concrete DatabaseService dependency
- Used COALESCE(spread_group_key, group_id) in v_spread_pnl to maintain backward compatibility with existing credit_spread.py strategies that set group_id in attributes
- Filtered v_spread_pnl to opening-side orders only (buy, buy_to_open, sell_short, sell_to_open) consistent with v_playground_stats pattern

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Worktree was behind dev branch**
- **Found during:** Pre-task setup
- **Issue:** Worktree was at commit 2df7d83 (old dev), missing PlaceMultiLegOrder handler, Attributes field on CreateOrderRequest, analytics-schema.sql, and all planning files
- **Fix:** Merged latest dev (343650f) into worktree via `git merge dev --no-edit`
- **Verification:** All required source files present and build succeeds

**2. [Rule 2 - Missing Critical] Extracted buildMultiLegRequests helper for testability**
- **Found during:** Task 3 (unit test creation)
- **Issue:** Server struct uses concrete `*data.DatabaseService` (not interface), making it impractical to unit test PlaceMultiLegOrder directly without full DB setup
- **Fix:** Extracted the request-building logic into a standalone `buildMultiLegRequests` function that can be tested independently
- **Files modified:** src/go/backtester-api/router/grpc.go
- **Verification:** `go build ./src/go/backtester-api/...` passes, all tests pass

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing critical)
**Impact on plan:** Both deviations were necessary for correct execution. The helper extraction improves code organization.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - no external service configuration required. Run `psql -f infra/analytics-schema.sql` against the playground database to deploy the new views and index.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Spread grouping infrastructure complete, ready for Phase 16 Plan 02 (Metabase dashboard queries)
- v_spread_pnl and v_spread_stats views ready for Metabase question configuration
- Python strategies using PlaceMultiLegOrder will automatically get spread tracking with no code changes

## Self-Check: PASSED

All files verified present. All commit hashes verified in git log.

---
*Phase: 16-spread-analytics*
*Completed: 2026-03-30*
