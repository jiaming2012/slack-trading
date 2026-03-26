# Codebase Structure

**Analysis Date:** 2026-03-25

## Directory Layout

```
slack-trading/
├── cmd/                        # Server entrypoint and dev runner
│   ├── main.go                 # Main server binary
│   └── run-dev.sh              # Dev startup script
├── src/
│   ├── go/                     # All Go source packages
│   │   ├── backtester-api/     # Core backtester domain (models, router, services, rpc)
│   │   ├── data/               # DatabaseService (persistence + caching layer)
│   │   ├── dbutils/            # PostgreSQL connection and migration
│   │   ├── eventconsumers/     # Background workers (Slack, Tradier, ESDB, etc.)
│   │   ├── eventmodels/        # Shared domain types and event definitions
│   │   ├── eventproducers/     # REST API route handlers and event sources
│   │   ├── eventpubsub/        # In-process pub/sub event bus
│   │   ├── eventservices/      # External API clients (Polygon, Tradier, ORATS)
│   │   ├── eventstore/         # EventStoreDB integration
│   │   ├── playground/         # Generated protobuf/Twirp stubs
│   │   ├── playground.proto    # Protobuf service definition
│   │   ├── coinbase/           # Coinbase API client (legacy)
│   │   ├── coingecko/          # CoinGecko API client (legacy)
│   │   ├── handler/            # Slack command handler
│   │   ├── indicators/         # Technical indicators (Bollinger, RSI)
│   │   ├── logger/             # Logging utilities
│   │   ├── models/             # Legacy shared models
│   │   ├── sheets/             # Google Sheets integration
│   │   ├── slack/              # Slack utilities
│   │   ├── strategy/           # Legacy strategy code
│   │   ├── testing/            # Test utilities
│   │   ├── utils/              # Common utilities (env, HTTP, crypto, CSV)
│   │   ├── worker/             # WebSocket/datafeed workers
│   │   └── options-config.yaml # Options trading configuration
│   ├── clients/
│   │   └── python/             # Python strategy clients
│   │       ├── rpc/            # Generated protobuf/Twirp stubs (Python)
│   │       ├── trading_engine.py
│   │       ├── backtester_playground_client_grpc.py
│   │       ├── options_strategy_basic_v7.py
│   │       └── ... (strategies, visualizations, tests)
│   ├── cmd/                    # Standalone Go commands and Python scripts
│   │   ├── backtester/         # Standalone backtester
│   │   ├── backtester-sandbox/ # Backtester sandbox (has own .git)
│   │   ├── fetch_market_data/  # Market data fetcher
│   │   ├── import_signals/     # Signal importer
│   │   ├── import_trading_view_data/ # TradingView data importer
│   │   └── stats/              # Statistical analysis scripts (Python)
│   └── python/                 # Python utilities (market calendars)
├── integration_testing/        # E2E tests using TestContainers
├── deprecated/                 # Archived code
├── eventstoredb/               # EventStoreDB Docker Compose setup
├── .clusters/                  # Kubernetes/Flux CD manifests
│   └── production/flux-system/ # GitOps deployment config
├── .planning/                  # GSD planning documents
├── go.mod                      # Go module definition
├── go.sum                      # Go dependency checksums
├── taskfile.yml                # Task runner configuration
├── Dockerfile                  # Production Docker image
├── Dockerfile.base             # Base image (stage 1)
├── Dockerfile.base2            # Base image (stage 2)
├── Dockerfile.dev              # Development Docker image
└── grodt.yml                   # Conda environment definition
```

## Directory Purposes

**`cmd/`:**
- Purpose: Server entrypoint
- Contains: `main.go` (465 lines) — bootstraps entire application; `run-dev.sh` — sets env vars for development
- Key files: `cmd/main.go`

**`src/go/backtester-api/`:**
- Purpose: Core backtester domain — the heart of the system
- Contains: Domain models, RPC handlers, business services, database layer
- Key subdirectories:
  - `models/` — 80+ files: `Playground`, `OrderRecord`, `TradeRecord`, `CandleRepository`, `BacktesterAccount`, `Clock`, broker interfaces, mock implementations
  - `router/` — `grpc.go` (Twirp RPC handlers, 1266 lines), `handler.go` (REST + live order handling)
  - `rpc/` — `twirp.go` (Twirp server setup with panic recovery middleware)
  - `services/` — `tradier_broker.go`, `order_queue.go`, `playground.go`, `accounts.go`, `live_repositories.go`
  - `mock/` — Test mocks
  - `db/` — `init.sql` (database initialization)

**`src/go/data/`:**
- Purpose: Database service layer — persistence and in-memory caching
- Contains: `DatabaseService` struct (1634 lines) managing playgrounds, orders, trades, live accounts
- Key files: `src/go/data/database_service.go`

**`src/go/dbutils/`:**
- Purpose: PostgreSQL connection, GORM auto-migration
- Contains: `InitPostgres()`, `InitPostgresWithUrl()`, `UpdateOrder()`
- Key files: `src/go/dbutils/postgres.go`

**`src/go/eventconsumers/`:**
- Purpose: Background worker goroutines that subscribe to the event bus
- Contains: 21 files — Slack notifier, Tradier API worker, ESDB consumer, account worker, signal processor, trading bot, option alerts
- Key files: `tradier_api_worker.go` (polls Tradier for order updates), `slacknotifier.go`, `global_request_dispatcher.go`

**`src/go/eventmodels/`:**
- Purpose: Shared domain types and event definitions used across all packages
- Contains: 100+ files — `StockSymbol`, `OptionSymbol`, `OptionContractV3`, `Candle`, `FIFOQueue`, `GlobalResponseDispatcher`, all event name constants, request/response DTOs
- Key files: `event_names.go`, `globaldispatcher.go`, `fifo_queue.go`, `instrument.go`, `candle.go`

**`src/go/eventproducers/`:**
- Purpose: REST API route handlers organized by domain area
- Contains: Subdirectories for each API domain: `accountapi/`, `alertapi/`, `datafeedapi/`, `optionsapi/`, `signalapi/`, `strategyapi/`, `tradeapi/`, `slack/`
- Key files: `src/go/eventproducers/esdb_producer.go`, `src/go/eventproducers/api_request_2.go`

**`src/go/eventpubsub/`:**
- Purpose: In-process publish/subscribe event bus
- Contains: 4 files — global `EventBus` singleton, publish/subscribe helpers
- Key files: `services.go` (Publish/Subscribe/Unsubscribe), `models.go`, `sync.go`

**`src/go/eventservices/`:**
- Purpose: External API clients and data fetching
- Contains: 35 files — Polygon (options, stocks, cache), Tradier (orders, quotes), ORATS, Financial Modeling Prep, market calendar, strategy signals
- Key files: `polygon_cache.go` (3-bucket thread-safe cache), `fetch_option_chain_with_params.go` (parallel fetches), `polygon.go`, `tradier.go`

**`src/go/playground/`:**
- Purpose: Generated protobuf and Twirp Go stubs
- Contains: `playground.pb.go`, `playground.twirp.go`, `playground_grpc.pb.go`
- Generated: Yes (via `task gen:proto`)
- Committed: Yes

**`src/go/indicators/`:**
- Purpose: Technical indicator calculations
- Contains: Bollinger Bands, RSI (with tests)

**`src/go/utils/`:**
- Purpose: Common Go utilities
- Contains: Environment variable loading, HTTP helpers, encryption, CSV parsing, candle utilities

**`src/clients/python/`:**
- Purpose: Python trading strategy clients
- Contains: Twirp RPC client wrapper, trading engine, multiple strategy implementations, visualization tools, tests
- Key files: `backtester_playground_client_grpc.py` (RPC wrapper), `trading_engine.py` (main engine), `options_strategy_basic_v7.py` (covered call strategy)

**`src/clients/python/rpc/`:**
- Purpose: Generated Python protobuf and Twirp stubs
- Contains: `playground_pb2.py`, `playground_twirp.py`
- Generated: Yes (via `task gen:proto`)

**`integration_testing/`:**
- Purpose: End-to-end tests using TestContainers (Docker-based)
- Contains: 18 test files covering live account operations, order lifecycle, options simulation
- Key files: `setup.go` (TestContainers setup for app + Postgres + ESDB), `utils.go`

## Key File Locations

**Entry Points:**
- `cmd/main.go`: Server entrypoint — starts REST (port 8080), Twirp (port 5051), all event consumers
- `cmd/run-dev.sh`: Development runner script
- `src/clients/python/trading_engine.py`: Python trading engine entrypoint

**Configuration:**
- `src/go/options-config.yaml`: Options trading configuration
- `taskfile.yml`: Task runner definitions (build, test, deploy, simulation)
- `go.mod`: Go module `github.com/jiaming2012/slack-trading` (Go 1.22.4)
- `grodt.yml`: Conda environment definition
- `Dockerfile`: Production container image
- `Dockerfile.dev`: Development container image

**Proto Definition:**
- `src/go/playground.proto`: Protobuf service definition (PlaygroundService with 20 RPC methods)

**Core Logic:**
- `src/go/backtester-api/models/playground.go`: Playground domain model (2909 lines)
- `src/go/backtester-api/router/grpc.go`: Twirp RPC handler implementations (1266 lines)
- `src/go/data/database_service.go`: Database service layer (1634 lines)
- `src/go/eventservices/polygon_cache.go`: Polygon API cache with disk persistence

**Testing:**
- `integration_testing/`: E2E tests (TestContainers)
- `src/go/backtester-api/models/*_test.go`: Unit tests for domain models
- `src/go/eventservices/integration_tests/`: Integration tests for external services
- `src/clients/python/test_*.py`: Python strategy tests

## Naming Conventions

**Files:**
- Go: `snake_case.go` — e.g., `database_service.go`, `order_record.go`, `polygon_cache.go`
- Go tests: `*_test.go` co-located with source
- Python: `snake_case.py` — e.g., `trading_engine.py`, `backtester_playground_client_grpc.py`
- Python tests: `test_*.py` co-located with source

**Directories:**
- Go packages: `lowercase` or `snake_case` — e.g., `eventmodels`, `eventconsumers`, `backtester-api`
- API subdirectories: `{domain}api` — e.g., `accountapi`, `tradeapi`, `alertapi`

**Go Packages:**
- Import path: `github.com/jiaming2012/slack-trading/src/go/<package>`
- Backtester models: `github.com/jiaming2012/slack-trading/src/go/backtester-api/models`

## Where to Add New Code

**New RPC Method:**
1. Add method to `src/go/playground.proto`
2. Run `task gen:proto` to regenerate stubs
3. Implement handler in `src/go/backtester-api/router/grpc.go` on the `Server` struct
4. Add Python client wrapper in `src/clients/python/backtester_playground_client_grpc.py`

**New Domain Model:**
- GORM model: `src/go/backtester-api/models/` — create new file with `snake_case.go` naming
- Add auto-migration in `src/go/dbutils/postgres.go`
- Shared type (non-GORM): `src/go/eventmodels/`

**New Trading Strategy (Python):**
- Strategy file: `src/clients/python/<strategy_name>.py`
- Test file: `src/clients/python/test_<strategy_name>.py`
- Follow pattern of existing strategies (e.g., `credit_spread_strategy.py`, `mean_reversion_strategy.py`)

**New Event Consumer (Go):**
- Create file in `src/go/eventconsumers/` following the worker pattern:
  - Struct with `wg *sync.WaitGroup` field
  - `Start(ctx context.Context)` method that calls `pubsub.Subscribe()` and runs goroutine
  - Constructor function `NewXxxClient(wg, ...)`
- Register in `cmd/main.go`

**New REST API Route:**
- Create handler subdirectory in `src/go/eventproducers/<domain>api/`
- Add `handler.go` with `SetupHandler(router *mux.Subrouter)` function
- Register in `cmd/main.go` via `router.PathPrefix("/<path>").Subrouter()`

**New External Service Client:**
- Add to `src/go/eventservices/`
- Follow pattern of existing clients (e.g., `polygon.go`, `tradier.go`)

**New Integration Test:**
- Add test file in `integration_testing/` following `live_account_*_test.go` pattern
- Use `setup.go` helpers for TestContainers setup
- Add task entry in `taskfile.yml` under `test:e2e:*`

**Utilities:**
- Go utilities: `src/go/utils/`
- Python utilities: `src/clients/python/utils.py`

## Special Directories

**`src/go/playground/`:**
- Purpose: Generated protobuf and Twirp Go stubs
- Generated: Yes — via `task gen:proto`
- Committed: Yes
- Do NOT edit manually

**`src/clients/python/rpc/`:**
- Purpose: Generated protobuf and Twirp Python stubs
- Generated: Yes — via `task gen:proto`
- Committed: Yes
- Do NOT edit manually

**`deprecated/`:**
- Purpose: Archived/unused code
- Generated: No
- Committed: Yes

**`.clusters/production/flux-system/`:**
- Purpose: Kubernetes Flux CD GitOps configuration
- Contains: `gotk-components.yaml`, `gotk-sync.yaml`, `kustomization.yaml`

**`eventstoredb/`:**
- Purpose: EventStoreDB Docker Compose setup for local development
- Contains: `docker-compose.yaml`

**`.cache/polygon/`:**
- Purpose: Disk cache for Polygon API aggregate bar data
- Generated: Yes (at runtime)
- Committed: No

**`src/cmd/`:**
- Purpose: Standalone command-line tools and scripts (separate from the main server)
- Contains: `backtester/` (standalone backtester), `fetch_market_data/`, `import_signals/`, `stats/` (Python statistical analysis)
- Note: `backtester-sandbox/` has its own `.git` — it is a separate sub-repo

---

*Structure analysis: 2026-03-25*
