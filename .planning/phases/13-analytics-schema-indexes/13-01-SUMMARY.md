---
phase: 13-analytics-schema-indexes
plan: 01
subsystem: database
tags: [postgres, indexes, sql-views, metabase, analytics, pnl]

# Dependency graph
requires:
  - phase: 12-deploy-metabase-harden-infrastructure
    provides: Metabase running on Docker Desktop with metabase_ro read-only user
provides:
  - Composite indexes on order_records, trade_records, equity_plot_records for analytics queries
  - SQL views (v_trade_fills, v_order_pnl, v_playground_stats, v_open_slippage) for P&L analytics
  - Metabase admin configured with hidden join tables, disabled JSON unfolding, disabled re-fingerprinting
affects: [14-core-performance-dashboards, 15-simulator-persistence, 16-spread-analytics]

# Tech tracking
tech-stack:
  added: []
  patterns: [idempotent SQL migrations in infra/, CREATE INDEX CONCURRENTLY for zero-downtime, CREATE OR REPLACE VIEW for safe re-runs]

key-files:
  created: [infra/analytics-schema.sql]
  modified: []

key-decisions:
  - "Used simple AVG(price) not VWAP for GetAvgFillPrice to match Go server CalcRealizedPL() as authoritative source"
  - "v_playground_stats only aggregates opening-side orders (buy/buy_to_open, sell_short/sell_to_open) to avoid double-counting P&L"
  - "GRANTs wrapped in DO block for idempotency when metabase_ro role may not exist yet"

patterns-established:
  - "Idempotent SQL migrations: infra/ directory with CREATE INDEX CONCURRENTLY IF NOT EXISTS and CREATE OR REPLACE VIEW"
  - "Analytics views respect GORM soft deletes with WHERE deleted_at IS NULL"

requirements-completed: [SCHEMA-01, SCHEMA-02]

# Metrics
duration: human-interactive (checkpoint-based)
completed: 2026-03-30
---

# Phase 13 Plan 01: Analytics Schema & Indexes Summary

**Composite indexes on trading tables and SQL views replicating CalcRealizedPL() for Metabase P&L, win rate, and profit factor analytics**

## Performance

- **Duration:** Human-interactive (checkpoint-based with manual migration and Metabase config)
- **Tasks:** 2/2
- **Files created:** 1

## Accomplishments
- 6 composite indexes created on order_records, trade_records, equity_plot_records, order_closes, and trade_closed_by for analytics query patterns
- 4 SQL views providing flattened trade fills, per-order P&L, per-playground stats, and slippage analysis
- Metabase admin configured: join tables hidden, JSON unfolding disabled, periodic re-fingerprinting disabled
- metabase_ro granted SELECT on all new views

## Task Commits

Each task was committed atomically:

1. **Task 1: Create analytics schema SQL migration with indexes and views** - `c946608` (feat) + `836da28` (fix for production DB table names)
2. **Task 2: Run migration and configure Metabase admin settings** - Human checkpoint (approved)

## Files Created/Modified
- `infra/analytics-schema.sql` - Idempotent SQL migration with 6 composite indexes, 4 analytics views, and metabase_ro grants

## Decisions Made
- Used simple AVG(price) not VWAP for avg fill price to match Go server's CalcRealizedPL() as the authoritative source (Python's _calc_trade_position uses true VWAP but cross-validates against Go)
- v_playground_stats only aggregates opening-side orders to avoid double-counting P&L, matching playground_metrics.py behavior
- GRANTs wrapped in DO block for idempotency when metabase_ro role may not exist yet
- Metabase set up via Docker run (not docker-compose) due to Docker Desktop credential helper issue over SSH

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Table name was playground_sessions not playgrounds (GORM naming)**
- **Found during:** Task 2 (migration execution)
- **Issue:** GORM maps the Go `Playground` model to `playground_sessions` table, not `playgrounds`
- **Fix:** Updated SQL to reference correct table name in views
- **Files modified:** infra/analytics-schema.sql
- **Committed in:** 836da28

**2. [Rule 2 - Missing Critical] metabase_ro GRANTs wrapped in DO block for idempotency**
- **Found during:** Task 2 (migration execution)
- **Issue:** GRANT would fail if metabase_ro role doesn't exist yet on a fresh DB
- **Fix:** Wrapped GRANTs in a DO block that checks for role existence
- **Files modified:** infra/analytics-schema.sql
- **Committed in:** 836da28

**3. [Process] Metabase Docker run instead of docker-compose**
- **Found during:** Task 2 (Metabase setup)
- **Issue:** Docker Desktop credential helper issue over SSH prevented docker-compose usage
- **Fix:** Used direct Docker run command instead
- **Impact:** Functionally equivalent, Metabase running correctly on 192.168.8.164:3001

---

**Total deviations:** 3 (1 bug fix, 1 missing critical, 1 process)
**Impact on plan:** All fixes necessary for correct production deployment. No scope creep.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - migration already applied to production, Metabase already configured.

## Next Phase Readiness
- All indexes and views are live on production Postgres (159.89.226.131)
- Metabase can query v_playground_stats, v_order_pnl, v_trade_fills, v_open_slippage
- Ready for Phase 14: Core Performance Dashboards (building Metabase questions/dashboards on these views)

---
*Phase: 13-analytics-schema-indexes*
*Completed: 2026-03-30*
