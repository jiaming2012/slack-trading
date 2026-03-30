---
phase: 15-simulator-persistence-backtest-comparison
verified: 2026-03-30T17:00:00Z
status: human_needed
score: 7/7 must-haves verified (automated); 1 item needs human verification
re_verification: false
human_verification:
  - test: "Run provision-metabase.py and inspect the Strategy Comparison dashboard in Metabase"
    expected: "Dashboard 4 (Strategy Comparison) appears with 5 cards (Backtest Runs table, Equity Curves, Parameter Values, Best Return, Worst Return) and a strategy_name filter. Filtering by strategy name correctly scopes the cards. Existing 3 dashboards are unaffected."
    why_human: "The dashboard code is wired and syntactically correct, but the actual Metabase UI rendering and filter wiring can only be confirmed by running the provisioning script against a live Metabase instance."
---

# Phase 15: Simulator Persistence & Backtest Comparison — Verification Report

**Phase Goal:** Backtest results are saved to Postgres so the operator can compare strategy runs side-by-side in Metabase
**Verified:** 2026-03-30T17:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running demo_mean_reversion.py with --save-to-db calls SavePlayground RPC on completion | VERIFIED | Lines 579-607 of demo_mean_reversion.py: `if args.save_to_db and not args.live:` block calls `SavePlaygroundRequest` then `network_call_with_retry` |
| 2 | After save, a backtest_runs row exists with correct metrics from v_playground_stats | VERIFIED | `persistence.py` SELECTs from `v_playground_stats` (line 73-78) before INSERT; `analytics-schema.sql` has `backtest_runs` table (lines 278-300) and all 3 indexes |
| 3 | Strategy parameters are captured as JSONB in the backtest_runs row | VERIFIED | `save_backtest_run` calls `json.dumps(parameters)` (line 98); MeanReversionStrategy.get_parameters() returns 9 tuning params; demo appends `args.model` |
| 4 | Running without --save-to-db does NOT persist anything | VERIFIED | Entire persistence block is guarded by `if args.save_to_db and not args.live:` — no persistence calls exist outside this guard |
| 5 | Operator can see a sortable table of all backtest runs with key metrics | VERIFIED | `Comparison: Backtest Runs` card in `build_strategy_comparison_cards()` queries `backtest_runs` with `ORDER BY br.created_at DESC` |
| 6 | Operator can overlay equity curves for multiple backtest runs | VERIFIED | `Comparison: Equity Curves` card JOINs `equity_plot_records` to `backtest_runs` and groups by `client_id` for multi-run overlay |
| 7 | Operator can filter backtest runs by strategy name | VERIFIED (code) / ? (UI) | `STRATEGY_TEMPLATE_TAGS` and `STRATEGY_PARAMETER` wired in `assemble_strategy_comparison()` with `_strategy_dash_card` param mapping; requires human confirmation in Metabase |

**Score:** 7/7 automated truths verified; 1 human verification pending

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `infra/analytics-schema.sql` | backtest_runs table with indexes and GRANT | VERIFIED | Lines 278-300: CREATE TABLE, 3 indexes (`idx_backtest_runs_strategy`, `idx_backtest_runs_playground`, `idx_backtest_runs_created`), GRANT SELECT to metabase_ro at line 314 |
| `src/clients/python/engine/persistence.py` | save_backtest_run() function using psycopg2 | VERIFIED | 115-line module; exports `save_backtest_run`; queries `v_playground_stats` before INSERT; accepts `conn` param for testability |
| `src/clients/python/strategies/base_strategy.py` | get_parameters() method on BaseStrategy | VERIFIED | Lines 68-74: concrete `get_parameters()` returning `{}` by default |
| `src/clients/python/strategies/mean_reversion.py` | Override get_parameters() with tuning params | VERIFIED | Lines 163-175: returns all 9 tuning params (max_loss_pct, stop_percentile, total_shares_per_group, num_exit_tiers, htf_horizon, tier_spacing, stop_widen_on_exit, min_expected_profit, ev_model) |
| `src/clients/python/demos/demo_mean_reversion.py` | --save-to-db CLI flag and post-run persistence call | VERIFIED | Line 347: `--save-to-db` argparse flag; lines 579-607: full persistence block with SavePlayground RPC + save_backtest_run() |
| `src/clients/python/tests/test_persistence.py` | Unit tests for persistence logic | VERIFIED | 4 tests — all pass: test_save_backtest_run_inserts_correct_row, test_save_backtest_run_queries_v_playground_stats, test_parameters_serialized_as_json, test_save_backtest_run_handles_no_stats |
| `infra/provision-metabase.py` | Strategy Comparison dashboard (Dashboard 4) with 5 cards | VERIFIED | build_strategy_comparison_cards() creates 5 cards; assemble_strategy_comparison() lays them out; main() calls both; summary prints "4 dashboards" |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `demo_mean_reversion.py` | `engine/persistence.py` | `save_backtest_run()` call | VERIFIED | Line 590: `from engine.persistence import save_backtest_run`; called at line 595 |
| `engine/persistence.py` | `backtest_runs` table | `INSERT INTO backtest_runs` | VERIFIED | Lines 86-108: INSERT with all required columns |
| `engine/persistence.py` | `v_playground_stats` view | SELECT before INSERT | VERIFIED | Lines 73-79: SELECT total_trades, total_pnl, win_rate, profit_factor WHERE playground_id = %s |
| `provision-metabase.py` | `backtest_runs` table | SQL queries in 5 cards | VERIFIED | All 5 strategy comparison card SQLs reference `backtest_runs br` |
| `provision-metabase.py` | `equity_plot_records` table | JOIN for equity curve overlay | VERIFIED | `Comparison: Equity Curves` card: `JOIN backtest_runs br ON br.playground_id = e.playground_session_id` |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `persistence.py` | stats (win_rate, profit_factor, etc.) | SELECT from v_playground_stats | Yes — live DB query | FLOWING |
| `persistence.py` | row_id | INSERT ... RETURNING id | Yes — real INSERT | FLOWING |
| `demo_mean_reversion.py` | params | strategy.get_parameters() + args.model | Yes — from strategy instance attrs | FLOWING |
| `provision-metabase.py` | Dashboard card data | Metabase queries backtest_runs at display time | Yes — SQL queries against real tables | FLOWING (requires live Metabase to confirm rendering) |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| persistence.py unit tests pass | `pytest tests/test_persistence.py -x -v` | 4 passed in 0.11s | PASS |
| provision-metabase.py parses without syntax errors | `python3 -c "import ast; ast.parse(...)"` | "syntax ok" | PASS |
| `--save-to-db` flag defined in demo | `grep "save-to-db" demo_mean_reversion.py` | Found at line 347 | PASS |
| `save_backtest_run` imported in demo | `grep "save_backtest_run" demo_mean_reversion.py` | Found at line 590 | PASS |
| `build_strategy_comparison_cards` in provision script | `grep "build_strategy_comparison_cards" provision-metabase.py` | Found at lines 524, 671 | PASS |
| "4 dashboards" in summary output | `grep "4 dashboards" provision-metabase.py` | Found at line 677 | PASS |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PERSIST-01 | 15-01 | Simulator playgrounds persist order_records and trade_records to Postgres on completion | SATISFIED | SavePlayground RPC called in demo (lines 580-587) when --save-to-db passed; REQUIREMENTS.md already marks [x] |
| PERSIST-02 | 15-01 | backtest_runs summary table (final_balance, win_rate, profit_factor, parameters JSONB) | SATISFIED | analytics-schema.sql lines 278-300; all D-02 columns present including parameters JSONB; REQUIREMENTS.md already marks [x] |
| DASH-03 | 15-02 | Strategy comparison dashboard: compare backtests by parameters and strategy types | SATISFIED (code) / NEEDS HUMAN (UI) | provision-metabase.py has full Dashboard 4 implementation; REQUIREMENTS.md marks [ ] (not yet checked off — needs human UI verification to close) |

**Note on DASH-03:** The REQUIREMENTS.md still shows DASH-03 as unchecked (`[ ]`), while PERSIST-01 and PERSIST-02 are checked. The DASH-03 code is fully implemented and wired, but the requirement checkbox should be updated to `[x]` after human verification of the Metabase dashboard appearance.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No stubs, placeholders, or empty implementations found. All data paths are wired end-to-end.

---

### Human Verification Required

#### 1. Strategy Comparison Dashboard in Metabase

**Test:** Run `python infra/provision-metabase.py --password <pw>` against the local/production Metabase instance, then open the "Strategy Comparison" dashboard.

**Expected:**
- Dashboard 4 titled "Strategy Comparison" exists alongside the existing 3 dashboards
- Dashboard shows 5 cards: Backtest Runs table, Equity Curves (line chart), Parameter Values table, Best Return scalar, Worst Return scalar
- A "Strategy" filter chip is visible and correctly scopes all 5 cards when a strategy name is entered
- Existing dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) still work correctly

**Why human:** The Python code is syntactically correct and all Metabase API calls are structurally sound, but the actual UI rendering of the filter bindings, card layout, and equity curve grouping-by-client_id can only be confirmed by running the provisioning script against a live Metabase instance.

---

### Gaps Summary

No automated gaps. All 7 truths verified at all levels (exists, substantive, wired, data-flowing). The single human verification item (DASH-03 UI appearance in Metabase) does not block the automated verdict — the code is complete and correct.

The REQUIREMENTS.md checkbox for DASH-03 remains unchecked (`[ ]`). After human verification confirms the Metabase dashboard works correctly, REQUIREMENTS.md should be updated to `[x] **DASH-03**`.

---

_Verified: 2026-03-30T17:00:00Z_
_Verifier: Claude (gsd-verifier)_
