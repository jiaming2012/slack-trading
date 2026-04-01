# Milestones

## v3.0 TradeSignal Framework (Shipped: 2026-04-01)

**Phases completed:** 10 phases, 24 plans, 40 tasks

**Key accomplishments:**

- TradeSignal Go struct with UUID, SignalName registry, SavedEvent implementation, and TradeSignalProto proto message with signal_id on PlaceOrderRequest and Order
- Nullable signal_id UUID wired through full order lifecycle: PlaceOrder RPC -> CreateOrderRequest -> commitOrderRecord -> OrderRecord -> convertOrder proto response
- ISignalRepository interface and InMemorySignalRepository with clock-gated delivery using sorted-slice cursor pattern, verified by 6 TDD test cases including race detection
- WriteSignal/GetSignals/GetProcessedSignals Twirp RPCs with global signal repository, OTel counter, and 7 unit tests
- ESDBSignalRepository with write-through ESDB persistence and global trade-signals stream naming fix
- Environment-based signal repository injection wired through Server/Twirp/main.go with ESDB integration test proving write-read round-trip and filtering by name, symbol, and time
- Stateless MA crossover datasource module extracting signal detection from V1, consumed by MeanReversionStrategyV2 with identical behavior
- Behavioral diff tests proving zero metric drift between V1 and V2 mean-reversion strategies, with datasource unit tests and V2 demo launcher
- BaseStrategy on_signal() hook plus OptionsMeanReversionStrategyV2 with options_ma_crossover datasource and 3 passing diff tests
- CreditSpreadStrategyV2 consuming credit_spread_signals datasource with bidirectional signal output, validated by 5 behavioral diff tests
- CoveredCall and Wheel strategies migrated to V2 with supertrend-based datasource extraction and 8 behavioral diff tests proving zero metric drift
- ESDB signal replay via proto field with date-filtered preload into InMemorySignalRepository, plus grodt.signals.consumed OTel counter
- DatasourceHeartbeat class emitting grodt.datasource.heartbeat gauge every 30s, Grafana signal panels, and datasource staleness alerting at 5m threshold
- Dual-run integration test proving ESDB replay delivers identical signals in identical order to in-memory baseline across 10 signals and 10 clock ticks
- WriteSignal, GetSignals, GetProcessedSignals Twirp RPCs with proto stubs, OTel telemetry, globalSignalRepo wiring, and 10 unit tests
- Three signal RPC wrapper methods (write_signal, get_signals, get_processed_signals) added to BacktesterPlaygroundClient with protobuf Timestamp conversion and retry logic
- Extracted supertrend signal detection into callable-based datasources for covered call and wheel strategies, created V2 classes delegating to datasources, and proved V1/V2 behavioral equivalence with 9 diff tests
- PDFWheelStrategyV2 with compound signal datasource extraction, Kelly sizing, and 6 diff tests proving V1/V2 behavioral equivalence
- on_signal() callback wired in client.py tick() for TradeSignal delivery; all 6 V1 strategy files moved to deprecated/ with imports updated across 25 files
- All 6 datasource scripts wired with standalone __main__ blocks using DatasourceHeartbeat gauge and WriteSignal RPC for live signal production

---

## v2.0 Metabase Analytics (Shipped: 2026-03-30)

**Phases completed:** 5 phases, 8 plans, 17 tasks

**Key accomplishments:**

- Local Metabase docker-compose (v0.59.4 on port 3001) with idempotent SQL creating metabaseappdb + read-only metabase_ro user on DO Postgres
- Composite indexes on trading tables and SQL views replicating CalcRealizedPL() for Metabase P&L, win rate, and profit factor analytics
- Metabase provisioning script creating 3 dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) with 15 native SQL cards, playground filters, and equity curve with drawdown
- 6 SQL analytics views deployed and 3 Metabase dashboards (Trading Performance, Slippage Analysis, Portfolio Analytics) provisioned and verified with real trading data
- backtest_runs SQL table with psycopg2 persistence module, strategy parameter capture via get_parameters(), and --save-to-db CLI flag on demo script
- Strategy Comparison dashboard with 5 cards (runs table, equity curves overlay, parameter comparison, best/worst return scalars) filtered by strategy_name
- PlaceMultiLegOrder injects shared spread_group_key UUID + leg_role into each leg's attributes; SQL views aggregate per-spread P&L with GIN index
- Metabase Dashboard 5 with spread summary/timeline/detail cards plus E2E test validating PlaceMultiLegOrder attribute injection round-trip

---

## v1.1 Dashboard Enhancements (Shipped: 2026-03-30)

**Phases completed:** 4 phases, 4 plans, 9 tasks

**Key accomplishments:**

- Added client_id label to all OTel metrics via PlaygroundAttrs, fixed order_filled log reporting zero fill_quantity from Trade-based ExecutionFillRequest
- Grafana dashboard with client_id dropdown filter, Active Playgrounds table, Open Orders detail from Loki, and traceID in order/position log panels
- Two Grafana dashboards (mean_reversion + covered_call) with strategy-specific panels, pre-filtered signal queries, and Docker Compose auto-provisioning

---

## v1.0 Live Simulation Observability (Shipped: 2026-03-28)

**Phases completed:** 7 phases, 18 plans, 34 tasks

**Key accomplishments:**

- Moved 48 Python files into 7 subdirectory packages, merged type definitions, updated all imports to absolute paths, and validated 344 pytest tests collect successfully
- BaseStrategy ABC with 3 abstract + 3 lifecycle methods, all 6 strategies adapted, and universal run_strategy() tick loop in trading engine
- All 6 demo scripts wired through run_strategy(), optimizer updated with strategy factory pattern (D-06) and enable_retraining=False (D-05), 344 tests collect with 296 passing
- Fixed 40 test failures from incomplete DIR-08 by updating stale @patch() decorator paths and fixing credit spread test setup
- TracerProvider + MeterProvider with OTLP HTTP exporters, logrus logfmt formatter, and W3C propagator activating existing tracer spans
- Centralized OTel metrics registry with 8 instruments, plus structured logs and counters for order placed/filled/rejected events gated to live/reconcile playgrounds
- Candle counter increments in simulateTick, tick latency measurement in NextTick, structured data gap warnings, and signal counter in ProcessSignalTriggeredEvent
- 30-second heartbeat goroutine with OTel gauge metrics (active playgrounds, open orders, uptime) and structured log, wired via DatabaseService stats provider
- OTel Python SDK with OTLP HTTP exporters (no grpcio), idempotent setup_otel(), and StrategyHeartbeat daemon thread emitting gauge metric + structured log every 30s
- OTel spans wired into tick loop (live mode only), trace_id injected on all Twirp RPC requests via _get_trace_id() helper
- RED:
- Grafana live simulation dashboard with playground_id filtering, 4 row layout (System Health, Order Activity, Market Data, Positions), and file-based provisioning via docker-compose volume mounts
- Grafana unified alerting with 3 rules (Go heartbeat stale, Python heartbeat stale, error rate spike), Slack contact point via env var webhook, and docker-compose provisioning
- Production Docker Compose with grodt, otel-lgtm, postgres, eventstore.db services plus env template with CHANGEME secret placeholders

---
