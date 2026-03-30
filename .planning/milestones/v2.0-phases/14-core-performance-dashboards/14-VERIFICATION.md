---
phase: 14-core-performance-dashboards
verified: 2026-03-30T15:29:49Z
status: human_needed
score: 6/7 must-haves verified
human_verification:
  - test: "Open http://192.168.8.164:3001 and navigate to Trading Performance dashboard"
    expected: "Playground filter dropdown shows human-readable names (client_id values); selecting a playground populates Total P&L, Win Rate, Profit Factor, Total Trades scalar cards; equity curve chart shows line with drawdown; P&L over time chart shows cumulative line"
    why_human: "Dashboard rendering, filter interactivity, and chart display require browser access to live Metabase instance"
  - test: "Navigate to Slippage Analysis dashboard and select a playground"
    expected: "Slippage Summary table shows rows for 'open' and 'close' slippage types; Per-Trade Slippage Detail table shows rows with symbol, side, requested_price, fill_price, slippage values"
    why_human: "Table population requires live DB connection and Metabase rendering"
  - test: "Navigate to Portfolio Analytics dashboard and select a playground"
    expected: "Per-Symbol P&L table shows breakdown by symbol and class; Position History table shows chronological trades"
    why_human: "Table population requires live DB connection and Metabase rendering"
  - test: "Cross-check numbers: run SELECT * FROM v_playground_stats WHERE playground_id = '<id>' in psql and compare total_pnl, win_rate, profit_factor against Metabase dashboard numbers"
    expected: "Values match within rounding tolerance (win_rate within 0.1%, P&L within 0.01)"
    why_human: "Requires psql access to production DB and live Metabase dashboard to compare simultaneously"
---

# Phase 14: Core Performance Dashboards Verification Report

**Phase Goal:** Operator can select any playground and see its trading performance -- P&L, win rate, profit factor, slippage, and per-symbol breakdown -- all from Metabase
**Verified:** 2026-03-30T15:29:49Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                         | Status      | Evidence                                                                                                          |
|----|---------------------------------------------------------------------------------------------------------------|-------------|-------------------------------------------------------------------------------------------------------------------|
| 1  | Provisioning script creates Trading Performance dashboard with P&L, win rate, profit factor, equity curve cards | ✓ VERIFIED  | Lines 200-304: `build_trading_performance_cards` + `assemble_trading_performance` create 7 cards with correct SQL |
| 2  | Provisioning script creates Slippage Analysis dashboard with open/close/total slippage cards                  | ✓ VERIFIED  | Lines 308-374: `build_slippage_cards` + `assemble_slippage_analysis` create 3 cards querying `v_all_slippage`     |
| 3  | Provisioning script creates Portfolio Analytics dashboard with position history and per-symbol breakdown cards | ✓ VERIFIED  | Lines 378-456: `build_portfolio_cards` + `assemble_portfolio_analytics` create 4 cards with per-symbol and history SQL |
| 4  | All dashboards have playground filter using client_id display with playground_id value                        | ✓ VERIFIED  | `PLAYGROUND_PARAMETER` dict (lines 140-146) with `type: string/=`; `_param_mapping` wires each card via `template-tag` |
| 5  | Script is re-runnable (idempotent) -- updates existing cards/dashboards, does not duplicate                   | ✓ VERIFIED  | `upsert_card` (line 75): `find_card_by_name` then PUT if exists, POST if not; same pattern in `upsert_dashboard`  |
| 6  | Close slippage SQL view exists in analytics-schema.sql for DASH-02                                            | ✓ VERIFIED  | Line 239: `CREATE OR REPLACE VIEW v_close_slippage AS` with 4 close sides; line 269: `v_all_slippage` UNION view   |
| 7  | Dashboard numbers match playground_metrics.py output within rounding tolerance (live data validation)         | ? UNCERTAIN | Human-verified per 14-02-SUMMARY (Task 2), but cannot verify programmatically without live Metabase access        |

**Score:** 6/7 truths verified programmatically (1 requires human/live-system confirmation)

### Required Artifacts

| Artifact                        | Expected                                             | Status      | Details                                                              |
|---------------------------------|------------------------------------------------------|-------------|----------------------------------------------------------------------|
| `infra/provision-metabase.py`   | Metabase API provisioning script for all 3 dashboards | ✓ VERIFIED  | 530 lines, valid Python syntax, all 3 dashboards present             |
| `infra/analytics-schema.sql`    | Extended with v_close_slippage and v_all_slippage views | ✓ VERIFIED | Lines 239-272: both views with correct SQL; GRANTs at lines 284-285  |

**Artifact minimum line check:** provision-metabase.py is 530 lines (min_lines requirement: 200 -- passes by 2.6x).

### Key Link Verification

| From                          | To                                              | Via                                      | Status       | Details                                                                          |
|-------------------------------|-------------------------------------------------|------------------------------------------|--------------|----------------------------------------------------------------------------------|
| `infra/provision-metabase.py` | Metabase REST API                               | `session.post`/`session.put` HTTP calls  | ✓ WIRED      | Lines 80, 84, 96, 100, 112: POST/PUT to `/api/card`, `/api/dashboard` endpoints  |
| `infra/provision-metabase.py` | `infra/analytics-schema.sql` (shared SQL views) | SQL queries reference same views         | ✓ WIRED      | v_playground_stats (lines 210, 215, 222, 229, 271, 277), v_order_pnl (lines 236, 387, 404, 414, 430), v_all_slippage (lines 317, 330, 342), v_trade_fills (line 404), equity_plot_records (line 252) -- all views queried |

**Key link pattern note:** The PLAN specifies pattern `requests\.post.*api/card|requests\.put.*api/dashboard` but the script uses a `requests.Session()` object (`session.post`, `session.put`) which is the correct idiom. The HTTP calls reach the same endpoints -- this is not a gap.

### Data-Flow Trace (Level 4)

The provisioning script is a deployment tool (not a rendering component) -- it makes HTTP calls to the Metabase API. The data flow is:

| Artifact                 | Data Variable      | Source                                   | Produces Real Data | Status      |
|--------------------------|--------------------|------------------------------------------|--------------------|-------------|
| `infra/provision-metabase.py` | `db_id`       | `GET /api/database` (live Metabase)      | Yes                | ✓ FLOWING   |
| `infra/provision-metabase.py` | card SQL queries | `v_playground_stats`, `v_order_pnl`, `v_all_slippage`, `v_trade_fills`, `equity_plot_records` | Yes -- views query real `order_records`, `trade_records`, `equity_plot_records` tables | ✓ FLOWING |
| Metabase dashboard cards | Rendered numbers   | Live Postgres via `metabase_ro`          | ? (live system)    | ? SKIP      |

### Behavioral Spot-Checks

| Behavior                             | Command                                                                          | Result       | Status  |
|--------------------------------------|----------------------------------------------------------------------------------|--------------|---------|
| provision-metabase.py parses without errors | `python3 -c "import ast; ast.parse(open('infra/provision-metabase.py').read())"` | syntax ok    | ✓ PASS  |
| All 3 dashboard names defined        | grep count in script                                                             | 9 occurrences | ✓ PASS  |
| All SQL views referenced             | grep count for v_playground_stats, v_order_pnl, v_all_slippage, v_trade_fills, equity_plot_records | 16 occurrences | ✓ PASS |
| v_close_slippage view exists in SQL  | grep in analytics-schema.sql                                                     | line 239     | ✓ PASS  |
| v_all_slippage view exists in SQL    | grep in analytics-schema.sql                                                     | line 269     | ✓ PASS  |
| metabase_ro GRANTs present           | grep in analytics-schema.sql                                                     | lines 284-285 | ✓ PASS |
| Metabase dashboards live at 192.168.8.164:3001 | Browser verification                                                  | Reported in 14-02-SUMMARY | ? SKIP (no server access) |

### Requirements Coverage

| Requirement | Source Plan | Description                                                        | Status          | Evidence                                                                                      |
|-------------|-------------|--------------------------------------------------------------------|-----------------|-----------------------------------------------------------------------------------------------|
| DASH-01     | 14-01, 14-02 | Trading performance dashboard: P&L over time, win rate, profit factor, gross profit/loss | ✓ SATISFIED | `Trading Performance` dashboard with 7 cards: Total P&L, Win Rate, Profit Factor, Total Trades, P&L Over Time, Equity Curve with Drawdown, Gross Profit/Loss, Win/Loss Breakdown |
| DASH-02     | 14-01, 14-02 | Slippage analysis dashboard: open/close/total slippage per playground | ✓ SATISFIED   | `Slippage Analysis` dashboard with 3 cards querying `v_all_slippage` (UNION of v_open_slippage + v_close_slippage); slippage_type column distinguishes open/close/all |
| DASH-04     | 14-01, 14-02 | Portfolio analytics: position history, per-symbol/asset-class breakdown | ✓ SATISFIED | `Portfolio Analytics` dashboard with `Portfolio: Per-Symbol P&L Breakdown`, `Portfolio: Position History`, `Portfolio: P&L by Asset Class`, `Portfolio: Active Symbols` cards |

**Orphaned requirements check:** REQUIREMENTS.md traceability table maps DASH-01, DASH-02, DASH-04 to Phase 14 -- all three are claimed in the plans and verified. No orphaned requirements.

**Documentation inconsistency (non-blocking):** REQUIREMENTS.md checkbox section marks DASH-01, DASH-02, DASH-04 as `[x]` (complete), but the traceability table still shows "Pending". This is a stale table -- the checkbox section is authoritative. Not a code gap.

### Anti-Patterns Found

| File                             | Line | Pattern                               | Severity  | Impact       |
|----------------------------------|------|---------------------------------------|-----------|--------------|
| `infra/provision-metabase.py`    | 110  | Docstring says `ordered_cards` but code uses `dashcards` | Info | Stale comment from pre-fix state; no functional impact |

No stubs, empty returns, hardcoded empty data, or placeholder comments found in either artifact. All SQL queries reference real views and tables. No `TODO`, `FIXME`, or `placeholder` patterns.

### Human Verification Required

#### 1. Trading Performance Dashboard Rendering

**Test:** Open http://192.168.8.164:3001, navigate to "Trading Performance" dashboard, select any playground from the filter dropdown.
**Expected:** Four scalar cards display non-null numbers for Total P&L, Win Rate, Profit Factor, Total Trades. Equity Curve chart renders a line. P&L Over Time chart renders cumulative P&L.
**Why human:** Dashboard rendering, filter interactivity, and chart display require browser access to the live Metabase instance.

#### 2. Slippage Analysis Dashboard -- Open and Close Rows

**Test:** Navigate to "Slippage Analysis", select the same playground.
**Expected:** "Slippage: Summary" table shows two rows: one for `open` slippage_type and one for `close` slippage_type. "Slippage: Per-Trade Detail" table shows individual fill rows.
**Why human:** Requires live Metabase and a playground that has both open and close orders with slippage in the production DB.

#### 3. Portfolio Analytics Dashboard

**Test:** Navigate to "Portfolio Analytics", select the same playground.
**Expected:** "Portfolio: Per-Symbol P&L Breakdown" table shows rows grouped by symbol/class. "Portfolio: Position History" table shows chronological trade rows.
**Why human:** Requires live Metabase and real trade data for the selected playground.

#### 4. Number Accuracy Cross-Check (Success Criterion 4)

**Test:** Pick one playground. Run `SELECT * FROM v_playground_stats WHERE playground_id = '<id>'` in psql. Compare total_pnl, win_rate, profit_factor against what the Trading Performance dashboard shows.
**Expected:** Values match within rounding tolerance (win_rate within 0.1%, P&L within 0.01).
**Why human:** Requires simultaneous access to production Postgres and live Metabase dashboard. The 14-02-SUMMARY states this was verified by the user during Task 2, but no automated evidence exists in the codebase.

### Gaps Summary

No code gaps found. All automated checks pass:

- `infra/provision-metabase.py` is a 530-line, syntactically valid Python script with all 3 dashboards fully defined, idempotent upsert pattern, template-tag playground filters, and drawdown SQL in the equity curve card.
- `infra/analytics-schema.sql` contains `v_close_slippage` and `v_all_slippage` views with correct SQL and `metabase_ro` GRANTs.
- All commits documented in SUMMARYs (eb1bcd2, 2d2eb97, c49aad1, 4466f3e) exist in git history.
- Metabase v0.59 API compatibility fix (4466f3e) is correctly applied: `set_dashboard_cards` uses `dashcards` key and unique negative IDs.

The one unverifiable truth is Success Criterion 4 (dashboard numbers match playground_metrics.py within rounding tolerance) -- this requires live system access. Per 14-02-SUMMARY, the user performed this verification manually on 2026-03-30.

---

_Verified: 2026-03-30T15:29:49Z_
_Verifier: Claude (gsd-verifier)_
