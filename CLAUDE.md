# Project: slack-trading (grodt)

Event-driven trading platform: Go backend server + Python strategy clients, communicating via Twirp RPC. Deploys to Vultr Kubernetes.

## Quick Reference

- **Go module**: `github.com/jiaming2012/slack-trading` (Go 1.22.4)
- **Import convention**: `github.com/jiaming2012/slack-trading/src/go/<package>`
- **Proto**: `src/go/playground.proto` → Go stubs in `src/go/playground/`, Python stubs in `src/clients/python/rpc/`
- **Server entrypoint**: `cmd/main.go` — starts REST (:8080), Twirp (:5051), event consumers
- **Python env**: `grodt` conda env (`/Users/jamal/miniconda3/envs/grodt/bin/python`), numpy pinned to 1.26.4 (pandas_ta compat)

## Layout

```
cmd/main.go                     # Server entrypoint
cmd/run-dev.sh                  # Dev runner (sets GO_ENV, OPTIONS_CONFIG_PATH for worktrees)
src/go/
  backtester-api/               # Core backtester service
    models/                     #   Domain: Playground, OrderRecord, CandleRepository, Account
    router/grpc.go              #   Twirp RPC handlers (1248 lines — the main API surface)
    services/                   #   Order queue, Tradier broker, live accounts
    rpc/twirp.go                #   Twirp server setup
  eventservices/                # Polygon client, Tradier, polygon_cache.go
  eventmodels/                  # Shared domain types (StockSymbol, OptionSymbol, etc.)
  data/                         # Database service layer (Postgres)
  eventconsumers/               # Slack, Tradier, ESDB workers
  eventproducers/               # API handlers, Slack commands
  eventpubsub/                  # In-process pub/sub
  playground.proto              # Protobuf definition
  options-config.yaml           # Options trading config (dev)
src/clients/python/
  backtester_playground_client_grpc.py  # Twirp client (place_order, tick, fetch_ladder)
  options_strategy_basic_v7.py         # Covered call strategy (~1500 lines)
  demo_covered_call.py                 # Standalone demo script
  test_demo_covered_call.py            # Regression test (pins reference metrics)
  trading_engine.py                    # Main trading engine
  rpc/                                 # Generated protobuf stubs
integration_testing/            # E2E tests (live account, Tradier, Postgres)
deprecated/                     # Archived code
```

## Build & Test

```bash
go build ./cmd/main.go               # Build server
go build ./src/go/...                 # Build all packages
task test                             # Unit tests (backtester-api)
task test:e2e                         # E2E tests
task test:integration                 # Integration tests (eventservices)
task app:dev                          # Run dev server (GO_ENV=development)
task gen:proto                        # Regenerate protobuf stubs
```

## Key Subsystems

### Backtester Loop (Python → Go → Python)
1. Python creates playground via `CreatePlayground` RPC (fetches Polygon data, builds candle repos)
2. Python calls `NextTick` in a loop — server advances clock, returns new candles + trades
3. Python strategy evaluates signals from candle indicators, calls `PlaceOrder` RPC
4. Go fills orders via simulated tick matching in `simulateTick` (playground.go)

### Polygon API Cache (`eventservices/polygon_cache.go`)
- 3 buckets: option contracts, stock ticks, aggregate bars
- Thread-safe (`sync.RWMutex` per bucket), composite keys
- Integrated in `FetchOptionChainV1` and `populateTickDataToOptionChainMap`
- `DeepCopy()` on cached values to prevent mutation

### Parallel Option Fetches (`eventservices/fetch_option_chain_with_params.go`)
- `errgroup` worker pool (concurrency limit 5) replaces sequential loop with 50ms sleeps

## Environment & Config

- `PROJECT_DIR` → repo root (e.g., `/Users/jamal/projects/slack-trading` or worktree path)
- `OPTIONS_CONFIG_FILE` → config filename (e.g., `options-config.yaml`)
- `OPTIONS_CONFIG_PATH` → full path override (set by `cmd/run-dev.sh` for worktree support)
- Options config default path: `${PROJECT_DIR}/src/go/${OPTIONS_CONFIG_FILE}`

## Worktree Notes

- Branch `claude/nifty-diffie` lives in worktree `.claude/worktrees/nifty-diffie`
- `$PROJECT_DIR` points to the repo root (main repo or worktree)
- `cmd/run-dev.sh` auto-resolves `OPTIONS_CONFIG_PATH` relative to its own location
- Taskfile `dir:` fields should use relative paths (not `$PROJECT_DIR`) for worktree compat
- Cannot `git checkout claude/nifty-diffie` from main repo while worktree is active

## Gotchas

- **Polygon.io 403**: Some symbol/date combos (e.g., MSFT 2024) return Forbidden. AAPL 2025 dates work.
- **Port 8080 conflict**: REST server logs error but doesn't crash (Twirp on 5051 still works)
- **`gh` alias**: User's shell aliases `gh` to `git checkout`. Use `/usr/local/bin/gh` or `command gh`.
- **numpy version**: Must be 1.26.4 — pandas_ta requires <2.0, matplotlib requires >=1.23
- **`eventservices/integration_tests`**: Contains an intentionally-failing stub test (`require.Fail(t, "finish the test")`)
- **Server startup**: Loads all persisted playgrounds from Postgres — emits "no candles found" warnings for live playgrounds (normal when market data hasn't caught up)

## Deploy

- Kubernetes on Vultr (Flux CD GitOps)
- Images: `ewr.vultrcr.com/grodt/`
- Sealed secrets: `sealedsecret.yaml`

<!-- GSD:project-start source:PROJECT.md -->
## Project

**Live Simulation Observability**

An observability layer for the slack-trading platform's live simulation mode. Surfaces real-time visibility into order lifecycle, strategy decisions, market data flow, and system health — built on OpenTelemetry, visualized in Grafana with Loki for logs. Solves the core problem: "is the strategy actually running?" especially during quiet periods with no trades.

**Core Value:** When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

### Constraints

- **Tech stack**: OpenTelemetry (already partially adopted) → Loki (logs) + Grafana (dashboards/alerts)
- **Python compatibility**: numpy pinned to 1.26.4 (pandas_ta compat) — OTel Python packages must be compatible
- **Local dev first**: Docker Compose for local iteration, then Digital Ocean for production
- **Infrastructure provisioning**: Digital Ocean MCP server for creating cloud resources
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.22.4 - Server, backtester engine, event consumers/producers, all backend logic
- Python 3.10 - Strategy clients, backtesting orchestration, optimization, metrics visualization
- Protobuf (proto3) - RPC interface definition (`src/go/playground.proto`)
- YAML - Configuration (`src/go/options-config.yaml`, `grodt.yml`, `taskfile.yml`)
- Bash - Dev scripts (`cmd/run-dev.sh`, `deploy-app.sh`)
## Runtime
- Go 1.22.4 (installed in Docker via `Dockerfile.base2`)
- Module: `github.com/jiaming2012/slack-trading`
- Module path: `go.mod` at repo root
- Conda environment `grodt` defined in `grodt.yml`
- Python 3.10 (conda-forge channel)
- Alternative: pip virtualenv in `src/clients/python/env/` with `src/clients/python/requirements.txt`
- Conda binary: `/Users/jamal/miniconda3/envs/grodt/bin/python`
- Go modules - `go.mod` / `go.sum` (lockfile present)
- Conda - `grodt.yml` for Python env management
- pip - `src/clients/python/requirements.txt` for virtualenv fallback
## Frameworks
- Twirp RPC v8.1.3 (`github.com/twitchtv/twirp`) - Primary RPC framework (JSON over HTTP, port 5051)
- Gorilla Mux v1.8.1 (`github.com/gorilla/mux`) - REST HTTP router (port 8080)
- GORM v1.25.12 (`gorm.io/gorm`) - ORM for PostgreSQL
- EventBus (`github.com/asaskevich/EventBus`) - In-process pub/sub event bus
- Testify v1.9.0 (`github.com/stretchr/testify`) - Go test assertions and mocking
- TestContainers v0.35.0 (`github.com/testcontainers/testcontainers-go`) - Docker-based integration tests
- pytest (Python) - Strategy client tests
- Task (Taskfile v3) - Task runner (`taskfile.yml`)
- protoc + twirp plugin - Protobuf code generation for Go
- protoc + twirpy plugin - Protobuf code generation for Python
- Docker - Container builds (`Dockerfile`, `Dockerfile.base`, `Dockerfile.base2`)
- OpenTelemetry v1.27.0 - Tracing and metrics (OTLP HTTP exporters)
- Logrus v1.9.3 (`github.com/sirupsen/logrus`) - Structured logging
- otellogrus (`github.com/uptrace/opentelemetry-go-extra/otellogrus`) - OTel-Logrus bridge
- net/http/pprof - Go profiling endpoints (imported in `cmd/main.go`)
## Key Dependencies
- `github.com/polygon-io/client-go` v1.16.6 - Polygon.io market data SDK
- `github.com/gorilla/websocket` v1.5.3 - WebSocket connections (market data streaming)
- `github.com/EventStore/EventStore-Client-Go/v4` v4.1.0 - EventStoreDB client
- `gorm.io/driver/postgres` v1.5.11 - PostgreSQL driver for GORM (uses pgx v5)
- `github.com/jackc/pgx/v5` v5.7.2 - PostgreSQL driver (underlying)
- `github.com/google/uuid` v1.6.0 - UUID generation for playground IDs
- `google.golang.org/protobuf` v1.35.2 - Protobuf runtime
- `google.golang.org/api` v0.185.0 - Google APIs (Sheets, Drive)
- `golang.org/x/oauth2` v0.22.0 - OAuth2 for Google service account auth
- `github.com/joho/godotenv` v1.5.1 - .env file loading
- `gopkg.in/yaml.v3` v3.0.1 - YAML parsing for options config
- `github.com/spf13/cobra` v1.8.1 - CLI command framework
- `github.com/gocarina/gocsv` v0.0.0 - CSV parsing (ORATS data)
- `github.com/montanaflynn/stats` v0.7.1 - Statistical functions
- `golang.org/x/sync` v0.10.0 - errgroup for concurrent option fetches
- `twirp` 0.0.7 - Twirp RPC client for Python
- `protobuf` 5.29.3 - Protobuf runtime (matches Go version)
- `polygon-api-client` 0.2.11 - Polygon.io Python SDK
- `pandas` 1.3.5 - Data manipulation
- `pandas_ta` 0.3.14b0 - Technical analysis indicators
- `ta-lib` (conda) - TA-Lib C library bindings
- `numpy` 1.23.5 (conda) / 1.21.6 (pip) - Numerical computing (must be <2.0 for pandas_ta)
- `scikit-optimize` 0.10.2 - Bayesian hyperparameter optimization
- `scikit-learn` 1.6.0 - Machine learning models
- `matplotlib` 3.5.3 - Charting
- `plotly` 5.18.0 - Interactive visualizations
- `loguru` 0.7.3 - Python structured logging
- `structlog` 24.4.0 - Alternative structured logging
- `websocket-client` 1.6.1 / `websockets` 11.0.3 - WebSocket connections
- `github.com/jinzhu/copier` v0.4.0 - Deep copy for cache safety
- `github.com/patrickmn/go-cache` v2.1.0 - In-memory caching
- `github.com/go-resty/resty/v2` v2.15.2 - HTTP client
- `github.com/go-playground/validator/v10` v10.22.1 - Input validation
- `github.com/olekukonko/tablewriter` v0.0.5 - CLI table formatting
## Configuration
- `.env` file loaded via `godotenv` (at `$PROJECT_DIR/.env`)
- `GO_ENV` controls environment: `development` | `production`
- `PROJECT_DIR` points to repo root
- `OPTIONS_CONFIG_FILE` / `OPTIONS_CONFIG_PATH` for options trading config
- `LOG_LEVEL` controls logrus verbosity
- `taskfile.yml` - All build/test/deploy tasks
- `Dockerfile` - Production image (FROM `grodt-base-image-2:3.9.0`)
- `Dockerfile.base` - Base image with Python 3.10, Conda, TA-Lib C library (Ubuntu 20.04)
- `Dockerfile.base2` - Second-layer base with Go 1.22.4, go mod download, conda env update
- `grodt.yml` - Conda environment specification
- Command: `task gen:proto`
- Input: `src/go/playground.proto`
- Go output: `src/go/playground/playground.pb.go`, `playground.twirp.go`
- Python output: `src/clients/python/rpc/playground_pb2.py`, `playground_twirp.py`
## Platform Requirements
- macOS (darwin) - Primary dev platform
- Go 1.22.4
- Conda (miniconda3) with `grodt` environment
- Docker for local databases (`task db:start`)
- PostgreSQL 13 (via Docker or port-forward to cluster)
- EventStoreDB 24.2.0 (via Docker or port-forward to cluster)
- protoc compiler with twirp and twirpy plugins
- Kubernetes on Vultr
- Container registry: `ewr.vultrcr.com/grodt/`
- Flux CD for GitOps deployment
- Sealed Secrets for secret management (`.clusters/production/sealedsecret.yaml`)
- Multi-layer Docker images: `grodt-base-image` -> `grodt-base-image-2` -> app image
- 8080 - REST API (Gorilla Mux)
- 5051 - Twirp RPC server
- 5432 - PostgreSQL
- 2113 - EventStoreDB HTTP
- 1113 - EventStoreDB TCP
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Use `snake_case.go` for all Go source files: `playground.go`, `order_record.go`, `database_service.go`
- Test files: `*_test.go` co-located with source: `playground_test.go`
- Mock files: `mock_*.go` co-located with models: `mock_database.go`, `mock_broker.go`, `mock_options_broker.go`
- Interface files: `*_interface.go`: `database_service_interface.go`, `broker_interface.go`, `live_account_interface.go`
- Error files: `error.go` or `errors.go` per package
- Use `snake_case.py` for source: `trading_engine.py`, `risk_management.py`
- Test files: `test_*.py` prefix: `test_kelly_sizing.py`, `test_partial_exit_manager.py`
- Lowercase, single-word where possible: `models`, `data`, `utils`, `router`
- Multi-word with hyphens in directory names: `backtester-api` (imported as `backtester_router` when aliased)
- Event-prefixed packages: `eventmodels`, `eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`
- PascalCase for exported: `NewPlayground()`, `PlaceOrder()`, `GetOrder()`
- camelCase for unexported: `commitTradableOrderToOrderQueue()`, `updateOpenOrdersCache()`
- Constructor pattern: `New<Type>()` returns `*Type` and `error`: `NewPlayground(...)`, `NewOrderRecord(...)`, `NewCandleRepository(...)`
- `Get` prefix for simple accessors: `GetSymbol()`, `GetTicker()`, `GetStatus()`
- `Fetch` prefix for operations that hit external resources (DB, API): `FetchEquity()`, `FetchPositions()`, `FetchCandles()`
- camelCase for local and unexported fields: `startTime`, `stockSymbol`, `playgroundClient`
- PascalCase for exported struct fields: `Balance`, `Orders`, `ClientID`
- PascalCase structs: `Playground`, `OrderRecord`, `TradeRecord`, `DatabaseService`
- Interfaces prefixed with `I`: `IDatabaseService`, `IBroker`, `ILiveAccount`, `IOptionsBroker`, `IReconcilePlayground`
- String-based enums as typed strings: `type StockSymbol string`, `type OptionSymbol string`
- Constants as PascalCase vars: `PlaygroundEnvironmentSimulator`, `PlaygroundEnvironmentLive`
- `snake_case` for functions and methods: `kelly_fraction()`, `compute_exit_plan()`, `check_exits()`
- Classes in PascalCase: `ExitPlan`, `ExitTier`, `BacktesterPlaygroundClient`
- Mixed convention -- two styles coexist:
- **Use the `Err` prefix style for new code** (matches Go convention and newer codebase patterns)
## Code Style
- Go: Standard `gofmt` (no custom formatter config detected)
- Python: No `.flake8`, `pyproject.toml`, or formatter config detected
- No `.editorconfig`, `.prettierrc`, or `.golangci.yml` present
- No linter configuration files detected
- Rely on Go compiler warnings and `go vet` implicitly
## Import Organization
- `log "github.com/sirupsen/logrus"` -- universal across the codebase
- `pb "github.com/jiaming2012/slack-trading/src/go/playground"` -- protobuf stubs
- `backtester_router "github.com/jiaming2012/slack-trading/src/go/backtester-api/router"` -- disambiguating
- All internal imports: `github.com/jiaming2012/slack-trading/src/go/<package>`
- None (no `tsconfig.json` or Go module path aliasing beyond the module path)
## Error Handling
- Wrap errors with context using `fmt.Errorf("...: %w", err)`:
- Sentinel errors defined as package-level vars in `error.go` / `errors.go`:
- Check errors with `errors.Is()` for sentinel errors
- Functions return `(result, error)` tuples consistently
- Validation errors returned from constructors (e.g., `NewOrderRecord` validates all params)
## Logging
- Always import as: `log "github.com/sirupsen/logrus"`
- Use level-specific methods: `log.Infof()`, `log.Debugf()`, `log.Errorf()`, `log.Warnf()`, `log.Fatalf()`
- Include context in log messages with format strings:
- Structured logging with `WithFields` used in the GORM logger adapter (`src/go/logger/logger.go`):
- OpenTelemetry integration via `otellogrus` hook (configured in `cmd/main.go`)
- `Info`: Significant state changes (order placed, playground created, loading operations)
- `Debug`: Internal flow tracing (cache hits, mock operations, request IDs)
- `Error`: Failed operations that don't crash the program
- `Fatal`: Startup failures only (missing config, DB connection failure)
- `Warn`: Degraded operations (slow SQL, skipping duplicates)
## Configuration
- Loaded via `github.com/joho/godotenv` from `.env` files
- Accessed through `utils.GetEnv("VAR_NAME")` helper (returns value + error)
- Key vars: `PROJECT_DIR`, `GO_ENV`, `POLYGON_API_KEY`, `POSTGRES_HOST`, `LOG_LEVEL`
- `.env` files are gitignored
- Options config: `src/go/options-config.yaml` loaded via `gopkg.in/yaml.v3`
- Conda environment: `grodt.yml`
- GORM: `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`, `gorm:"column:balance;type:numeric;not null"`
- JSON: `json:"msg"`, `json:"-"` for excluded fields
- Combined: fields can have both `json` and `gorm` tags
## Comments
- Comment blocks before complex test functions explaining the scenario being tested:
- Inline comments for non-obvious logic or workarounds
- TODO comments for incomplete implementations: `// need to get the external order id. Maybe place it on the live order?`
- Not applicable (no TypeScript)
- Minimal usage. Some types and exported functions lack doc comments.
- When present, follow standard Go convention: `// RouterSetupItem defines a single route handler configuration.`
- Module-level docstrings in test files explaining purpose and usage:
## Function Design
- Always return `(*Type, error)` even when error may be nil
- Validate all inputs in the constructor
- Many parameters (sometimes 15+) passed positionally -- no options pattern used
- Example: `NewOrderRecord(id, externalOrderID, clientRequestID, playgroundID, class, accountType, timestamp, symbol, side, quantity, orderType, duration, requestedPrice, ...)`
- Defined in separate `*_interface.go` files in `src/go/backtester-api/models/`
- Used for dependency injection: `IDatabaseService`, `IBroker`, `IOptionsBroker`
- Mock implementations live alongside production code in the same package
## Module Design
- Domain types in `eventmodels/` (shared across packages)
- Service logic in `eventservices/`
- Consumer workers in `eventconsumers/`
- HTTP handler producers in `eventproducers/` with sub-packages per API domain
- Backtester core in `backtester-api/models/`, `backtester-api/services/`, `backtester-api/router/`
## Git Workflow
- Feature branches: `claude/<descriptive-name>` (e.g., `claude/nifty-diffie`)
- Main branches: `main`, `dev`
- Short imperative messages, often single-quoted: `'update readme'`, `'update for running in live mode'`
- No conventional commits prefix (no `feat:`, `fix:`, etc.)
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Two-process architecture: Go server manages state, order execution, and market data; Python clients implement trading strategies
- In-process pub/sub event bus for decoupled communication between Go components
- Twirp RPC (port 5051) as the primary API surface for backtesting and live trading
- REST API (Gorilla mux, port 8080) for signals, data feeds, alerts, accounts, and Slack integration
- GORM/PostgreSQL for persistence with auto-migration on startup
- EventStoreDB for event sourcing of signals and account state
## Layers
- Purpose: Accept external requests from Python clients and HTTP consumers
- Location: `src/go/backtester-api/router/grpc.go` (Twirp handlers, 1266 lines), `src/go/backtester-api/rpc/twirp.go` (server setup), `src/go/backtester-api/router/handler.go` (REST handlers for `/playground`)
- Contains: `Server` struct implementing all `PlaygroundService` RPC methods; REST route setup
- Depends on: `data.DatabaseService`, `eventservices.PolygonOptionsClient`, `models.*`
- Used by: Python clients via Twirp, HTTP clients via REST
- Purpose: Core business entities and trading logic
- Location: `src/go/backtester-api/models/` (80+ files)
- Contains: `Playground` (2909 lines), `OrderRecord`, `TradeRecord`, `CandleRepository`, `BacktesterAccount`, `Clock`, `LiveAccount`, broker interfaces
- Depends on: `eventmodels` (shared types), `models` (legacy shared models)
- Used by: Router, DatabaseService, Services
- Purpose: Persistence, caching, and orchestration of playgrounds and orders
- Location: `src/go/data/database_service.go` (1634 lines)
- Contains: `DatabaseService` struct with in-memory caches for playgrounds, orders, trades, live accounts; GORM queries
- Depends on: `gorm.DB`, `models.*`, `eventmodels.*`, `dbutils`
- Used by: RPC handlers, backtester router
- Purpose: Broker integration, order queue processing, live account management
- Location: `src/go/backtester-api/services/`
- Contains: `TradierBroker` (live broker), `MockBroker`, order queue draining, live repository management
- Key files: `services/tradier_broker.go`, `services/order_queue.go`, `services/playground.go`, `services/accounts.go`
- Depends on: `models.*`, external Tradier API
- Used by: DatabaseService, Router
- Purpose: In-process decoupled communication between Go components
- Location: `src/go/eventpubsub/` (4 files)
- Contains: Thin wrapper around `EventBus` library; `Publish`, `Subscribe`, `Unsubscribe` functions
- Pattern: Global singleton `bus` initialized via `Init()`; async subscriptions
- Depends on: `github.com/asaskevich/EventBus`
- Used by: Event consumers, event producers, global dispatcher
- Purpose: Background workers that subscribe to pub/sub events and perform side effects
- Location: `src/go/eventconsumers/` (21 files)
- Contains: `SlackNotifierClient`, `TradierApiWorker`, `GlobalDispatchWorker`, `AccountWorkerClient`, ESDB consumers
- Pattern: Each consumer has a `Start(ctx)` method that subscribes to topics and runs a goroutine
- Depends on: `eventpubsub`, `eventmodels`, external services (Slack, Tradier, ESDB)
- Purpose: REST API handlers and external event sources that publish to the event bus
- Location: `src/go/eventproducers/` (22 files across subdirectories)
- Contains: API route handlers (`tradeapi`, `accountapi`, `datafeedapi`, `alertapi`, `signalapi`, `optionsapi`, `strategyapi`), Slack command handler, ESDB producer
- Depends on: `eventpubsub`, `eventmodels`
- Purpose: Clients for external APIs (Polygon, Tradier, ORATS, etc.)
- Location: `src/go/eventservices/` (35 files)
- Contains: `PolygonOptionsClient` (options data), `PolygonTickDataMachine` (stock ticks), `PolygonCache` (3-bucket thread-safe cache), Tradier order/quote fetching, market calendar
- Depends on: External HTTP APIs, `eventmodels`
- Purpose: Domain types shared across all Go packages
- Location: `src/go/eventmodels/` (100+ files)
- Contains: `StockSymbol`, `OptionSymbol`, `OptionContractV3`, `Candle`, `FIFOQueue`, `GlobalResponseDispatcher`, event name constants, request/response DTOs
- Used by: All Go packages
- Purpose: Implement trading strategies that drive the backtester loop
- Location: `src/clients/python/`
- Contains: `BacktesterPlaygroundClient` (Twirp RPC wrapper), `TradingEngine` (main engine), strategy implementations, visualization tools
- Depends on: Go server via Twirp RPC on port 5051
## Data Flow
- `DatabaseService` holds in-memory maps of all active playgrounds, orders, trades, and live accounts
- Playgrounds are loaded from PostgreSQL on server startup via `loadData()`
- `Playground` struct maintains its own position cache, open orders cache, and FIFO queues for new candles/trades
- Thread safety via `sync.Mutex` on `DatabaseService`, individual playground mutexes, and per-bucket mutexes on `PolygonCache`
## Key Abstractions
- Purpose: A trading session (backtesting, live, or reconciliation)
- Examples: `src/go/backtester-api/models/playground.go`
- Pattern: UUID primary key, GORM model with embedded `Meta`, contains `BacktesterAccount`, `Clock`, `CandleMasterRepository`, order queues
- Environments: `simulator` (backtesting), `live` (real broker), `reconcile` (cross-checking)
- Purpose: Represent orders and their fills
- Examples: `src/go/backtester-api/models/order_record.go`, `src/go/backtester-api/models/trade_record.go`
- Pattern: GORM models with M2M relationships (Closes, ClosedBy, Reconciles); status lifecycle (pending -> filled/rejected/canceled)
- Purpose: Time-series container for OHLCV candle data per symbol and timeframe
- Examples: `src/go/backtester-api/models/candle_repository.go`, `src/go/backtester-api/models/candle_master_repository.go`
- Pattern: Holds candle data fetched from Polygon; supports indicator computation; master repo manages multiple symbol/period repos
- Purpose: Abstract broker operations for live and mock trading
- Examples: `src/go/backtester-api/models/broker_interface.go`
- Pattern: Interface with implementations `TradierBroker` (live), `MockBroker` (testing/simulation)
- Implementations: `src/go/backtester-api/services/tradier_broker.go`, `src/go/backtester-api/models/mock_broker.go`
- Purpose: Generic thread-safe queue for async event processing
- Examples: `src/go/eventmodels/fifo_queue.go`
- Pattern: Used for order updates, new candles, new trades between goroutines
## Entry Points
- Location: `cmd/main.go` (465 lines)
- Triggers: `task app:dev` or `go run ./cmd/main.go`
- Responsibilities:
- Location: `cmd/run-dev.sh`
- Sets `GO_ENV=development`, resolves `OPTIONS_CONFIG_PATH` for worktree support
- Runs `go run ./main.go`
- Location: `src/clients/python/trading_engine.py`
- Triggers: `task sim` or `task metrics:simulation`
- Responsibilities: Creates playground, runs tick loop with configurable strategies, collects metrics
## Error Handling
- Twirp RPC: Errors returned from `Server` methods are automatically serialized as Twirp error responses
- Panic recovery middleware wraps the Twirp handler (`src/go/backtester-api/rpc/twirp.go`)
- REST API: `eventpubsub.PublishRequestError()` sends errors through the event bus to the `GlobalDispatchWorker`, which routes to the waiting HTTP handler
- `eventmodels.WebError` wraps errors with HTTP status codes for REST responses
- Database operations use GORM error handling with `fmt.Errorf` wrapping
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
