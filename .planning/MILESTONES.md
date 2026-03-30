# Milestones

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
