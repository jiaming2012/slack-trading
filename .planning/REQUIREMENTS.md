# Requirements: Live Simulation Observability

**Defined:** 2026-03-25
**Core Value:** When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

## v1 Requirements

### OTel Foundation

- [ ] **OTEL-01**: Go server initializes TracerProvider with OTLP HTTP exporter on startup
- [ ] **OTEL-02**: Go server initializes MeterProvider with OTLP HTTP exporter on startup
- [ ] **OTEL-03**: Graceful shutdown calls ForceFlush on both providers before exit
- [ ] **OTEL-04**: All existing tracer spans (~15 files) produce real traces after provider init
- [ ] **OTEL-05**: Structured log fields follow consistent conventions (playground_id, order_id, symbol, environment)
- [ ] **OTEL-06**: OTel environment variables (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME) configurable per environment

### Observability Backend

- [ ] **BACK-01**: Docker Compose starts OTel Collector, Loki, Grafana, Tempo, and Prometheus with a single command
- [ ] **BACK-02**: OTel Collector config routes traces to Tempo, logs to Loki, metrics to Prometheus
- [ ] **BACK-03**: Grafana starts with Loki, Tempo, and Prometheus pre-configured as data sources

### Order Lifecycle (Live Playgrounds Only)

- [ ] **ORD-01**: Order placement emits structured log with playground_id, symbol, side, quantity, order_type, and environment="live"
- [ ] **ORD-02**: Order fill emits structured log with fill price, quantity, and timestamp
- [ ] **ORD-03**: Order rejection emits structured log with rejection reason
- [ ] **ORD-04**: Only live playgrounds (Meta.Environment == "live") emit order telemetry; simulator playgrounds are excluded

### Heartbeat

- [ ] **BEAT-01**: Go server emits a heartbeat metric gauge every 30 seconds with service label
- [ ] **BEAT-02**: Go server emits a periodic structured heartbeat log with active playground count and last tick time
- [ ] **BEAT-03**: Python strategy client emits a heartbeat metric gauge every 30 seconds
- [ ] **BEAT-04**: Python strategy client emits a periodic structured heartbeat log with strategy state

### Market Data Flow

- [ ] **DATA-01**: Candle arrival events are logged with symbol, timeframe, and timestamp
- [ ] **DATA-02**: Tick processing events are logged with processing latency
- [ ] **DATA-03**: Data gaps (missing candles, stale data) emit warning-level logs

### Strategy Decisions

- [ ] **STRAT-01**: Python strategy logs indicator evaluation results (indicator name, value, threshold)
- [ ] **STRAT-02**: Python strategy logs signal generation events (signal type, direction, confidence)
- [ ] **STRAT-03**: Python strategy logs "no action" decisions with reason (e.g., "below threshold", "position full")

### End-to-End Tick Tracing

- [ ] **TICK-01**: Each client tick loop iteration is a single trace span capturing: client tick → server NextTick RPC → response → strategy signal evaluation → place_order RPC (if any)
- [ ] **TICK-02**: Client tick span includes attributes: playground_id, tick_number, symbols, duration_ms
- [ ] **TICK-03**: Server NextTick handler span is linked as a child of the client tick span via W3C traceparent propagation through Twirp
- [ ] **TICK-04**: PlaceOrder RPC span is linked as a child of the client tick span when orders are placed

### Python Instrumentation

- [ ] **PYTEL-01**: OTel Python SDK installed in grodt conda env (compatible with numpy 1.26.4)
- [ ] **PYTEL-02**: Python client initializes TracerProvider + MeterProvider with OTLP HTTP exporters
- [ ] **PYTEL-03**: trading_engine.py tick loop instrumented with parent span per iteration
- [ ] **PYTEL-04**: backtester_playground_client_grpc.py tick() and place_order() methods emit child spans with W3C traceparent headers

### Strategy Consolidation

- [ ] **CONS-01**: All 6 active strategies (covered_call, wheel, pdf_wheel, mean_reversion, options_mean_reversion, credit_spread) implement a common strategy interface compatible with trading_engine
- [ ] **CONS-02**: trading_engine.py refactored as the single tick loop orchestrator that all strategies run through
- [ ] **CONS-03**: Each demo script (demo_covered_call, demo_wheel, demo_pdf_wheel, demo_mean_reversion, demo_options_mean_reversion, demo_credit_spread) uses trading_engine.run_strategy() instead of its own loop
- [ ] **CONS-04**: All existing strategy tests pass after consolidation

### Directory Restructure

- [ ] **DIR-01**: Python client reorganized into strategies/, engine/, lib/, tools/, demos/, tests/, deprecated/ subdirectories
- [ ] **DIR-02**: Strategy files moved to strategies/ (covered_call, wheel, pdf_wheel, mean_reversion, options_mean_reversion, credit_spread)
- [ ] **DIR-03**: Core runtime files moved to engine/ (trading_engine, client, types, rpc_profiler)
- [ ] **DIR-04**: Shared libraries moved to lib/ (pdf_builder, pdf_types, deviation_levels, partial_exit_manager, risk_management, return_models, candlestick_patterns)
- [ ] **DIR-05**: CLI tools moved to tools/ (build_pdf, plot_option_candlestick, mean_reversion_report, credit_spread_visualizations)
- [ ] **DIR-06**: Demo entry points moved to demos/
- [ ] **DIR-07**: Deprecated files (simple_open_strategy_v4, simple_stack_open_strategy_v2, simple_close_strategy, simple_stack_close_strategy, stack_close_strategy_psar, generate_signals, plot_playground, plot_candlestick) moved to deprecated/
- [ ] **DIR-08**: All imports updated across the codebase to reflect new paths

### Dashboards & Alerts

- [ ] **DASH-01**: Grafana dashboard shows aggregate live simulation activity (all live playgrounds)
- [ ] **DASH-02**: Dashboard has dropdown filter to select a specific playground_id
- [ ] **DASH-03**: Dashboard includes position summary panel showing open positions across live playgrounds
- [ ] **ALERT-01**: Grafana alert fires when Go server heartbeat is stale (>2 minutes)
- [ ] **ALERT-02**: Grafana alert fires when Python strategy heartbeat is stale (>2 minutes)
- [ ] **ALERT-03**: Grafana alert fires when error log rate exceeds threshold

### Production Deployment

- [ ] **DEPLOY-01**: Observability stack (Loki, Grafana, Tempo, Prometheus, OTel Collector) deployed to Digital Ocean
- [ ] **DEPLOY-02**: Go server and Python client send telemetry to Digital Ocean endpoint
- [ ] **DEPLOY-03**: Grafana accessible via web with authentication

## v2 Requirements

### Advanced Tracing

- **TRACE-01**: HTTP middleware instrumentation (otelhttp) for all REST and Twirp endpoints
- **TRACE-02**: Runtime metrics (CPU, memory, goroutines) via OTel runtime package

### Advanced Dashboards

- **ADVDASH-01**: Order flow timeline visualization
- **ADVDASH-02**: Strategy decision timeline panel
- **ADVDASH-03**: Fill quality metrics (slippage, timing analysis)

### Infrastructure

- **INFRA-01**: TLS for OTLP ingestion from Vultr to Digital Ocean
- **INFRA-02**: Managed database for Grafana state persistence
- **INFRA-03**: Log retention policy configuration (auto-cleanup after N days)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Distributed tracing to Polygon/Tradier APIs | Only instrument our side; external APIs are opaque |
| Custom Grafana plugins | Built-in panels sufficient for v1 |
| PagerDuty/OpsGenie integration | Grafana native alerts sufficient for single operator |
| Simulator playground monitoring | Simulators are short-lived; live mode is the pain point |
| Sub-second dashboard refresh | 10-30s refresh sufficient for trading monitoring |
| Per-tick spans in backtesting | Would cause 35%+ CPU overhead on hot path |

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| — | — | — |

**Coverage:**
- v1 requirements: 47 total
- Mapped to phases: 0
- Unmapped: 47 ⚠️

---
*Requirements defined: 2026-03-25*
*Last updated: 2026-03-25 after initial definition*
