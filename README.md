# slack-trading (grodt)

Event-driven trading platform: Go backend server + Python strategy clients, communicating via Twirp RPC. Features a TradeSignal framework for decoupled signal production/consumption, Metabase analytics dashboards, and OpenTelemetry observability.

---

## Table of Contents
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Build & Run](#build--run)
- [TradeSignal Framework](#tradesignal-framework)
- [Analytics (Metabase)](#analytics-metabase)
- [Observability](#observability)
- [Development](#development)
- [Docker & Deployment](#docker--deployment)
- [Infrastructure](#infrastructure)
- [Integrations](#integrations)

---

## Architecture

Two-process architecture: Go server manages state, order execution, and market data; Python clients implement trading strategies.

```
┌─────────────────────┐     Twirp RPC (5051)     ┌──────────────────────┐
│   Python Clients    │ ◄──────────────────────► │     Go Server        │
│                     │                           │                      │
│ ┌─────────────────┐ │                           │ ┌──────────────────┐ │
│ │ Datasources     │ │  WriteSignal              │ │ Signal Repo      │ │
│ │ (ma_crossover,  │─┼──────────────────────────►│ │ (InMemory/ESDB)  │ │
│ │  credit_spread) │ │                           │ │                  │ │
│ └─────────────────┘ │                           │ └──────────────────┘ │
│                     │                           │                      │
│ ┌─────────────────┐ │  NextTick (signals +      │ ┌──────────────────┐ │
│ │ Strategies V2   │ │  candles via TickDelta)    │ │ Playground       │ │
│ │ (MeanReversion, │◄┼──────────────────────────┤│ │ (clock, orders,  │ │
│ │  CreditSpread,  │ │                           │ │  signal queue)   │ │
│ │  CoveredCall,   │ │  PlaceOrder (signal_id)   │ │                  │ │
│ │  Wheel, etc.)   │─┼──────────────────────────►│ │                  │ │
│ └─────────────────┘ │                           │ └──────────────────┘ │
└─────────────────────┘                           └──────────────────────┘
         │                                                  │
         │ OTel                                    OTel     │
         ▼                                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Observability Stack                            │
│  Grafana (dashboards, alerts) + Loki (logs) + Tempo (traces)    │
│  + Prometheus (metrics) + OTel Collector                         │
└──────────────────────────────────────────────────────────────────┘
         │
         ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   PostgreSQL     │    │   EventStoreDB   │    │    Metabase      │
│ (orders, trades, │    │ (signals, events)│    │ (5 dashboards,   │
│  backtest_runs)  │    │                  │    │  24 cards)        │
└──────────────────┘    └──────────────────┘    └──────────────────┘
```

### Key Subsystems

- **TradeSignal Framework** (v3.0): Decoupled signal production via datasource modules, consumed by strategies through `on_signal()`. Signals are global events stored in EventStoreDB.
- **Backtester Loop**: Python creates playground → calls NextTick in loop → receives signals + candles → calls PlaceOrder
- **Analytics**: 5 Metabase dashboards (Trading Performance, Slippage, Portfolio, Strategy Comparison, Spread Analytics) provisioned via `infra/provision-metabase.py`
- **Observability**: OpenTelemetry traces + metrics + logs, Grafana dashboards with alerting

---

## Quick Start

### Prerequisites
- Go 1.22.4
- Python 3.10 (conda `grodt` environment)
- Docker (for PostgreSQL, EventStoreDB, observability stack)
- Task (taskfile runner)

### Setup
```bash
# Clone and set environment
export TRADING_PROJECT_DIR=$(pwd)
export PYTHONPATH=${TRADING_PROJECT_DIR}:${TRADING_PROJECT_DIR}/src/clients/python:${PYTHONPATH}

# Create conda environment
conda env create -f grodt.yml
conda activate grodt

# Start databases
task db:start

# Build and run
task app:dev
```

---

## Project Structure

```
cmd/main.go                          # Server entrypoint
src/go/
  backtester-api/
    models/                          # Domain: Playground, OrderRecord, TradeSignal,
                                     #   ISignalRepository, InMemory/ESDBSignalRepository
    router/grpc.go                   # Twirp RPC handlers (WriteSignal, GetSignals, PlaceOrder, etc.)
    services/                        # Order queue, Tradier broker, live accounts
    rpc/twirp.go                     # Twirp server setup
  eventmodels/                       # Shared types: TradeSignal, SignalName, StockSymbol
  eventservices/                     # Polygon, Tradier, cache
  eventconsumers/                    # Slack, Tradier, ESDB workers
  eventproducers/                    # API handlers, ESDB producer
  data/                              # Database service (Postgres/GORM)
  telemetry/                         # OTel metrics (SignalsProduced, SignalsConsumed)
  playground.proto                   # Protobuf definitions (TradeSignalProto, TickDelta, etc.)

src/clients/python/
  engine/
    client.py                        # BacktesterPlaygroundClient (Twirp RPC wrapper)
    trading_engine.py                # Main tick loop with on_signal() dispatch
    heartbeat.py                     # StrategyHeartbeat (OTel gauge)
    datasource_heartbeat.py          # DatasourceHeartbeat (OTel gauge)
    otel.py                          # OTel SDK setup
    persistence.py                   # save_backtest_run() for Postgres
  datasources/
    base.py                          # Datasource skeleton (dual-mode pattern)
    ma_crossover.py                  # MA crossover signal producer
    options_ma_crossover.py          # Options MA crossover signals
    credit_spread_signals.py         # Credit spread signals
    covered_call_signals.py          # Covered call signals
    wheel_signals.py                 # Wheel strategy signals
    pdf_wheel_signals.py             # PDF wheel signals
  strategies/
    base_strategy.py                 # BaseStrategy ABC with on_signal() hook
    mean_reversion_v2.py             # MeanReversionStrategyV2 (signal-based)
    options_mean_reversion_v2.py     # OptionsMeanReversionStrategyV2
    credit_spread_v2.py              # CreditSpreadStrategyV2
    covered_call_v2.py               # OptionsStrategyBasicV2
    wheel_v2.py                      # WheelStrategyV2
    pdf_wheel_v2.py                  # PDFWheelStrategyV2
  demos/                             # Demo/launcher scripts
  tests/                             # Unit + behavioral diff tests
  deprecated/                        # V1 strategies (pre-TradeSignal)
  rpc/                               # Generated protobuf stubs
  tools/                             # Utility scripts (playground_metrics.py)

infra/
  analytics-schema.sql               # SQL views + indexes + backtest_runs table
  provision-metabase.py              # Programmatic Metabase dashboard creation (5 dashboards, 24 cards)
  init-metabase.sql                  # Metabase app DB + read-only user setup

observability/
  docker-compose.yaml                # grafana/otel-lgtm all-in-one
  dashboards/                        # Grafana dashboard JSON (auto-provisioned)
  alerting/                          # Grafana alert rules YAML

integration_testing/                 # E2E tests (TestContainers: Postgres, ESDB)
```

---

## Build & Run

```bash
go build ./cmd/main.go               # Build server
go build ./src/go/...                 # Build all packages
task test                             # Unit tests (backtester-api)
task test:e2e                         # E2E tests
task test:integration                 # Integration tests
task app:dev                          # Run dev server (GO_ENV=development)
task gen:proto                        # Regenerate protobuf stubs
```

### Ports
| Port | Service |
|------|---------|
| 8080 | REST API (Gorilla Mux) |
| 5051 | Twirp RPC |
| 5432 | PostgreSQL |
| 2113 | EventStoreDB HTTP |
| 3000 | Grafana |
| 4318 | OTLP HTTP receiver |

---

## TradeSignal Framework

Signals are **global events** — produced independently, consumed by playgrounds.

### Signal Flow
1. **Datasource** produces signals (e.g. MA crossover detected)
2. Signal stored in repository (InMemory for sim, ESDB for live)
3. **Playground** delivers signals via TickDelta, gated by clock time
4. **Strategy** receives signals via `on_signal()` method
5. Strategy calls `PlaceOrder` with `signal_id` linking back to the originating signal

### Running a Simulation
```bash
cd src/clients/python
python demos/demo_mean_reversion_v2.py --symbol COIN --start 2025-01-01 --end 2025-03-01
```

### Replay from Persisted Signals
```bash
# Replay from a sim run (opaque stream name from --save-to-db output)
python demos/demo_mean_reversion_v2.py --symbol COIN --replay-signals trade-signals-sim-<playground-id>

# Replay from live signal stream
python demos/demo_mean_reversion_v2.py --symbol COIN --replay-signals trade-signals
```

### Signal Stream Naming
- **Live signals**: Always saved to global `trade-signals` stream
- **Sim signals**: Saved to opaque per-run streams: `trade-signals-sim-{playground_id}` (only when `--save-to-db` is used)

### Save Backtest Results
```bash
python demos/demo_mean_reversion_v2.py --symbol COIN --save-to-db --client-id my-backtest-run
```

### Signal Types
Defined as typed constants in `src/go/eventmodels/signal_name.go`:
- `ma_crossover` — Moving average crossover
- `start_of_week` — Calendar-based signal
- `price_level_break` — Support/resistance break
- `rsi_threshold` — RSI overbought/oversold

---

## Analytics (Metabase)

5 dashboards provisioned programmatically via `infra/provision-metabase.py`:

| Dashboard | What it shows |
|-----------|--------------|
| Trading Performance | P&L, win rate, profit factor, equity curve with drawdown |
| Slippage Analysis | Open/close slippage per trade, distribution by symbol |
| Portfolio Analytics | Per-symbol P&L, position history, asset class breakdown |
| Strategy Comparison | Backtest runs table, equity curve overlay, parameter comparison |
| Spread Analytics | Spread P&L summary, win/loss ratio, per-spread detail |

### Provision Dashboards
```bash
python infra/provision-metabase.py --url http://localhost:3001 --user admin@example.com --password <pw>
```

### SQL Views
`infra/analytics-schema.sql` provides:
- `v_order_pnl` — Per-order realized P&L (replicates CalcRealizedPL)
- `v_playground_stats` — Aggregated P&L, win rate, profit factor per playground
- `v_trade_fills` — Flattened order+trade rows
- `v_open_slippage`, `v_close_slippage`, `v_all_slippage` — Slippage analysis
- `v_spread_pnl`, `v_spread_stats` — Spread grouping analytics
- `backtest_runs` — Summary table for persisted backtests

---

## Observability

Built on OpenTelemetry, visualized in Grafana.

### Stack
- **Grafana** (port 3000) — Dashboards + alerting
- **Loki** — Log aggregation (structured logs via OTel bridge)
- **Tempo** — Distributed tracing
- **Prometheus** — Metrics
- **OTel Collector** — OTLP receiver (port 4318)

### Key Metrics
- `grodt.strategy.heartbeat` — Strategy liveness (per playground)
- `grodt.datasource.heartbeat` — Datasource script liveness
- `grodt.signals.produced` — Signal production counter (by name, symbol)
- `grodt.signals.consumed` — Signal consumption counter (by strategy)
- `grodt.orders.placed/filled/rejected` — Order lifecycle

### Dashboards
- `grodt-live-simulation` — System health, orders, market data, signals & datasources
- `grodt-mean-reversion` — Strategy-specific panels
- `grodt-covered-call` — Strategy-specific panels

### Alerts
- Strategy heartbeat stale (5m)
- Datasource heartbeat stale (5m)
- Error rate spike

### Run Observability Stack
```bash
cd observability
docker compose up -d
```

---

## Development

### Conda Environment
```bash
conda activate grodt
```

### Taskfile
```bash
brew install go-task    # Mac
task list               # Show all commands
```

### Compile Protobuf
```bash
task gen:proto
```

Or manually:
```bash
cd src/go && protoc --go_out=. --twirp_out=. playground.proto
```

### Profiling
```bash
go tool pprof -seconds 30 -http localhost:8090 myserver http://localhost:8080/debug/pprof/profile
```

---

## Docker & Deployment

### Build Images
```bash
docker build -f Dockerfile.base -t grodt-base-image .
docker build -f Dockerfile.base2 -t grodt-base-image-2 .
docker build -f Dockerfile -t grodt .
```

### Deploy
```bash
./deploy-app.sh <patch|minor|major>
```

### Container Registry
```bash
docker login https://ewr.vultrcr.com/grodt -u $VULTR_REGISTRY_USER -p $VULTR_REGISTRY_PASS
```

### Production Stack (Digital Ocean)
```bash
cd /opt/slack-trading
docker compose -f docker-compose.prod.yaml up -d
```

Services: Go server, PostgreSQL 13, EventStoreDB 24.2.0, grafana/otel-lgtm

---

## Infrastructure

### Database
- **PostgreSQL** — Orders, trades, playgrounds, backtest_runs, analytics views
- **EventStoreDB** — Event sourcing (signals, account state)
- **Metabase** — Analytics dashboards (runs on Windows desktop, connects to DO Postgres)

### Kubernetes (Vultr)
- Flux CD for GitOps
- Sealed Secrets for secret management
- Manifests in `.clusters/production/`

### Environment Variables
Key vars (loaded from `.env` via godotenv):
- `TRADING_PROJECT_DIR` — Repo root
- `GO_ENV` — `development` | `production`
- `POLYGON_API_KEY` — Market data
- `POSTGRES_HOST/USER/PASSWORD/DB` — Database
- `OTEL_EXPORTER_OTLP_ENDPOINT` — OTel collector
- `EVENTSTOREDB_URL` — EventStoreDB

---

## Integrations

| Service | Purpose |
|---------|---------|
| Polygon.io | Market data (stocks, options, candles) |
| Tradier | Broker (live + sandbox trading) |
| EventStoreDB | Event sourcing (signals, account state) |
| Slack | Alerts and notifications |
| Google Sheets | Trade logging |

---

## Version History

| Version | Focus | Date |
|---------|-------|------|
| v3.0 | TradeSignal Framework — signal decoupling, strategy migration, replay | 2026-03-31 |
| v2.0 | Metabase Analytics — dashboards, backtest persistence, spread grouping | 2026-03-30 |
| v1.1 | Dashboard Enhancements — filtering, per-strategy panels | 2026-03-30 |
| v1.0 | Live Simulation Observability — OTel, Grafana, production deploy | 2026-03-28 |
