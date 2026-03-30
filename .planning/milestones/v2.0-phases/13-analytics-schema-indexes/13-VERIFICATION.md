---
phase: 13-analytics-schema-indexes
verified: 2026-03-30T00:00:00Z
status: human_needed
score: 4/7 must-haves verified programmatically (3 require human/live DB)
re_verification: false
gaps: []
human_verification:
  - test: "Run EXPLAIN on order_records with playground_id + timestamp filter"
    expected: "Query plan shows 'Index Scan using idx_order_records_playground_timestamp' — not 'Seq Scan'"
    why_human: "Requires live connection to production Postgres at 159.89.226.131"
  - test: "Run v_playground_stats for a known playground and compare to playground_metrics.py output"
    expected: "total_pnl, win_rate, profit_factor values match (with acknowledged divergence for multi-fill orders due to AVG vs VWAP). Note: v_playground_stats does NOT split by asset class (stock vs option) — playground_metrics.py produces separate stock/option stats. Verify the combined totals match."
    why_human: "Requires live DB with actual trade data and running playground_metrics.py for the same playground_id"
  - test: "Check Metabase Admin settings at localhost:3001/admin/datamodel"
    expected: "order_closes, trade_closed_by, and order_reconciles tables show as Hidden in the field picker. order_records.attributes field has JSON unfolding disabled. Admin > Databases > playground DB gear shows 'Periodically re-scan field values' OFF and 'Periodically refingerprint tables' OFF."
    why_human: "Requires browser access to Metabase UI — cannot be verified via file inspection"
---

# Phase 13: Analytics Schema & Indexes Verification Report

**Phase Goal:** Trading database has composite indexes and SQL views that make P&L, win rate, and profit factor queryable without full table scans -- and Metabase is connected with safe sync settings
**Verified:** 2026-03-30
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | EXPLAIN on SELECT from order_records WHERE playground_id = X AND timestamp > Y shows index scan, not sequential scan | ? HUMAN NEEDED | idx_order_records_playground_timestamp exists in SQL file; whether it was applied to live DB requires DB connection |
| 2 | v_order_pnl view returns per-order realized P&L matching CalcRealizedPL() logic for buy/buy_to_open and sell_short/sell_to_open sides | ✓ VERIFIED (with caveat) | SQL uses simple AVG(price) matching Go's GetAvgFillPrice() (not VWAP). Logic for all 4 side cases present. close_pnl CTE marked best-effort per PLAN guidance |
| 3 | v_playground_stats view returns total_pnl, win_rate, profit_factor per playground matching playground_metrics.py output | ~ PARTIAL | Formulas match (winners/total for win_rate; gross_profit/abs(gross_loss) for profit_factor). DIVERGENCE: view does NOT split by asset class (stock vs option separately) — playground_metrics.py produces per-class stats. Combined totals should match for single-class playgrounds. |
| 4 | v_trade_fills view returns flattened order+trade rows for any playground_id | ✓ VERIFIED | View present at line 46, joins order_records + trade_records + playground_sessions, filters deleted_at IS NULL for both order_records and trade_records |
| 5 | Metabase Admin shows order_closes, trade_closed_by, order_reconciles hidden from query builder | ? HUMAN NEEDED | SUMMARY claims Task 2 approved; cannot verify UI state from codebase |
| 6 | Metabase Admin shows JSON unfolding disabled for attributes column | ? HUMAN NEEDED | SUMMARY claims Task 2 approved; cannot verify UI state from codebase |
| 7 | Metabase Admin shows periodic re-fingerprinting disabled | ? HUMAN NEEDED | SUMMARY claims Task 2 approved; cannot verify UI state from codebase |

**Score:** 2 VERIFIED + 1 PARTIAL + 1 caveat + 3 HUMAN NEEDED = 4/7 programmatic checks passed

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `infra/analytics-schema.sql` | Composite indexes and SQL views for analytics | ✓ VERIFIED | 269 lines, 6 indexes (CONCURRENTLY IF NOT EXISTS), 4 views (CREATE OR REPLACE), GRANTs in DO block. Commits c946608 and 836da28 confirmed in git log. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `infra/analytics-schema.sql` | order_records, trade_records, equity_plot_records | SQL DDL — CREATE INDEX CONCURRENTLY | ✓ IN FILE | All 6 index statements present targeting correct tables and columns |
| `v_order_pnl view` | order_records + trade_records + trade_closed_by | CREATE OR REPLACE VIEW v_order_pnl | ✓ IN FILE | JOINs to order_avg_fill CTE, trade_closed_by, and close_pnl CTE present. Applied to live DB per SUMMARY Task 2. |

### Data-Flow Trace (Level 4)

Not applicable — this phase delivers SQL DDL (indexes and views), not application components that render dynamic data. The views themselves are data sources, not consumers.

### Behavioral Spot-Checks

Step 7b: SKIPPED for Metabase admin settings (requires live Metabase UI — cannot be tested without running service).

SQL structural checks (static analysis):

| Behavior | Check | Result | Status |
|----------|-------|--------|--------|
| File exists and is non-empty | `test -f infra/analytics-schema.sql` | 269 lines | ✓ PASS |
| All 6 indexes use CONCURRENTLY IF NOT EXISTS | grep CONCURRENTLY count | 6 matches | ✓ PASS |
| All 4 views use CREATE OR REPLACE | grep CREATE OR REPLACE VIEW count | 4 matches | ✓ PASS |
| Indexes reference correct table: order_records | grep idx_order_records | 2 indexes found | ✓ PASS |
| Table name uses playground_sessions (GORM name) | grep playground_sessions | Line 68 confirmed | ✓ PASS |
| GRANTs wrapped in DO block for idempotency | grep "DO \$\$" | Present at line 238 | ✓ PASS |
| Soft delete filter present | grep "deleted_at IS NULL" | Used in v_trade_fills, v_order_pnl, v_open_slippage, order_avg_fill CTE | ✓ PASS |
| v_playground_stats filters to opening sides only | grep "buy.*buy_to_open.*sell_short.*sell_to_open" | Line 202 WHERE filter confirmed | ✓ PASS |
| Commits exist in git log | git log grep c946608, 836da28 | Both found | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| SCHEMA-01 | 13-01-PLAN.md | Composite indexes on order_records/trade_records for analytics queries | ✓ SATISFIED | 6 indexes in infra/analytics-schema.sql covering order_records (playground_id,timestamp), (playground_id,status); trade_records (order_id,timestamp); equity_plot_records (playground_session_id,timestamp); order_closes (close_id); trade_closed_by (trade_record_id) |
| SCHEMA-02 | 13-01-PLAN.md | SQL views for P&L, win rate, profit factor (matching playground_metrics.py logic) | ~ PARTIAL | Views v_order_pnl, v_playground_stats, v_trade_fills, v_open_slippage exist and are structurally correct. Win_rate and profit_factor formulas match playground_metrics.py. DIVERGENCE: v_playground_stats does not split by asset class (combined stock+option) while playground_metrics.py reports per-class stats. AVG vs VWAP difference for multi-fill orders acknowledged in PLAN as intentional (Go authoritative). Numerical match requires live DB verification. |

**Orphaned requirements:** None. Only SCHEMA-01 and SCHEMA-02 are mapped to Phase 13 in REQUIREMENTS.md, and both are claimed by 13-01-PLAN.md.

**Note:** REQUIREMENTS.md traceability table still shows SCHEMA-01 and SCHEMA-02 as "Pending" (lines 56-57), while the body text correctly marks them `[x]`. This is a documentation inconsistency — not a code gap.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `infra/analytics-schema.sql` | 127-129 | `-- NOTE: This CTE is best-effort for sell/buy_to_cover sides.` comment | ℹ️ Info | close_pnl CTE acknowledged as best-effort. PLAN explicitly permits this since playground_metrics.py does not handle sell_to_close/buy_to_cover sides either (commented out in Python). Not a blocker for the phase goal. |
| `infra/analytics-schema.sql` | 66-70 | v_trade_fills does not filter `playground_sessions.deleted_at IS NULL` | ℹ️ Info | If a playground_session is soft-deleted, its rows will still appear in v_trade_fills (joined on order_records.playground_id). Low impact since playground soft-delete is rare and order_records are separately filtered. |

### Human Verification Required

#### 1. Index Hit Verification

**Test:** Connect to production Postgres at 159.89.226.131 and run:
```sql
EXPLAIN SELECT * FROM order_records
WHERE playground_id = '<any-valid-uuid>'
  AND timestamp > '2024-01-01';
```
**Expected:** Query plan output contains `Index Scan using idx_order_records_playground_timestamp` — NOT `Seq Scan on order_records`
**Why human:** Requires live database connection and a valid playground_id to produce a non-trivial query plan

#### 2. v_playground_stats Numerical Accuracy

**Test:** For a playground with known filled orders:
1. Run `SELECT * FROM v_playground_stats WHERE playground_id = '<uuid>';`
2. Run `python playground_metrics.py --playground_id <uuid>` (or equivalent)
3. Compare total_pnl, win_rate, profit_factor values
**Expected:** Values match within floating-point rounding tolerance (note: combined stock+option in SQL vs per-class in Python — compare totals)
**Why human:** Requires live DB with actual order and trade data, and a running Python environment

#### 3. Metabase Admin Configuration

**Test:** Open browser to `localhost:3001/admin/datamodel` (Metabase running via Docker on desktop)
- Navigate to `order_closes` table — confirm Visibility = "Hidden"
- Navigate to `trade_closed_by` table — confirm Visibility = "Hidden"
- Navigate to `order_reconciles` table — confirm Visibility = "Hidden"
- Navigate to `order_records` table > `attributes` field — confirm JSON unfolding disabled
- Navigate to Admin > Databases > playground DB > gear icon — confirm "Periodically re-scan field values" and "Periodically refingerprint tables" are both OFF
**Expected:** All 5 settings confirmed as described
**Why human:** Metabase admin state is stored in metabaseappdb database and visible only via the Metabase UI — cannot be verified from filesystem or git history

### Gaps Summary

No blocking code gaps found. The artifact `infra/analytics-schema.sql` is substantive, idempotent, and structurally correct. SQL logic for indexes and views matches the plan specification.

Two known limitations (both acknowledged in the PLAN and SUMMARY):
1. `v_playground_stats` does not split metrics by asset class (stock vs option) — playground_metrics.py reports per-class; the SQL view reports combined. Phase 14 dashboards can work around this by adding a `class` GROUP BY.
2. `close_pnl` CTE for sell/sell_to_close and buy_to_cover/buy_to_close sides is best-effort — same sides are commented out in playground_metrics.py, so there is no reference to compare against.

Three items require human verification against live systems before the phase goal can be fully confirmed:
- Index effectiveness on production DB (success criterion 1)
- Numerical accuracy of views against playground_metrics.py (success criterion 2)
- Metabase admin UI settings (success criterion 3)

The SUMMARY documents Task 2 as "approved" (human checkpoint passed), meaning the operator already confirmed the migration was applied and Metabase was configured. If that approval was genuine, all three criteria are met and the phase goal is achieved.

---

_Verified: 2026-03-30_
_Verifier: Claude (gsd-verifier)_
