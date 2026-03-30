# Project Research Summary

**Project:** slack-trading (grodt) — Metabase Analytics (v2.0)
**Domain:** Business analytics layer for a Go+Python trading platform using Metabase on PostgreSQL
**Researched:** 2026-03-29
**Confidence:** HIGH

## Executive Summary

This milestone adds a business-level analytics layer on top of the already-shipped v1.1 observability stack (OTel + Grafana + Loki). The core addition is Metabase OSS (v0.59.4), connected read-only to the existing PostgreSQL `playground` database, answering questions about P&L, win rate, profit factor, and strategy comparison. Grafana remains the operational tool ("is it alive?"); Metabase becomes the business analytics tool ("did this strategy make money?"). No new data pipeline infrastructure is needed — Metabase queries the tables that GORM already manages. The only new code is Metabase's Docker service entry, a handful of SQL views and tables in PostgreSQL, and two Go server write paths.

The biggest architectural decision is spread-aware P&L grouping. Option strategies like covered calls involve 2+ legs stored as individual `order_records` rows. A naive SQL aggregation produces meaningless per-leg numbers. The recommended approach is a tagging convention on order placement (`spread_group_key` in the existing JSONB `Attributes` field) paired with two new tables (`spread_groups`, `spread_group_legs`) that explicitly model strategy grouping. Additionally, a `backtest_runs` companion table captures per-run summary statistics so Metabase can compare strategies without querying millions of raw rows. These schema additions are the load-bearing work of this milestone. All four research documents converge on this conclusion independently.

The primary risk is memory pressure on the existing 4 GB DigitalOcean droplet. Metabase's JVM will OOM the server if unconstrained — setting `JAVA_OPTS=-Xmx768m` and a Docker `mem_limit: 1.5g` is non-negotiable before any other work proceeds. A secondary risk is Metabase's automatic schema sync scanning live OLTP tables during market hours; this is mitigated by disabling re-fingerprinting and adding composite indexes on `(playground_id, timestamp)` before connecting the trading database.

## Key Findings

### Recommended Stack

No new programming languages, frameworks, or data pipelines are needed. Metabase is a self-contained Docker service that connects directly to PostgreSQL. The existing `playground` database and its GORM-managed tables are the data source. The only new infrastructure is the Metabase container itself plus a dedicated `metabase` application database in the same PostgreSQL instance.

**Core technologies:**
- `metabase/metabase:v0.59.4` Docker image: Business analytics UI on port 3001, configured with `MB_JETTY_PORT=3001` to avoid collision with Grafana on 3000
- PostgreSQL `analytics` schema + SQL views: Stable named query surfaces that insulate Metabase dashboards from raw GORM table changes
- New tables `spread_groups`, `spread_group_legs`, `backtest_runs`: Schema additions to make spread analytics and strategy comparison tractable in SQL without complex ad-hoc joins
- `JAVA_OPTS=-Xmx768m -Xms256m`: JVM heap constraint — required to keep the 4 GB droplet stable alongside existing services

**Key version and config requirements:**
- Metabase v0.59.4 confirmed latest stable (March 2025 release)
- Port mapping: `3001:3000` — do NOT reassign Grafana/otel-lgtm from port 3000
- `MB_DB_TYPE=postgres`, `MB_DB_DBNAME=metabase` — dedicated app DB, never share the `playground` DB
- Postgres 13 is fully supported by Metabase v0.59

### Expected Features

**Must have (table stakes):**
- Total realized P&L per playground — core performance number
- Win rate (winners / total closed trades) — fundamental metric
- Profit factor (gross_profit / abs(gross_loss)) — industry standard; >1.5 = healthy
- Equity curve chart — `equity_plot_records` is already populated for live/reconcile playgrounds
- Trade count by symbol — GROUP BY symbol, class
- Playground filter (dropdown) and date range filter — Metabase `{{variable}}` syntax
- Slippage summary (requested vs fill price) — validates simulation fill assumptions

**Should have (differentiators):**
- Backtest comparison table — multiple runs side-by-side; requires simulator persistence + `backtest_runs` table
- Spread-aware P&L (multi-leg grouping) — covered call round-trip as a single trade unit; requires `spread_groups` tables and `v_spread_pnl` view
- Per-symbol P&L breakdown — GROUP BY symbol JOIN playground
- Strategy comparison by tags — GROUP BY unnested `tags` array
- Rejection rate analysis — reveals live sim config issues

**Defer (post-MVP):**
- Sharpe ratio — requires daily-bucketed equity returns; high SQL complexity, low urgency for MVP
- Trade duration histogram — nice-to-have; deliver after core metrics are validated
- Expected value vs realized P&L correlation — requires JSONB casting and statistical interpretation

**Explicit anti-features (do not build in Metabase):**
- Real-time live order monitoring — Grafana owns this
- Greeks aggregation — requires live market prices; Metabase queries Postgres, not live data
- Intraday candlestick charts — wrong tool; Grafana handles tick-level charts
- Alerts/notifications — Grafana unified alerting; do not duplicate in Metabase OSS
- Portfolio mark-to-market P&L — requires real-time price lookups

### Architecture Approach

Metabase is a pure read-only analytics layer. It connects to PostgreSQL via a dedicated `metabase_ro` user, writes nothing to application tables, and has no event bus or RPC surface. The architecture separates two credential concerns on the same Postgres instance: Metabase's own application state in a `metabase` database (owner: `metabase_app` user), and trading data queries via a read-only user against the `playground` database. The Go server gains two new write paths: spread group registration on `PlaceOrder` (reads `spread_group_key` from `Attributes`, upserts `spread_groups`/`spread_group_legs`) and backtest result persistence when `isBacktestComplete = true` (writes to `backtest_runs`).

**Major components:**
1. **Metabase service** — Docker container on port 3001, JVM heap-capped at 768m, PostgreSQL-backed app state; no code changes to Go or Python to deploy
2. **PostgreSQL analytics schema** — New tables (`spread_groups`, `spread_group_legs`, `backtest_runs`) and views (`v_trade_fills`, `v_spread_pnl`, `v_backtest_summary`, `option_legs`) — the stable, named query surface for all Metabase dashboards
3. **Go server spread registration** — New logic in `PlaceOrder` handler: reads `spread_group_key` from `Attributes` JSONB, upserts `spread_groups`, inserts `spread_group_legs`
4. **Go server backtest persistence** — New logic triggered by `isBacktestComplete = true`: computes summary stats and writes to `backtest_runs`; also calls `SavePlaygroundSession` for simulator playgrounds when `save_to_db = true`
5. **Python strategy tagging** — Convention: pass `spread_group_key` and `leg_role` in `Attributes` on each order placement; tag playground runs with `["simulator", "strategy:covered_call", "version:v7"]` on `CreatePlayground`

**Memory budget (4 GB droplet):**

| Component | Resident |
|-----------|---------|
| OS + Docker daemon | ~400 MB |
| EventStoreDB | ~300 MB |
| PostgreSQL 13 | ~200 MB |
| otel-lgtm (Grafana+Loki+Tempo+Prometheus+Collector) | ~700 MB |
| grodt (Go server) | ~150 MB |
| Metabase (JVM capped at -Xmx768m) | ~900 MB |
| Buffer | ~350 MB |
| **Total** | ~3.0 GB |

A 2 GB swap file on the droplet (documented in Phase 07 deployment steps) provides a safety net for peak bursts. Upgrade to s-2vcpu-8gb if instability occurs.

### Critical Pitfalls

1. **JVM OOM kills the trading server (CRITICAL)** — Metabase's default JVM grabs 25-50% of host RAM. On a 4 GB droplet, this causes OOM kills of the Go server or EventStoreDB during Metabase startup fingerprinting. Prevention: set `JAVA_OPTS=-Xmx768m -Xms256m` AND `mem_limit: 1.5g` in Docker Compose before first boot. Monitor `docker stats` during startup.

2. **H2 app database loses all dashboards on container restart (CRITICAL)** — Metabase's default embedded H2 database is wiped on any container recreation. Prevention: use `MB_DB_TYPE=postgres` and a dedicated `metabase` database from day one. This is non-negotiable; never run with H2 beyond throwaway local testing.

3. **Schema sync locks OLTP tables during market hours (CRITICAL)** — Metabase's automatic fingerprinting runs `SELECT * FROM order_records LIMIT 10000` on every column, including the JSONB `attributes` column. Full sequential scan on a live trading table causes Twirp RPC latency spikes. Prevention: disable "Periodically refingerprint tables" in Metabase Admin; hide join tables (`order_closes`, `trade_closed_by`, `order_reconciles`); add composite index `order_records(playground_id, timestamp)` before connecting the trading DB.

4. **Spread P&L double-counts multi-leg trades (HIGH)** — Options spreads are stored as individual `order_records` rows. A naive `SUM(quantity * price)` produces meaningless per-leg P&L or double-counts round-trips. Prevention: create `spread_groups`/`spread_group_legs` tables; build all P&L questions against the `v_spread_pnl` view, never raw `order_records`.

5. **Partial fill creates phantom realized P&L (HIGH)** — `TradeRecord` rows represent fills, not complete outcomes. P&L queries over partially-filled orders misrepresent performance. Prevention: filter all P&L queries to `order_records.status = 'filled'` only.

6. **Connection pool saturation (MODERATE)** — Metabase opens 15 connections per database (30 total for two DBs). Combined with GORM pool, this approaches Postgres default `max_connections = 100`. Prevention: set `max_connections = 200` in Postgres config; set `MB_APPLICATION_DB_MAX_CONNECTION_POOL_SIZE=5` and `MB_DB_CONNECTION_POOL_SIZE=8` in Metabase environment.

## Implications for Roadmap

All four research files converge on the same build order. The dependencies are structural: Metabase won't start without its app DB; dashboards require Metabase deployed; spread analytics require schema tables to exist; backtest comparison requires data in those tables. There is no viable alternative ordering.

### Phase 1: Deploy Metabase and Harden Infrastructure

**Rationale:** Everything else is blocked until Metabase runs and infrastructure pitfalls are neutralized. JVM OOM, H2 data loss, and connection pool saturation must be addressed before any configuration work — mistakes here destroy dashboard work built on top.
**Delivers:** Metabase accessible on port 3001, backed by PostgreSQL app DB, JVM-capped, Postgres credentials isolated. Trading server demonstrably unaffected.
**Addresses:** Foundation for all dashboard and analytics work
**Avoids:** B1 (JVM OOM), B2 (app DB isolation), B8 (H2 data loss), B9 (connection pool exhaustion)
**Key tasks:**
- Create `metabase` database and `metabase_app` user in Postgres (via `infra/init.sql`)
- Create `metabase_ro` read-only user with SELECT on `playground` database
- Add `metabase` service to `docker-compose.prod.yaml` with `JAVA_OPTS=-Xmx768m -Xms256m`, `mem_limit: 1.5g`, port `3001:3000`
- Update `.env.prod.template` with `METABASE_APP_DB`, `METABASE_DB_USER`, `METABASE_DB_PASS`, `METABASE_RO_PASS`
- Set `max_connections = 200` in Postgres config
- Open TCP 3001 inbound in DigitalOcean Cloud Firewall
- Validate with `docker stats` during first boot; confirm Go server latency unaffected

### Phase 2: Analytics Schema and DB Connection

**Rationale:** Analytics views and composite indexes must exist before Metabase queries the trading database. Connecting Metabase to an unindexed `order_records` table triggers the schema sync pitfall (B3).
**Delivers:** PostgreSQL analytics schema with composite indexes, option symbol parsing view, and Metabase table visibility rules configured.
**Uses:** Plain SQL migration files — GORM ignores non-public schemas, so no GORM migration needed
**Avoids:** B3 (schema sync locks), B7 (backtest row volume query lag), B11 (OCC symbol not parseable)
**Key tasks:**
- Create `analytics` schema in `playground` database
- Add composite indexes: `order_records(playground_id, timestamp)`, `trade_records(order_id, timestamp)`, `equity_plot_records(playground_id, timestamp)`
- Create `option_legs` view (parses OCC symbol into `underlying`, `expiry_date`, `option_type`, `strike_price`)
- Create `v_trade_fills` view (flattened order+trade+playground join)
- Connect trading DB in Metabase using `metabase_ro` credentials
- In Metabase Admin: disable re-fingerprinting; hide join tables; disable JSON unfolding for `attributes` column

### Phase 3: Core Performance Dashboard

**Rationale:** Core metrics are the minimum viable product and can be built from data that already exists. No Go or Python changes required. This phase validates that the pipeline produces correct numbers before adding complexity.
**Delivers:** Single-playground performance dashboard: total P&L, win rate, profit factor, average win/loss, trade count by symbol, equity curve, slippage summary. Playground filter dropdown.
**Addresses:** All table-stakes features from FEATURES.md
**Avoids:** B5 (partial fill phantom P&L — filter to `status='filled'`)
**Key tasks:**
- Build `v_pnl_per_trade` view (uses `order_closes` M2M to pair opening/closing orders)
- Build core performance Metabase dashboard with `{{playground_id}}` variable filter
- Equity curve chart from `equity_plot_records`
- Slippage analysis chart (requested_price vs fill price per side)
- Validate all numbers against known output from `src/clients/python/tools/playground_metrics.py` for the same playground

### Phase 4: Simulator Persistence and Backtest Comparison

**Rationale:** Strategy comparison requires backtest results persisted to Postgres. Currently, simulator playgrounds write orders in memory only — `CreatePlayground` for `PlaygroundEnvironmentSimulator` does NOT call `SavePlaygroundSession`. This phase makes backtests queryable.
**Delivers:** Simulator playgrounds persisted to Postgres on request; `backtest_runs` table populated on completion; multi-run comparison dashboard in Metabase.
**Addresses:** Backtest comparison table (key differentiator feature)
**Key tasks:**
- Add `backtest_runs` table (per schema in ARCHITECTURE.md)
- Modify `database_service.go` `CreatePlayground` simulator branch (~line 927): call `SavePlaygroundSession` when `req.SaveToDB == true`
- Add Go server logic to write `backtest_runs` when `isBacktestComplete = true` (compute final_balance, win_rate, profit_factor, max_drawdown from order/trade records)
- Update Python `CreatePlayground` call to pass `save_to_db: true` with tags `["simulator", "strategy:covered_call", "version:v7"]`
- Create `v_backtest_summary` view (joins `backtest_runs` + `playgrounds`)
- Build backtest comparison table in Metabase

### Phase 5: Spread-Aware Analytics

**Rationale:** Multi-leg spread P&L is the highest-risk domain feature. It requires schema changes, Go server logic, Python convention changes, and SQL view design. Building it last means it can be validated against the correct P&L baselines established in Phase 3.
**Delivers:** Spread-grouped P&L — covered call round-trips shown as single trade units with combined P&L; `v_spread_pnl` dashboard in Metabase.
**Addresses:** Spread-aware P&L (highest-value differentiator feature)
**Avoids:** B4 (multi-leg spread double-counting), B6 (early assignment breaks attribution)
**Key tasks:**
- Add `spread_groups` and `spread_group_legs` tables (per schema in ARCHITECTURE.md)
- Add Go server logic in `PlaceOrder` handler: read `spread_group_key` from `Attributes`, upsert `spread_groups`, insert `spread_group_legs`
- Update Python covered call strategy to pass `spread_group_key` (e.g., `"AAPL-covered-call-2024-01-19"`) and `leg_role` in order `Attributes`
- Create `calc_pnl` SQL function mirroring Go's `CalcRealizedPL()` traversal of `order_closes`/`trade_closed_by` M2M
- Create `v_spread_pnl` view
- Validate against a known covered call backtest, including early assignment edge case (B6)
- Build spread P&L Metabase dashboard

### Phase Ordering Rationale

- **Infrastructure before schema:** JVM and DB isolation pitfalls (B1, B2, B8) corrupt the environment permanently if encountered after dashboards are built. They must be neutralized first.
- **Schema before DB connection:** Indexes and table visibility rules must exist before Metabase fingerprints the trading DB (B3, B7). Adding them retroactively requires locking during an index build.
- **Core metrics before spread:** Core metrics validate data quality and establish P&L baselines. Spread analytics build on that foundation — if core P&L is wrong, spread P&L is also wrong.
- **Simulator persistence before comparison:** Strategy comparison requires data in `backtest_runs`. That data requires the persistence path to exist.
- **Spread last:** Highest complexity, most Go server changes, most Python coordination. Delivers the highest-value differentiator but requires stable infrastructure and verified metrics to validate against.
- **Each phase is independently valuable:** Stopping after Phase 3 still gives a fully functional P&L dashboard. Stopping after Phase 4 adds strategy comparison. Phase 5 adds the advanced spread analytics.

### Research Flags

Phases with well-documented patterns (skip research-phase during planning):
- **Phase 1:** Standard Metabase Docker deployment. All env vars verified in official docs. No speculation required.
- **Phase 2:** Standard PostgreSQL schema migration and index creation. Metabase Admin settings are documented. CREATE INDEX CONCURRENTLY is the standard pattern.
- **Phase 3:** Straightforward SQL aggregations on existing schema. FEATURES.md contains the exact formulas. `playground_metrics.py` is the reference implementation.

Phases that benefit from deeper research during planning:
- **Phase 4 (Simulator Persistence):** The `CreatePlayground` simulator branch is understood from codebase inspection, but verify whether `equity_plot_records` are written incrementally or only on completion. If only on completion and a backtest is interrupted, the equity curve will be missing. Check `database_service.go` `SaveEquityPlot` call sites before writing the task list.
- **Phase 5 (Spread Analytics):** The `calc_pnl` SQL function must accurately mirror Go's `CalcRealizedPL()` traversal of the `ClosedBy` M2M relationship. Read `order_record.go:CalcRealizedPL()` and the `order_closes` join table structure before writing the view. Early assignment (B6) needs a concrete test fixture to validate.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Metabase v0.59.4 verified via GitHub releases. All Docker env vars confirmed in official Metabase docs. Port conflict with Grafana is a concrete constraint from direct `docker-compose.prod.yaml` inspection. Memory numbers are community-sourced (exact values MEDIUM, mitigation strategy HIGH). |
| Features | HIGH | Schema columns verified by direct codebase inspection (`order_record.go`, `trade_record.go`, `equity_plot_record.go`, `playground_meta.go`). Feature prioritization based on `playground_metrics.py` as the authoritative source of in-use metric formulas. |
| Architecture | HIGH | Build order dependencies are structural — Metabase won't start without app DB; schema tables must exist before Go server can write to them. SQL view correctness for spread P&L is MEDIUM — needs validation against known backtest output before Phase 5 ships. |
| Pitfalls | HIGH | JVM OOM, H2 data loss, and schema sync behavior are documented in official Metabase docs and confirmed GitHub issues. Spread double-counting derived from direct GORM model inspection. Connection pool math is deterministic. |

**Overall confidence:** HIGH

### Gaps to Address

- **Simulator equity_plot_records write timing:** Confirm whether `equity_plot_records` rows are written incrementally during a simulator run or only when the playground finalizes. If only on finalization and the run is interrupted, the equity curve will be absent. Check `database_service.go` `SaveEquityPlot` call sites before Phase 4 task planning.
- **`CalcRealizedPL()` SQL translation accuracy:** The Go function traverses `ClosedBy` using an in-memory object graph. The SQL equivalent joins `order_closes` and `trade_closed_by`. Verify the traversal order matches before writing the `calc_pnl` SQL function — test against a playground with known P&L values including a partial close.
- **Metabase JSONB unfolding behavior:** If Metabase auto-unfolds the `attributes` JSONB column during schema sync, it generates hundreds of virtual columns that pollute the field picker. Confirm the field-level setting to disable JSON unfolding is applied during Phase 2 DB connection setup before building any questions.
- **Existing simulator playground backfill:** The 37 playgrounds currently in the DB have no `backtest_runs` entries and were likely created without `save_to_db = true`. Decide whether to backfill (one-time migration computing metrics from existing `order_records`) or start fresh from Phase 4 forward. Backfill is not required for MVP but may be wanted for historical comparisons.

## Sources

### Primary (HIGH confidence)
- [Metabase GitHub Releases](https://github.com/metabase/metabase/releases) — v0.59.4 latest stable (March 2025)
- [Metabase: Configuring Application Database](https://www.metabase.com/docs/latest/installation-and-operation/configuring-application-database) — MB_DB_* env vars, H2 vs Postgres app DB
- [Metabase: Customizing Jetty Webserver](https://www.metabase.com/docs/latest/configuring-metabase/customizing-jetty-webserver) — MB_JETTY_PORT for port remapping
- [Metabase: Running on Docker](https://www.metabase.com/docs/latest/installation-and-operation/running-metabase-on-docker) — container setup, JAVA_OPTS
- [Metabase: Sync and Scan docs](https://www.metabase.com/docs/latest/databases/sync-scan) — fingerprinting behavior, how to disable re-fingerprinting
- [Metabase: JVM troubleshooting](https://www.metabase.com/docs/latest/troubleshooting-guide/running) — -Xmx configuration, sawtooth GC pattern
- [Metabase: PostgreSQL connection](https://www.metabase.com/docs/latest/databases/connections/postgresql) — connection pool defaults (15 per DB)
- [Metabase: Caching docs](https://www.metabase.com/docs/latest/configuring-metabase/caching) — OSS caching limitations, TTL behavior
- [PostgreSQL: Schemas](https://www.postgresql.org/docs/current/ddl-schemas.html) — `analytics` schema isolation pattern
- Codebase: `src/go/data/database_service.go` lines 821-971 — CreatePlayground simulator branch confirmed does NOT call SavePlaygroundSession (direct inspection)
- Codebase: `src/go/backtester-api/models/order_record.go` — `tag`, `attributes` JSONB, `CalcRealizedPL()` logic (direct inspection)
- Codebase: `src/go/backtester-api/models/playground_meta.go` — Meta struct, `Environment`, `Tags`, `StartingBalance` (direct inspection)
- Codebase: `docker-compose.prod.yaml` — current 4-service production stack, existing port assignments (direct inspection)
- Codebase: `src/clients/python/tools/playground_metrics.py` — authoritative source of metric formulas currently in use (direct inspection)

### Secondary (MEDIUM confidence)
- [Metabase minimum resources community forum](https://discourse.metabase.com/t/what-are-metabase-minimum-resources/21470) — ~600MB idle baseline (multiple sources agree)
- [Metabase RAM usage discussion](https://discourse.metabase.com/t/metabase-memory-usage-on-docker/25583) — 2.5 GB available sufficient for 4 GB server (community reports)
- [GitHub: base memory increasing after sync #12060](https://github.com/metabase/metabase/issues/12060) — confirmed heap growth during schema sync
- [GitHub: long startup due to sync #19604](https://github.com/metabase/metabase/issues/19604) — startup time with multiple databases
- [LuxAlgo: Top 7 backtesting metrics](https://www.luxalgo.com/blog/top-7-metrics-for-backtesting-results/) — metric definitions and industry benchmarks
- [TradesViz: Multi-leg options journaling](https://www.tradesviz.com/how-to-journal-vertical-spreads/) — spread grouping patterns for journaling tools
- [Profit Factor definition and benchmarks](https://www.backtestbase.com/education/win-rate-vs-profit-factor) — >1.5 threshold sourced here

### Tertiary (LOW confidence — needs validation during implementation)
- [Assignment risk on limited-risk spreads — TradeStation](https://www.tradestation.com/learn/market-basics/options/understanding-the-risks/assignment-risk-on-limited-risk-options-spreads/) — early assignment edge case behavior; needs a concrete test fixture to verify against actual Go broker code

---
*Research completed: 2026-03-29*
*Ready for roadmap: yes*
