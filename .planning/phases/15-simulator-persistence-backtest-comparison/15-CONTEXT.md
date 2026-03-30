# Phase 15: Simulator Persistence & Backtest Comparison - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Persist simulator backtest results to Postgres (opt-in via flag) and build a Metabase comparison dashboard for side-by-side strategy analysis. Adds a `backtest_runs` summary table and extends the provisioning script with a fourth dashboard.

Requirements: PERSIST-01, PERSIST-02, DASH-03

</domain>

<decisions>
## Implementation Decisions

### D-01: Persistence trigger
- **Explicit `--save-to-db` flag** on the Python client CLI
- Only persist when the flag is passed — keeps DB clean during rapid iteration
- Calls the existing `SavePlayground` RPC on completion, which already persists order_records and trade_records
- No changes needed to the Go server's save logic for PERSIST-01 — the RPC exists, just not called for simulators

### D-02: backtest_runs table design
- **client_id as the primary human-readable identifier** (e.g. `mean-reversion-aapl-2026-03-30`)
- Table columns: playground_id (FK), client_id, strategy_name, parameters (JSONB), final_balance, starting_balance, win_rate, profit_factor, total_pnl, total_trades, start_date, end_date, created_at
- Populated by the Python client after backtest completion (compute metrics from playground state, then insert)

### D-03: Comparison dashboard (DASH-03)
- **Table + overlay charts**: sortable table of all backtest runs with key metrics
- Overlay equity curves for selected runs (multi-select playground filter)
- Filter by strategy type and date range
- Added to `infra/provision-metabase.py` following the existing dashboard-as-code pattern

### D-04: Parameter capture
- **JSONB column** (`parameters`) in backtest_runs
- Python client serializes strategy constructor args to JSON
- Flexible — new strategies don't require schema changes
- Metabase can filter/group by JSONB keys for parameter comparison

### Claude's Discretion
- Whether backtest_runs is populated via a new RPC or Python-side SQL insert
- Exact schema for backtest_runs (additional columns beyond the decided ones)
- Dashboard layout and card arrangement for DASH-03
- How to serialize strategy parameters (which args to include/exclude)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Persistence
- `src/go/backtester-api/router/grpc.go` — `SavePlayground` RPC handler (line ~674)
- `src/go/data/database_service.go` — `SavePlayground`, `SavePlaygroundSession` methods
- `src/go/data/in_memory.go` — `savePlaygroundTx`, `saveBalance`, `saveEquityPlotRecords`
- `src/go/playground.proto` — `SavePlaygroundRequest` message definition

### Python Client
- `src/clients/python/engine/client.py` — `BacktesterPlaygroundClient`, `PlaygroundEnvironment.SIMULATOR`
- `src/clients/python/engine/trading_engine.py` — `run_engine()` main loop
- `src/clients/python/demos/demo_mean_reversion.py` — CLI launcher for mean reversion strategy

### Dashboard Provisioning
- `infra/provision-metabase.py` — Existing dashboard-as-code script (extend with DASH-03)
- `infra/analytics-schema.sql` — SQL views and indexes (extend with backtest_runs table)

### Prior Phase Artifacts
- `.planning/phases/14-core-performance-dashboards/14-CONTEXT.md` — Dashboard-as-code decision
- `.planning/phases/13-analytics-schema-indexes/13-01-SUMMARY.md` — SQL views pattern

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `SavePlayground` RPC already persists playground + orders + trades to Postgres
- `v_playground_stats` view already computes P&L, win rate, profit factor per playground
- `provision-metabase.py` pattern for programmatic dashboard creation
- `BacktesterPlaygroundClient` already has `self.client_id` and `self.environment`

### Established Patterns
- Idempotent SQL scripts in `infra/` directory
- GORM auto-migration for Go models
- Python `argparse` for CLI flags in demo scripts
- JSONB columns used elsewhere (order_records.attributes)

### Integration Points
- Python calls `SavePlayground` RPC after tick loop completes
- backtest_runs table populated after save (Python computes metrics from playground state)
- New dashboard added to provision-metabase.py
- backtest_runs table created via analytics-schema.sql extension

</code_context>

<specifics>
## Specific Ideas

- Multi-select in comparison dashboard for overlaying equity curves of multiple runs
- Strategy name should match the `label` attribute added in the heartbeat fix (e.g. "mean_reversion", "covered_call")
- Parameters JSONB should capture the most impactful tuning knobs per strategy

</specifics>

<deferred>
## Deferred Ideas

- Automated backtest comparison reports (e.g. "best 5 runs for AAPL") — future enhancement
- Backtest scheduling/automation — out of scope
- Parameter optimization dashboard — could be Phase 16+ addition

</deferred>

---

*Phase: 15-simulator-persistence-backtest-comparison*
*Context gathered: 2026-03-30*
