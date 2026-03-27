# Roadmap: Live Simulation Observability

## Overview

Transform the slack-trading platform from opaque live simulation operation to full observability. Start by restructuring the Python codebase (so instrumentation targets the final structure), then activate Go's existing OTel spans, stand up a local telemetry backend, instrument both Go and Python sides, link them with cross-process tracing, build dashboards, and deploy to production. Each phase delivers independently useful capability.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Python Codebase Restructure** - Reorganize Python client into clean directory structure and consolidate strategies through trading_engine
- [ ] **Phase 2: Go OTel Foundation & Local Backend** - Initialize OTel providers in Go server and stand up local Grafana/Loki/Tempo/Prometheus stack
- [ ] **Phase 3: Go Telemetry Instrumentation** - Instrument order lifecycle, market data flow, and server heartbeat with structured logs and metrics
- [ ] **Phase 4: Python Telemetry Instrumentation** - Install Python OTel SDK, instrument strategy decisions, and add Python heartbeat
- [ ] **Phase 5: End-to-End Tick Tracing** - Link Python-to-Go RPC calls with W3C traceparent propagation for unified traces
- [ ] **Phase 6: Dashboards & Alerts** - Build Grafana dashboards for live simulation monitoring and configure staleness/error alerts
- [ ] **Phase 7: Production Deployment** - Deploy observability stack to Digital Ocean and connect live trading infrastructure

## Phase Details

### Phase 1: Python Codebase Restructure
**Goal**: Python client code is organized into a maintainable directory structure with all strategies running through a single engine
**Depends on**: Nothing (first phase)
**Requirements**: DIR-01, DIR-02, DIR-03, DIR-04, DIR-05, DIR-06, DIR-07, DIR-08, CONS-01, CONS-02, CONS-03, CONS-04
**Success Criteria** (what must be TRUE):
  1. Running `python -m demos.demo_covered_call` (or equivalent) from the restructured directory works end-to-end
  2. All 6 strategy demo scripts launch via trading_engine.run_strategy() instead of their own tick loops
  3. All existing strategy tests pass without modification to test logic (only import paths change)
  4. No Python files remain in the flat src/clients/python/ root (all moved to subdirectories or deprecated/)
**Plans:** 4 plans
Plans:
- [x] 01-01-PLAN.md -- Directory restructure: move all files, create packages, fix imports, update taskfile
- [x] 01-02-PLAN.md -- Strategy consolidation: BaseStrategy class, adapt 6 strategies, engine tick loop
- [x] 01-03-PLAN.md -- Demo refactoring, optimizer refactoring, full test validation
- [x] 01-04-PLAN.md -- Gap closure: fix stale @patch paths and test assertions (CONS-04)

### Phase 2: Go OTel Foundation & Local Backend
**Goal**: Existing Go OTel spans produce real traces visible in a local Grafana instance
**Depends on**: Nothing (independent of Phase 1, but sequenced after for workflow clarity)
**Requirements**: OTEL-01, OTEL-02, OTEL-03, OTEL-04, OTEL-05, OTEL-06, BACK-01, BACK-02, BACK-03
**Success Criteria** (what must be TRUE):
  1. Starting the Go server and making a Twirp RPC call produces a visible trace in Grafana/Tempo
  2. Running `docker compose up` in the observability directory starts Collector, Loki, Grafana, Tempo, and Prometheus with no manual configuration
  3. Grafana opens in a browser with Loki, Tempo, and Prometheus already configured as data sources
  4. Stopping the Go server flushes all pending telemetry (no data loss on shutdown)
  5. Structured log fields (playground_id, symbol, environment) appear consistently in Loki log entries
  6. NextTickRequest and PlaceOrderRequest carry trace_id for cross-process trace correlation
**Plans:** 3 plans
Plans:
- [x] 02-01-PLAN.md -- Go OTel SDK initialization: SetupOTelSDK function, logrus logfmt config, env vars
- [x] 02-02-PLAN.md -- Observability backend: Docker Compose with grafana/otel-lgtm, Taskfile entries
- [ ] 02-03-PLAN.md -- Proto trace_id propagation: add trace_id to NextTickRequest/PlaceOrderRequest, regenerate stubs, update Go handlers

### Phase 3: Go Telemetry Instrumentation
**Goal**: The operator can see order activity, market data flow, and server liveness through logs and metrics
**Depends on**: Phase 2
**Requirements**: ORD-01, ORD-02, ORD-03, ORD-04, DATA-01, DATA-02, DATA-03, BEAT-01, BEAT-02
**Success Criteria** (what must be TRUE):
  1. Placing an order in a live playground produces a structured log entry visible in Loki with order details (symbol, side, quantity)
  2. The Grafana metrics explorer shows a server heartbeat gauge updating every 30 seconds
  3. Candle arrival and tick processing events appear in Loki with symbol, timeframe, and latency
  4. Simulator playground orders do NOT produce telemetry logs (live-only filtering works)
**Plans:** 3 plans
Plans:
- [x] 03-01-PLAN.md -- Telemetry metrics package and order lifecycle instrumentation (placement, fill, rejection)
- [ ] 03-02-PLAN.md -- Market data flow: candle counters, tick latency, data gap warnings
- [x] 03-03-PLAN.md -- Server heartbeat: background goroutine with gauge metrics and structured logs

### Phase 4: Python Telemetry Instrumentation
**Goal**: The operator can see strategy decisions, Python heartbeat, and "is my strategy running?" status
**Depends on**: Phase 2
**Requirements**: PYTEL-01, PYTEL-02, PYTEL-03, PYTEL-04, STRAT-01, STRAT-02, STRAT-03, BEAT-03, BEAT-04
**Success Criteria** (what must be TRUE):
  1. Running a Python strategy client produces traces visible in Grafana/Tempo
  2. The Grafana metrics explorer shows a Python strategy heartbeat gauge updating every 30 seconds
  3. Strategy indicator evaluations and signal decisions appear as structured log entries in Loki
  4. "No action" decisions (below threshold, position full) are logged with explicit reasons
  5. OTel Python packages are installed without breaking numpy 1.26.4 or pandas_ta
**Plans:** 3 plans
Plans:
- [x] 04-01-PLAN.md -- OTel SDK install, setup_otel() module, StrategyHeartbeat daemon thread
- [ ] 04-02-PLAN.md -- SignalDecision dataclass and structured decision logging in BaseStrategy
- [ ] 04-03-PLAN.md -- Tick loop OTel spans, trace_id on RPC requests, engine wiring

### Phase 5: End-to-End Tick Tracing
**Goal**: A single trace in Grafana/Tempo shows the complete path from Python tick loop through Go server RPC and back
**Depends on**: Phase 3, Phase 4
**Requirements**: TICK-01, TICK-02, TICK-03, TICK-04
**Success Criteria** (what must be TRUE):
  1. Opening a trace in Tempo shows parent span (Python tick) with child spans for NextTick RPC and PlaceOrder RPC
  2. Each tick trace includes playground_id, tick_number, symbols, and duration_ms as searchable attributes
  3. Clicking a Go server span in Tempo navigates to the Python parent span that initiated the RPC call
**Plans:** 1 plan
Plans:
- [x] 05-01-PLAN.md -- W3C traceparent injection (Python) and otelhttp middleware (Go) for distributed trace linking

### Phase 6: Dashboards & Alerts
**Goal**: A purpose-built Grafana dashboard answers "what is happening right now?" and alerts fire when things go wrong
**Depends on**: Phase 3, Phase 4
**Requirements**: DASH-01, DASH-02, DASH-03, ALERT-01, ALERT-02, ALERT-03
**Success Criteria** (what must be TRUE):
  1. Opening the live simulation dashboard shows aggregate activity across all live playgrounds
  2. Selecting a specific playground_id from a dropdown filters the dashboard to that playground's data
  3. Open positions across live playgrounds are visible in a summary panel
  4. Stopping the Go server for >2 minutes triggers a Grafana alert notification
  5. Stopping the Python strategy for >2 minutes triggers a separate Grafana alert notification
**Plans:** 2 plans
Plans:
- [x] 06-01-PLAN.md -- Dashboard provisioning: provider YAML, live simulation dashboard JSON, docker-compose volume mounts
- [x] 06-02-PLAN.md -- Alert rules: heartbeat staleness + error rate alerts, Slack contact point, notification policy

### Phase 7: Production Deployment
**Goal**: The observability stack runs on Digital Ocean and receives telemetry from the live trading infrastructure
**Depends on**: Phase 6
**Requirements**: DEPLOY-01, DEPLOY-02, DEPLOY-03
**Success Criteria** (what must be TRUE):
  1. Grafana is accessible via a web URL with authentication required
  2. Traces and logs from the live Go server (running on Vultr K8s) appear in the Digital Ocean Grafana instance
  3. Traces and logs from the live Python strategy client appear in the same Grafana instance
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7
Note: Phases 3 and 4 depend only on Phase 2 (not each other) but are sequenced for workflow focus.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Python Codebase Restructure | 3/4 | Verifying | - |
| 2. Go OTel Foundation & Local Backend | 0/3 | Planned | - |
| 3. Go Telemetry Instrumentation | 0/3 | Planned | - |
| 4. Python Telemetry Instrumentation | 0/3 | Planned | - |
| 5. End-to-End Tick Tracing | 0/1 | Planned | - |
| 6. Dashboards & Alerts | 2/2 | Complete | 2026-03-27 |
| 7. Production Deployment | 0/? | Not started | - |
