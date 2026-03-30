# Roadmap: Grodt Trading Platform

## Milestones

- **v1.0 Live Simulation Observability** — Phases 1-7 (shipped 2026-03-28)
- **v1.1 Dashboard Enhancements** — Phases 8-11 (shipped 2026-03-30)
- **v2.0 Metabase Analytics** — Phases 12-16 (in progress)

## Phases

<details>
<summary>v1.0 Live Simulation Observability (Phases 1-7) — SHIPPED 2026-03-28</summary>

- [x] Phase 1: Python Codebase Restructure (4/4 plans)
- [x] Phase 2: Go OTel Foundation & Local Backend (3/3 plans)
- [x] Phase 3: Go Telemetry Instrumentation (3/3 plans)
- [x] Phase 4: Python Telemetry Instrumentation (3/3 plans)
- [x] Phase 5: End-to-End Tick Tracing (1/1 plan)
- [x] Phase 6: Dashboards & Alerts (2/2 plans)
- [x] Phase 7: Production Deployment (2/2 plans)

Full details: [milestones/v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md)

</details>

<details>
<summary>v1.1 Dashboard Enhancements (Phases 8-11) — SHIPPED 2026-03-30</summary>

- [x] Phase 8: Metric Labels & Bug Fix (1/1 plan)
- [x] Phase 9: Playground Filtering & Detail Panels (1/1 plan)
- [x] Phase 10: Signal/Candle Filtering & Python Heartbeat (1/1 plan)
- [x] Phase 11: Per-Strategy Dashboards (1/1 plan)

Full details: [milestones/v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md)

</details>

### v2.0 Metabase Analytics (In Progress)

**Milestone Goal:** Business-level trading analytics via Metabase -- P&L, strategy comparison, spread-aware multi-leg analysis, and backtest persistence. Grafana stays for real-time ops; Metabase answers "did this strategy make money?"

**Phase Numbering:**
- Integer phases (12, 13, 14): Planned milestone work
- Decimal phases (12.1, 12.2): Urgent insertions (marked with INSERTED)

- [ ] **Phase 12: Deploy Metabase & Harden Infrastructure** - Metabase running on user's desktop (Docker Desktop), backed by DO Postgres app DB, with read-only trading credentials and Cloud Firewall hardening
- [ ] **Phase 13: Analytics Schema & Indexes** - Composite indexes and SQL views for P&L, win rate, and profit factor on existing trading tables
- [ ] **Phase 14: Core Performance Dashboards** - Trading performance, slippage, and portfolio dashboards in Metabase using existing data
- [ ] **Phase 15: Simulator Persistence & Backtest Comparison** - Backtest results saved to Postgres with summary table and comparison dashboard
- [ ] **Phase 16: Spread Analytics** - Multi-leg option strategies grouped as single trades with spread-aware P&L dashboards

## Phase Details

### Phase 12: Deploy Metabase & Harden Infrastructure
**Goal**: Metabase is accessible at localhost:3001 on the user's desktop, backed by a dedicated Postgres app database on DO, with trading DB access isolated to a read-only user
**Depends on**: Nothing (first phase of v2.0)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):
  1. Operator can open Metabase UI at localhost:3001 from desktop browser
  2. Metabase container restart preserves all saved questions and dashboards (Postgres app DB, not H2)
  3. JVM stays under configured limit locally (mem_limit 1.5GB, -Xmx768m)
  4. Metabase can query trading tables but cannot INSERT, UPDATE, or DELETE any trading data
**Plans**: 1 plan

Plans:
- [ ] 12-01-PLAN.md — Local Metabase deployment with DB setup and infrastructure hardening

### Phase 13: Analytics Schema & Indexes
**Goal**: Trading database has composite indexes and SQL views that make P&L, win rate, and profit factor queryable without full table scans -- and Metabase is connected with safe sync settings
**Depends on**: Phase 12
**Requirements**: SCHEMA-01, SCHEMA-02
**Success Criteria** (what must be TRUE):
  1. Metabase questions using playground_id + timestamp filters hit indexes (no sequential scans on order_records during market hours)
  2. SQL views for P&L, win rate, and profit factor return correct values when compared against playground_metrics.py output for the same playground
  3. Metabase Admin shows join tables hidden, JSON unfolding disabled, and re-fingerprinting off
**Plans**: TBD

Plans:
- [ ] 13-01: TBD

### Phase 14: Core Performance Dashboards
**Goal**: Operator can select any playground and see its trading performance -- P&L, win rate, profit factor, slippage, and per-symbol breakdown -- all from Metabase
**Depends on**: Phase 13
**Requirements**: DASH-01, DASH-02, DASH-04
**Success Criteria** (what must be TRUE):
  1. Operator can select a playground from a dropdown and see total P&L, win rate, profit factor, and equity curve
  2. Operator can view open/close/total slippage per trade for any playground
  3. Operator can see position history and per-symbol P&L breakdown
  4. Dashboard numbers match playground_metrics.py output for the same playground within rounding tolerance
**Plans**: TBD
**UI hint**: yes

Plans:
- [ ] 14-01: TBD

### Phase 15: Simulator Persistence & Backtest Comparison
**Goal**: Backtest results are saved to Postgres so the operator can compare strategy runs side-by-side in Metabase
**Depends on**: Phase 14
**Requirements**: PERSIST-01, PERSIST-02, DASH-03
**Success Criteria** (what must be TRUE):
  1. Running a simulator backtest with save_to_db=true persists order_records and trade_records to Postgres
  2. Completed backtests appear in the backtest_runs table with final_balance, win_rate, profit_factor, and parameters
  3. Operator can compare multiple backtest runs side-by-side in Metabase, filtered by strategy type and parameter values
**Plans**: TBD

Plans:
- [ ] 15-01: TBD

### Phase 16: Spread Analytics
**Goal**: Multi-leg option strategies (covered calls, spreads) are grouped as single trade units with combined P&L, so the operator sees strategy-level performance instead of meaningless per-leg numbers
**Depends on**: Phase 15
**Requirements**: SCHEMA-03, SCHEMA-04, SPREAD-01, SPREAD-02, DASH-05
**Success Criteria** (what must be TRUE):
  1. Go server registers spread legs when PlaceOrder receives a spread_group_key attribute
  2. Python covered call strategy emits spread_group_key and leg_role on multi-leg orders
  3. Spread P&L view shows combined net profit per spread group (not per-leg), and a covered call round-trip appears as one trade unit
  4. Operator can view spread win/loss ratio and spread P&L over time in Metabase
**Plans**: TBD

Plans:
- [ ] 16-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 12 -> 12.1 -> 12.2 -> 13 -> ... -> 16

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Python Codebase Restructure | v1.0 | 4/4 | Complete | 2026-03-26 |
| 2. Go OTel Foundation & Local Backend | v1.0 | 3/3 | Complete | 2026-03-26 |
| 3. Go Telemetry Instrumentation | v1.0 | 3/3 | Complete | 2026-03-26 |
| 4. Python Telemetry Instrumentation | v1.0 | 3/3 | Complete | 2026-03-26 |
| 5. End-to-End Tick Tracing | v1.0 | 1/1 | Complete | 2026-03-27 |
| 6. Dashboards & Alerts | v1.0 | 2/2 | Complete | 2026-03-27 |
| 7. Production Deployment | v1.0 | 2/2 | Complete | 2026-03-28 |
| 8. Metric Labels & Bug Fix | v1.1 | 1/1 | Complete | 2026-03-29 |
| 9. Playground Filtering & Detail Panels | v1.1 | 1/1 | Complete | 2026-03-29 |
| 10. Signal/Candle Filtering & Python Heartbeat | v1.1 | 1/1 | Complete | 2026-03-29 |
| 11. Per-Strategy Dashboards | v1.1 | 1/1 | Complete | 2026-03-29 |
| 12. Deploy Metabase & Harden Infrastructure | v2.0 | 0/1 | Planned    |  |
| 13. Analytics Schema & Indexes | v2.0 | 0/0 | Not started | - |
| 14. Core Performance Dashboards | v2.0 | 0/0 | Not started | - |
| 15. Simulator Persistence & Backtest Comparison | v2.0 | 0/0 | Not started | - |
| 16. Spread Analytics | v2.0 | 0/0 | Not started | - |
