---
phase: 14-core-performance-dashboards
plan: 01
subsystem: infra
tags: [metabase, sql, python, dashboards, analytics]

# Dependency graph
requires:
  - phase: 13-analytics-schema-indexes
    provides: SQL views (v_playground_stats, v_order_pnl, v_trade_fills, v_open_slippage)
  - phase: 12-deploy-metabase-harden-infrastructure
    provides: Metabase instance at 192.168.8.164:3001 with metabase_ro role
provides:
  - Metabase provisioning script (infra/provision-metabase.py) for 3 dashboards
  - v_close_slippage and v_all_slippage SQL views for slippage analysis
  - Idempotent dashboard-as-code pattern via Metabase REST API
affects: [15-backtest-persistence, 16-spread-analytics]

# Tech tracking
tech-stack:
  added: []
  patterns: [metabase-api-provisioning, upsert-card-pattern, native-sql-questions, template-tag-filters]

key-files:
  created:
    - infra/provision-metabase.py
  modified:
    - infra/analytics-schema.sql

key-decisions:
  - "Native SQL questions for all cards (not MBQL) for full SQL control and view compatibility"
  - "Card names prefixed with dashboard name for organization (Trading: Total P&L, Slippage: Summary, etc.)"
  - "Supports both API key and session auth via CLI args"

patterns-established:
  - "Metabase API upsert: find_card_by_name then PUT/POST for idempotent provisioning"
  - "Template-tag playground_id filter shared across all dashboards"

requirements-completed: [DASH-01, DASH-02, DASH-04]

# Metrics
duration: 2m24s
completed: 2026-03-30
---

# Phase 14 Plan 01: Core Performance Dashboards Summary

**Metabase provisioning script creating 3 dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) with 15 native SQL cards, playground filters, and equity curve with drawdown**

## Performance

- **Duration:** 2m 24s
- **Started:** 2026-03-30T15:05:03Z
- **Completed:** 2026-03-30T15:07:27Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Extended analytics-schema.sql with v_close_slippage and v_all_slippage views for complete slippage coverage
- Created 518-line idempotent provisioning script with 15 native SQL cards across 3 dashboards
- All cards use template-tag playground_id filter with ::uuid cast for consistent filtering
- Equity curve card includes drawdown calculation via window functions (per D-04)

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend analytics-schema.sql with close slippage and combined slippage views** - `eb1bcd2` (feat)
2. **Task 2: Create Metabase provisioning script with all 3 dashboards** - `2d2eb97` (feat)

## Files Created/Modified
- `infra/analytics-schema.sql` - Added v_close_slippage (close-side slippage) and v_all_slippage (UNION of open+close) views with metabase_ro grants
- `infra/provision-metabase.py` - Re-runnable Python script that creates 3 Metabase dashboards with 15 native SQL cards via REST API

## Decisions Made
- Used native SQL questions (not MBQL) for all cards -- gives full SQL control and directly queries existing views
- Card names prefixed with dashboard name (e.g., "Trading: Total P&L") for clear organization in Metabase card list
- Script supports both --api-key and --password auth modes for flexibility

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None - all dashboard cards have complete SQL queries wired to existing views and tables.

## User Setup Required

To provision dashboards, run:
```bash
python infra/provision-metabase.py --password <metabase-admin-password>
# or
python infra/provision-metabase.py --api-key <metabase-api-key>
```

## Next Phase Readiness
- All 3 core dashboards defined and ready to provision against Metabase
- v_close_slippage and v_all_slippage views ready for deployment to production DB
- Strategy comparison dashboard (DASH-03) deferred to Phase 15

## Self-Check: PASSED

- infra/analytics-schema.sql: FOUND
- infra/provision-metabase.py: FOUND
- 14-01-SUMMARY.md: FOUND
- Commit eb1bcd2 (Task 1): FOUND
- Commit 2d2eb97 (Task 2): FOUND

---
*Phase: 14-core-performance-dashboards*
*Completed: 2026-03-30*
