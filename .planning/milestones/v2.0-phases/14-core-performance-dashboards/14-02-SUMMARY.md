---
phase: 14-core-performance-dashboards
plan: 02
subsystem: infra
tags: [metabase, sql-views, dashboards, analytics, postgres]

# Dependency graph
requires:
  - phase: 14-01
    provides: analytics-schema.sql with slippage views, provision-metabase.py provisioning script
provides:
  - 3 live Metabase dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics)
  - SQL views deployed to production database (v_trade_fills, v_order_pnl, v_playground_stats, v_open_slippage, v_close_slippage, v_all_slippage)
  - metabase_ro grants applied for all views
affects: [15-simulator-persistence, 16-spread-analytics]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Metabase v0.59 uses `dashcards` key (not `ordered_cards`) for PUT /api/dashboard/:id"
    - "Each new dashcard needs a unique negative id for Metabase v0.59+"

key-files:
  created: []
  modified:
    - infra/analytics-schema.sql
    - infra/provision-metabase.py

key-decisions:
  - "Fixed Metabase v0.59 API compatibility: dashcards key and unique negative IDs for new cards"

patterns-established:
  - "Metabase provisioning script is idempotent and can be re-run to update dashboards"

requirements-completed: [DASH-01, DASH-02, DASH-04]

# Metrics
duration: human-gated
completed: 2026-03-30
---

# Phase 14 Plan 02: Deploy SQL Migration & Verify Dashboards Summary

**6 SQL analytics views deployed and 3 Metabase dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) provisioned and verified with real trading data**

## Performance

- **Duration:** Human-gated (migration + provisioning + manual verification)
- **Started:** 2026-03-30
- **Completed:** 2026-03-30
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Deployed 6 SQL views to production Postgres: v_trade_fills, v_order_pnl, v_playground_stats, v_open_slippage, v_close_slippage, v_all_slippage
- Provisioned 15 cards across 3 Metabase dashboards via API (Trading Performance, Slippage Analysis, Portfolio Analytics)
- User verified all dashboards display correct data with playground filter working

## Task Commits

Each task was committed atomically:

1. **Task 1: Run SQL migration and provisioning script** - `c49aad1` + `4466f3e` (feat + fix)
   - `c49aad1`: Merge of plan 14-01 worktree with analytics-schema.sql and provision-metabase.py
   - `4466f3e`: Fix provisioning script for Metabase v0.59 API compatibility
2. **Task 2: Verify dashboards display correct data** - Human-verified (no code changes)

## Files Created/Modified
- `infra/analytics-schema.sql` - 6 SQL views for trading analytics (P&L, slippage, stats)
- `infra/provision-metabase.py` - Metabase API provisioning script for 3 dashboards with 15 cards

## Decisions Made
- Fixed Metabase v0.59 API: switched from `ordered_cards` to `dashcards` key for dashboard layout PUT requests
- Each new dashcard assigned unique negative ID (Metabase v0.59+ requirement for card creation)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Metabase v0.59 API incompatibility in provisioning script**
- **Found during:** Task 1 (Run provisioning script)
- **Issue:** Metabase v0.59 uses `dashcards` key instead of `ordered_cards` for PUT /api/dashboard/:id, and requires unique negative IDs for new dashcards
- **Fix:** Updated provision-metabase.py to use `dashcards` key and assign unique negative IDs
- **Files modified:** infra/provision-metabase.py
- **Verification:** Script ran successfully, all 15 cards created across 3 dashboards
- **Committed in:** 4466f3e

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Essential fix for Metabase API compatibility. No scope creep.

## Issues Encountered
- Metabase v0.59 changed its API contract for dashboard card management. Resolved by updating the provisioning script to match the new API shape.

## User Setup Required
None - dashboards are deployed and verified.

## Next Phase Readiness
- All 3 core dashboards operational in Metabase at 192.168.8.164:3001
- SQL views provide the analytics foundation for Phase 15 (backtest comparison) and Phase 16 (spread analytics)
- Provisioning script pattern established for future dashboard creation

## Self-Check: PASSED

- infra/analytics-schema.sql: FOUND
- infra/provision-metabase.py: FOUND
- Commit c49aad1: FOUND
- Commit 4466f3e: FOUND

---
*Phase: 14-core-performance-dashboards*
*Completed: 2026-03-30*
