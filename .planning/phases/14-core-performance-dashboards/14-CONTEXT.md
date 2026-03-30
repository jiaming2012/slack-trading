# Phase 14: Core Performance Dashboards - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Build Metabase dashboards that let the operator select any playground and see trading performance -- P&L, win rate, profit factor, slippage, and per-symbol breakdown. Dashboards are provisioned programmatically via the Metabase API for reproducibility.

Requirements: DASH-01, DASH-02, DASH-04

</domain>

<decisions>
## Implementation Decisions

### D-01: Dashboard structure
- **3 separate dashboards**, not one combined mega-dashboard:
  1. **Trading Performance** — P&L over time, win rate, profit factor, gross profit/loss (DASH-01)
  2. **Slippage Analysis** — open/close/total slippage per trade, per playground (DASH-02)
  3. **Portfolio Analytics** — position history, per-symbol/asset-class breakdown (DASH-04)
- Each dashboard focused, loads faster, easier to bookmark specific views

### D-02: Playground selector / filters
- Primary filter: **client_id** (human-readable), fallback to **playground_id** when client_id is null
- Secondary filters: **environment** (live/simulator), **tags**
- **Multi-select supported** — user can select multiple playgrounds to see combined stats
- Filter pattern consistent across all 3 dashboards

### D-03: Dashboard-as-code via Metabase API
- **Python script**: `infra/provision-metabase.py` creates all questions, dashboards, and filters programmatically via Metabase REST API
- Fully reproducible — run on fresh Metabase to rebuild everything from scratch
- Version-controlled alongside `infra/analytics-schema.sql`
- Re-runnable to update dashboard definitions
- This was deferred from Phase 12 — now resolved

### D-04: Equity curve visualization
- **Line chart with drawdown overlay**: equity over time as a line, shaded drawdown from peak
- Horizontal reference line at starting_balance for context
- Shows both returns and risk in one view

### D-05: Maintenance strategy
- SQL views abstract the raw schema — dashboards query views, not tables directly
- API provisioning script means dashboards are reproducible (no manual rebuild)
- `analytics-schema.sql` is idempotent — safe to re-run after GORM schema changes
- If GORM model changes break a view, re-run analytics-schema.sql to update

### Claude's Discretion
- Exact Metabase question types (native SQL vs simple questions vs custom questions)
- Chart types for non-equity visualizations (bar, table, number cards)
- Dashboard layout and card sizing
- Filter widget types (dropdown vs search vs multi-select)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### SQL Views (data layer)
- `infra/analytics-schema.sql` — All composite indexes and SQL views (v_trade_fills, v_order_pnl, v_playground_stats, v_open_slippage)
- `infra/init-metabase.sql` — metabaseappdb creation and metabase_ro role setup

### Reference Implementation
- `src/clients/python/tools/playground_metrics.py` — P&L, win rate, profit factor calculations (Python reference for validation)

### Domain Models
- `src/go/backtester-api/models/order_record.go` — CalcRealizedPL() logic (authoritative P&L source)
- `src/go/backtester-api/models/equity_plot_record.go` — Equity curve data model

### Prior Phase Artifacts
- `.planning/phases/13-analytics-schema-indexes/13-01-SUMMARY.md` — What was built in Phase 13
- `.planning/phases/12-deploy-metabase-harden-infrastructure/12-01-SUMMARY.md` — Metabase infrastructure setup

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- 4 SQL views deployed to production: v_playground_stats, v_order_pnl, v_trade_fills, v_open_slippage
- Metabase running at 192.168.8.164:3001 with playground DB connected via metabase_ro
- Metabase API accessible at http://192.168.8.164:3001/api/ (session auth with jac475@cornell.edu)

### Established Patterns
- Idempotent SQL scripts in `infra/` directory
- Python scripts for infrastructure tooling (grodt conda env)
- Metabase admin settings already configured: join tables hidden, JSON unfolding off, re-fingerprinting off

### Integration Points
- Metabase API for programmatic dashboard creation
- playground_sessions.client_id as the primary human-readable playground identifier
- equity_plot_records table for equity curve time series
- v_playground_stats already aggregates P&L, win rate, profit factor per playground

</code_context>

<specifics>
## Specific Ideas

- Multi-playground selection for combined stats across playgrounds
- Drawdown overlay on equity curve (not just the line)
- Provisioning script should be re-runnable (update existing dashboards, not create duplicates)

</specifics>

<deferred>
## Deferred Ideas

- Strategy comparison dashboard (DASH-03) — belongs in Phase 15 (requires backtest persistence)
- Spread analytics dashboard (DASH-05) — belongs in Phase 16 (requires spread schema)
- Per-asset-class split in v_playground_stats (stock vs option) — noted in Phase 13 verification, can be addressed in Phase 14 dashboards via GROUP BY class

</deferred>

---

*Phase: 14-core-performance-dashboards*
*Context gathered: 2026-03-30*
