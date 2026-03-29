# Roadmap: Grodt Trading Platform

## Milestones

- **v1.0 Live Simulation Observability** -- Phases 1-7 (shipped 2026-03-28)
- **v1.1 Dashboard Enhancements** -- Phases 8-11 (in progress)

## Phases

<details>
<summary>v1.0 Live Simulation Observability (Phases 1-7) -- SHIPPED 2026-03-28</summary>

- [x] Phase 1: Python Codebase Restructure (4/4 plans)
- [x] Phase 2: Go OTel Foundation & Local Backend (3/3 plans)
- [x] Phase 3: Go Telemetry Instrumentation (3/3 plans)
- [x] Phase 4: Python Telemetry Instrumentation (3/3 plans)
- [x] Phase 5: End-to-End Tick Tracing (1/1 plan)
- [x] Phase 6: Dashboards & Alerts (2/2 plans)
- [x] Phase 7: Production Deployment (2/2 plans)

Full details: [milestones/v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md)

</details>

### v1.1 Dashboard Enhancements (In Progress)

**Milestone Goal:** Improve Grafana dashboard usability with per-playground filtering, richer panel detail, client_id labels, and per-strategy dashboards.

- [x] **Phase 8: Metric Labels & Bug Fix** - Add client_id to all Go metrics and fix order_filled quantity bug
- [ ] **Phase 9: Playground Filtering & Detail Panels** - Grafana dropdown filtering by client_id and enriched order/position panels
- [ ] **Phase 10: Signal/Candle Filtering & Python Heartbeat** - Per-signal-type and per-symbol filtering, per-playground heartbeat from Python
- [ ] **Phase 11: Per-Strategy Dashboards** - Separate Grafana dashboard per strategy type

## Phase Details

### Phase 8: Metric Labels & Bug Fix
**Goal**: Every Go-side OTel metric includes client_id as a label, and order_filled logs report accurate quantity
**Depends on**: Phase 7 (v1.0 production deployment)
**Requirements**: LABEL-01, PANEL-04
**Success Criteria** (what must be TRUE):
  1. Querying any OTel metric in Grafana can filter/group by client_id alongside playground_id
  2. When an order is filled, the structured log event includes the actual filled quantity (not zero or missing)
  3. Metrics for playgrounds without a client_id still work (empty string or "unset" fallback)
**Plans**: 1 plan
Plans:
- [x] 08-01-PLAN.md -- Add client_id to PlaygroundAttrs, update all metric sites, fix order_filled quantity bug

### Phase 9: Playground Filtering & Detail Panels
**Goal**: Operator can filter the entire dashboard by client_id and see detailed order/position information including trace links
**Depends on**: Phase 8
**Requirements**: LABEL-02, PANEL-01, PANEL-03, PANEL-05, PANEL-06
**Success Criteria** (what must be TRUE):
  1. Grafana dashboard has a dropdown variable that lists client_id values (falling back to playground_id when no client_id is set)
  2. Active Playgrounds panel shows a table of client_id and playground_id pairs instead of a single count number
  3. Open Orders panel displays the order symbol and quantity for each open order
  4. Recent Order Events and Position Details panels include a trace_id column, linkable to Tempo traces
**Plans**: 1 plan
Plans:
- [ ] 09-01-PLAN.md -- Update dashboard JSON with client_id filter, table panels, and trace_id in log formats

### Phase 10: Signal/Candle Filtering & Python Heartbeat
**Goal**: Operator can drill into signals by type and candles by symbol, and see per-playground heartbeat status from the Python strategy
**Depends on**: Phase 8
**Requirements**: LABEL-03, LABEL-04, PANEL-02
**Success Criteria** (what must be TRUE):
  1. Signals Generated panel can be filtered by signal_type name (e.g., show only mean_reversion signals)
  2. Candles Processed panel can be filtered by candle symbol (e.g., show only AAPL candles)
  3. Python Strategy Heartbeat panel shows individual per-playground heartbeat status instead of a single binary alive/dead indicator
**Plans**: TBD
**UI hint**: yes

### Phase 11: Per-Strategy Dashboards
**Goal**: Each strategy type has its own dedicated Grafana dashboard with strategy-specific panels
**Depends on**: Phase 9, Phase 10
**Requirements**: STRAT-01
**Success Criteria** (what must be TRUE):
  1. At least two strategy-specific dashboards exist (e.g., mean_reversion, covered_call) accessible from Grafana
  2. Each strategy dashboard shows panels relevant to that strategy type (not a copy of the main dashboard)
  3. Strategy dashboards inherit the client_id/playground_id filtering from the main dashboard pattern
**Plans**: TBD
**UI hint**: yes

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Python Codebase Restructure | v1.0 | 4/4 | Complete | 2026-03-26 |
| 2. Go OTel Foundation & Local Backend | v1.0 | 3/3 | Complete | 2026-03-26 |
| 3. Go Telemetry Instrumentation | v1.0 | 3/3 | Complete | 2026-03-26 |
| 4. Python Telemetry Instrumentation | v1.0 | 3/3 | Complete | 2026-03-26 |
| 5. End-to-End Tick Tracing | v1.0 | 1/1 | Complete | 2026-03-27 |
| 6. Dashboards & Alerts | v1.0 | 2/2 | Complete | 2026-03-27 |
| 7. Production Deployment | v1.0 | 2/2 | Complete | 2026-03-28 |
| 8. Metric Labels & Bug Fix | v1.1 | 1/1 | Complete | 2026-03-29 |
| 9. Playground Filtering & Detail Panels | v1.1 | 0/1 | Planning complete | - |
| 10. Signal/Candle Filtering & Python Heartbeat | v1.1 | 0/0 | Not started | - |
| 11. Per-Strategy Dashboards | v1.1 | 0/0 | Not started | - |
