# Roadmap: Live Simulation Observability

## Milestones

- ✅ **v1.0 Live Simulation Observability** - Phases 1-7 (shipped 2026-03-28)
- ✅ **v1.1 Dashboard Enhancements** - Phases 8-11 (shipped 2026-03-30)
- ✅ **v2.0 Metabase Analytics** - Phases 12-16 (shipped 2026-03-30)
- 🚧 **v3.0 TradeSignal Framework** - Phases 17-26 (in progress)

## Phases

<details>
<summary>v1.0 Live Simulation Observability (Phases 1-7) - SHIPPED 2026-03-28</summary>

- [x] **Phase 1: Python Codebase Restructure** - Reorganize Python clients into packages
- [x] **Phase 2: Strategy Abstraction** - BaseStrategy ABC and unified tick loop
- [x] **Phase 3: Demo Script Consolidation** - Wire all demos through run_strategy()
- [x] **Phase 4: Go OTel Foundation** - TracerProvider, MeterProvider, structured logs
- [x] **Phase 5: Go Instrumentation** - Metrics and logs for orders, candles, signals, heartbeat
- [x] **Phase 6: Python OTel Integration** - OTel SDK, heartbeat, trace propagation
- [x] **Phase 7: Grafana Dashboard & Alerting** - Dashboard, alert rules, production deploy

</details>

<details>
<summary>v1.1 Dashboard Enhancements (Phases 8-11) - SHIPPED 2026-03-30</summary>

- [x] **Phase 8: Client ID Labeling** - client_id on all OTel metrics
- [x] **Phase 9: Dashboard Enrichment** - Dropdowns, tables, trace links
- [x] **Phase 10: Per-Strategy Dashboards** - Mean reversion + covered call dashboards
- [x] **Phase 11: Signal & Candle Filtering** - Filter by signal_type and symbol

</details>

<details>
<summary>v2.0 Metabase Analytics (Phases 12-16) - SHIPPED 2026-03-30</summary>

- [x] **Phase 12: Deploy Metabase & Harden Infrastructure** - Metabase docker-compose, read-only user, firewall
- [x] **Phase 13: Analytics Schema & Indexes** - Composite indexes, SQL views for P&L/slippage/stats
- [x] **Phase 14: Core Performance Dashboards** - Trading, Slippage, Portfolio dashboards
- [x] **Phase 15: Simulator Persistence & Backtest Comparison** - backtest_runs table, comparison dashboard
- [x] **Phase 16: Spread Analytics** - Spread grouping, views, dashboard

</details>

### v3.0 TradeSignal Framework (In Progress)

**Milestone Goal:** Decouple signal production from strategy execution via a unified TradeSignal event stream, enabling replayable simulations, live signal persistence, and consistent live/sim parity.

- [x] **Phase 17: Signal Foundation** - TradeSignal struct, proto messages, signal_id on orders (completed 2026-03-30)
- [x] **Phase 18: Signal Repository & Sim Mode** - ISignalRepository interface with in-memory implementation and tick-synchronized delivery (completed 2026-03-30)
- [ ] **Phase 19: RPC Endpoints & Python Integration** - WriteSignal/GetSignals RPCs, Python client wrappers, datasource script pattern
- [x] **Phase 20: ESDB Persistence** - EventStoreDB signal repository for live mode with queryability (completed 2026-03-31)
- [x] **Phase 21: First Strategy Migration & Validation** - Migrate one strategy end-to-end, prove behavioral diff testing (completed 2026-03-31)
- [ ] **Phase 22: Remaining Strategy Migrations** - All strategies migrated, originals deprecated
- [x] **Phase 23: Replay & Telemetry** - Replay from persisted signals, OTel integration, alerting (completed 2026-03-31)
- [x] **Phase 24: Signal RPC Endpoints** - WriteSignal/GetSignals/GetProcessedSignals RPCs (gap closure) (completed 2026-03-31)
- [x] **Phase 25: Complete Strategy Migrations** - Recover missing V2s, PDFWheel V2, on_signal() wiring, deprecate V1s (gap closure) (completed 2026-03-31)
- [ ] **Phase 26: Python Datasource Wiring & Observability** - Python client wrappers, __main__ blocks, DatasourceHeartbeat (gap closure)

## Phase Details

### Phase 17: Signal Foundation
**Goal**: A canonical TradeSignal type exists in Go and proto, with signal_id linking every order to its originating signal
**Depends on**: Nothing (first phase of v3.0)
**Requirements**: SIG-01, SIG-02, SIG-03
**Success Criteria** (what must be TRUE):
  1. TradeSignal struct exists in Go models with Name, Attributes (flexible map), and Timestamp fields
  2. Proto definition includes TradeSignalProto message and signal_id field on PlaceOrderRequest
  3. Signal name constants are defined as a typed registry (not freeform strings)
  4. Go unit tests verify TradeSignal serialization round-trip and signal_id presence on order requests
**Plans**: 2 plans

Plans:
- [x] 17-01-PLAN.md — TradeSignal type, SignalName registry, proto TradeSignalProto message
- [x] 17-02-PLAN.md — signal_id wiring through order lifecycle (PlaceOrder -> OrderRecord -> response)

### Phase 18: Signal Repository & Sim Mode
**Goal**: Strategies can write and read signals through a repository interface, with in-memory implementation powering simulations via tick-synchronized delivery
**Depends on**: Phase 17
**Requirements**: REPO-01, REPO-03
**Success Criteria** (what must be TRUE):
  1. ISignalRepository interface exists with Write and Read methods, and InMemorySignalRepository implements it
  2. Signals written to the repository are delivered to strategies via TickDelta (gated by playground clock time)
  3. Opt-in CLI flag persists sim signals to EventStoreDB when specified
  4. Unit tests verify clock-gated signal delivery prevents lookahead bias
**Plans**: 2 plans

Plans:
- [x] 18-01-PLAN.md — ISignalRepository interface, InMemorySignalRepository with clock-gated delivery tests
- [x] 18-02-PLAN.md — TickDelta wiring (proto + simulateTick + NextTick conversion) and ESDB batch persistence

### Phase 19: RPC Endpoints & Python Integration
**Goal**: Python datasource scripts and strategies can produce and consume signals through Twirp RPC endpoints
**Depends on**: Phase 18
**Requirements**: DS-01, DS-02, DS-03, RPC-01
**Success Criteria** (what must be TRUE):
  1. WriteSignal RPC endpoint accepts signals from Python clients and stores them via the repository interface
  2. GetSignals RPC endpoint returns signals filtered by name, symbol, and time range for a given strategy
  3. A standalone Python datasource script can run from __main__ and produce signals to a per-symbol event stream
  4. Sim strategies can import datasource modules directly (no RPC needed for sim signal generation)
**Plans**: 2 plans

Plans:
- [ ] 19-01-PLAN.md — Proto messages, Go RPC handlers (WriteSignal, GetSignals, GetProcessedSignals), OTel counters, unit tests
- [ ] 19-02-PLAN.md — Python client write_signal()/get_processed_signals() wrappers, datasource skeleton package

### Phase 20: ESDB Persistence
**Goal**: Live environments persist signals to a single global EventStoreDB stream with Go-side queryability by name, symbol, and timeframe
**Depends on**: Phase 19
**Requirements**: REPO-02, QUERY-01
**Success Criteria** (what must be TRUE):
  1. ESDBSignalRepository writes signals to the global `trade-signals` EventStoreDB stream in live mode
  2. Environment-based injection selects InMemory (sim) or ESDB (live) repository at startup
  3. Signals are queryable by name, symbol, and timeframe via Go-side filtering (read stream + filter)
  4. Integration test verifies signal write-read round-trip through ESDB
**Plans**: 2 plans

Plans:
- [x] 20-01-PLAN.md — ESDBSignalRepository implementation, global stream name fix, unit tests
- [x] 20-02-PLAN.md — Environment-based injection wiring, ESDB integration test

### Phase 21: First Strategy Migration & Validation
**Goal**: MeanReversionStrategy migrated to consume TradeSignals from datasource module, proving the migration pattern and behavioral diff testing approach
**Depends on**: Phase 20
**Requirements**: MIG-03
**Success Criteria** (what must be TRUE):
  1. One strategy (e.g., CoveredCall) consumes TradeSignals instead of inline signal detection
  2. Behavioral diff test compares migrated strategy output against original on the same data with zero metric drift
  3. The diff testing pattern is documented and reusable for remaining strategy migrations
**Plans**: 2 plans

Plans:
- [x] 21-01-PLAN.md — MA crossover datasource module and MeanReversionStrategyV2
- [x] 21-02-PLAN.md — Behavioral diff test, datasource unit tests, V2 demo launcher

### Phase 22: Remaining Strategy Migrations
**Goal**: All existing strategies consume TradeSignals, with originals moved to deprecated
**Depends on**: Phase 21
**Requirements**: MIG-01, MIG-02
**Success Criteria** (what must be TRUE):
  1. All existing strategies are migrated to consume TradeSignals instead of inline signal detection
  2. Original strategy files are moved to the deprecated/ folder
  3. Each migrated strategy passes its behavioral diff test against the original
  4. The trading engine runs end-to-end with only migrated strategies (no legacy signal paths)
**Plans**: 5 plans

Plans:
- [x] 22-01-PLAN.md — BaseStrategy on_signal() hook + OptionsMeanReversion datasource/V2/diff test
- [x] 22-02-PLAN.md — CreditSpread datasource/V2/diff test
- [ ] 22-03-PLAN.md — CoveredCall + Wheel datasources/V2s/diff tests
- [ ] 22-04-PLAN.md — PDFWheel datasource/V2/diff test (depends on Wheel V2)
- [ ] 22-05-PLAN.md — Trading engine on_signal() wiring + move all V1 to deprecated/ + end-to-end verification

### Phase 23: Replay & Telemetry
**Goal**: Strategies can replay persisted signal streams for reproducible simulations, with signals visible in the observability stack
**Depends on**: Phase 22
**Requirements**: REPLAY-01, REPLAY-02, OBS-01, OBS-02
**Success Criteria** (what must be TRUE):
  1. Sim strategies can replay persisted signal streams from ESDB, gated by playground clock time
  2. Integration tests verify replay results match in-memory datasource results for the same inputs
  3. Signals appear in Grafana via OTel telemetry (metrics and/or log panels)
  4. An alert fires when a strategy's expected TradeSignal is not produced within the configured interval
**Plans**: 3 plans

Plans:
- [x] 23-01-PLAN.md — Proto replay_signal_stream field, Go replay preload, signal consumption counter, Python CLI flag
- [x] 23-02-PLAN.md — DatasourceHeartbeat class, Grafana signal panels, datasource heartbeat alert
- [x] 23-03-PLAN.md — Dual-run replay integration test (ESDB replay vs in-memory comparison)

### Phase 24: Signal RPC Endpoints
**Goal**: WriteSignal, GetSignals, and GetProcessedSignals Twirp RPCs exist and are callable from Python
**Depends on**: Phase 18 (ISignalRepository)
**Requirements**: DS-01, DS-02, RPC-01
**Gap Closure**: Phase 19 work absent from branch — RPCs need to be (re)built
**Success Criteria** (what must be TRUE):
  1. WriteSignal RPC endpoint accepts signals from Python clients and stores them via the signal repository
  2. GetSignals RPC endpoint returns signals filtered by name, symbol, and time range
  3. GetProcessedSignals RPC endpoint returns signals consumed by a specific playground
  4. Python proto stubs are regenerated and include all three new RPCs
  5. OTel counter tracks signals produced via WriteSignal
  6. Unit tests verify each handler's happy path and validation errors
**Plans**: 2 plans

Plans:
- [x] 24-01-PLAN.md — Proto messages, Go RPC handlers (WriteSignal, GetSignals, GetProcessedSignals), globalSignalRepo wiring, OTel counter, unit tests
- [x] 24-02-PLAN.md — Python client write_signal()/get_signals()/get_processed_signals() wrapper methods

### Phase 25: Complete Strategy Migrations
**Goal**: All strategies migrated to TradeSignal framework, on_signal() wired in Python client, V1 files moved to deprecated/
**Depends on**: Phase 24
**Requirements**: MIG-01, MIG-02
**Gap Closure**: Phase 22-03 files missing from disk; 22-04/22-05 unexecuted; on_signal() is dead code
**Success Criteria** (what must be TRUE):
  1. covered_call_v2.py and wheel_v2.py exist on disk with corresponding datasource modules and diff tests
  2. PDFWheel V2 strategy and datasource exist with diff test
  3. Python client.py tick() extracts new_signals from TickDelta and calls strategy.on_signal() for each
  4. Trading engine runs end-to-end with only V2 strategies
  5. All V1 strategy files moved to deprecated/ folder
  6. Each migrated strategy passes its behavioral diff test
**Plans**: 3 plans

Plans:
- [x] 25-01-PLAN.md — CoveredCall V2 + Wheel V2 datasources, strategies, diff tests
- [x] 25-02-PLAN.md — PDFWheel V2 datasource, strategy, diff test
- [ ] 25-03-PLAN.md — on_signal() wiring in client.py, move V1s to deprecated/

### Phase 26: Python Datasource Wiring & Observability
**Goal**: Datasource scripts run standalone via __main__ calling WriteSignal RPC, DatasourceHeartbeat emits metrics
**Depends on**: Phase 24 (WriteSignal RPC), Phase 25 (V2 strategies)
**Requirements**: DS-03, OBS-02
**Gap Closure**: No __main__ blocks on datasources; DatasourceHeartbeat never instantiated
**Success Criteria** (what must be TRUE):
  1. Python client wrapper functions write_signal() and get_processed_signals() exist and call the Twirp RPCs
  2. At least one datasource script has a __main__ block that produces signals via WriteSignal RPC
  3. Sim strategies can import datasource modules directly (no RPC needed)
  4. DatasourceHeartbeat is instantiated in at least one datasource script and emits the heartbeat gauge
  5. OBS-02 alert can fire when heartbeat gauge goes stale
**Plans**: 1 plan

Plans:
- [ ] 26-01-PLAN.md — __main__ blocks on all 6 datasources with DatasourceHeartbeat and WriteSignal RPC

## Progress

**Execution Order:**
Phases execute in numeric order: 17 -> 18 -> 19 -> 20 -> 21 -> 22 -> 23 -> 24 -> 25 -> 26

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 17. Signal Foundation | v3.0 | 2/2 | Complete    | 2026-03-30 |
| 18. Signal Repository & Sim Mode | v3.0 | 2/2 | Complete    | 2026-03-30 |
| 19. RPC Endpoints & Python Integration | v3.0 | 0/2 | Not started | - |
| 20. ESDB Persistence | v3.0 | 2/2 | Complete    | 2026-03-31 |
| 21. First Strategy Migration & Validation | v3.0 | 2/2 | Complete    | 2026-03-31 |
| 22. Remaining Strategy Migrations | v3.0 | 2/5 | In Progress|  |
| 23. Replay & Telemetry | v3.0 | 3/3 | Complete    | 2026-03-31 |
| 24. Signal RPC Endpoints | v3.0 | 2/2 | Complete    | 2026-03-31 |
| 25. Complete Strategy Migrations | v3.0 | 2/3 | Complete    | 2026-03-31 |
| 26. Python Datasource Wiring & Observability | v3.0 | 0/1 | Planned    |  |
