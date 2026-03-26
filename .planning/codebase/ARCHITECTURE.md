# Architecture

**Analysis Date:** 2026-03-25

## Pattern Overview

**Overall:** Event-driven trading platform with a Go monolithic server backend and Python strategy clients, communicating via Twirp RPC (protobuf over HTTP).

**Key Characteristics:**
- Two-process architecture: Go server manages state, order execution, and market data; Python clients implement trading strategies
- In-process pub/sub event bus for decoupled communication between Go components
- Twirp RPC (port 5051) as the primary API surface for backtesting and live trading
- REST API (Gorilla mux, port 8080) for signals, data feeds, alerts, accounts, and Slack integration
- GORM/PostgreSQL for persistence with auto-migration on startup
- EventStoreDB for event sourcing of signals and account state

## Layers

**RPC/API Layer (Twirp + REST):**
- Purpose: Accept external requests from Python clients and HTTP consumers
- Location: `src/go/backtester-api/router/grpc.go` (Twirp handlers, 1266 lines), `src/go/backtester-api/rpc/twirp.go` (server setup), `src/go/backtester-api/router/handler.go` (REST handlers for `/playground`)
- Contains: `Server` struct implementing all `PlaygroundService` RPC methods; REST route setup
- Depends on: `data.DatabaseService`, `eventservices.PolygonOptionsClient`, `models.*`
- Used by: Python clients via Twirp, HTTP clients via REST

**Domain Model Layer:**
- Purpose: Core business entities and trading logic
- Location: `src/go/backtester-api/models/` (80+ files)
- Contains: `Playground` (2909 lines), `OrderRecord`, `TradeRecord`, `CandleRepository`, `BacktesterAccount`, `Clock`, `LiveAccount`, broker interfaces
- Depends on: `eventmodels` (shared types), `models` (legacy shared models)
- Used by: Router, DatabaseService, Services

**Database Service Layer:**
- Purpose: Persistence, caching, and orchestration of playgrounds and orders
- Location: `src/go/data/database_service.go` (1634 lines)
- Contains: `DatabaseService` struct with in-memory caches for playgrounds, orders, trades, live accounts; GORM queries
- Depends on: `gorm.DB`, `models.*`, `eventmodels.*`, `dbutils`
- Used by: RPC handlers, backtester router

**Services Layer:**
- Purpose: Broker integration, order queue processing, live account management
- Location: `src/go/backtester-api/services/`
- Contains: `TradierBroker` (live broker), `MockBroker`, order queue draining, live repository management
- Key files: `services/tradier_broker.go`, `services/order_queue.go`, `services/playground.go`, `services/accounts.go`
- Depends on: `models.*`, external Tradier API
- Used by: DatabaseService, Router

**Event Pub/Sub Layer:**
- Purpose: In-process decoupled communication between Go components
- Location: `src/go/eventpubsub/` (4 files)
- Contains: Thin wrapper around `EventBus` library; `Publish`, `Subscribe`, `Unsubscribe` functions
- Pattern: Global singleton `bus` initialized via `Init()`; async subscriptions
- Depends on: `github.com/asaskevich/EventBus`
- Used by: Event consumers, event producers, global dispatcher

**Event Consumers:**
- Purpose: Background workers that subscribe to pub/sub events and perform side effects
- Location: `src/go/eventconsumers/` (21 files)
- Contains: `SlackNotifierClient`, `TradierApiWorker`, `GlobalDispatchWorker`, `AccountWorkerClient`, ESDB consumers
- Pattern: Each consumer has a `Start(ctx)` method that subscribes to topics and runs a goroutine
- Depends on: `eventpubsub`, `eventmodels`, external services (Slack, Tradier, ESDB)

**Event Producers:**
- Purpose: REST API handlers and external event sources that publish to the event bus
- Location: `src/go/eventproducers/` (22 files across subdirectories)
- Contains: API route handlers (`tradeapi`, `accountapi`, `datafeedapi`, `alertapi`, `signalapi`, `optionsapi`, `strategyapi`), Slack command handler, ESDB producer
- Depends on: `eventpubsub`, `eventmodels`

**External Services:**
- Purpose: Clients for external APIs (Polygon, Tradier, ORATS, etc.)
- Location: `src/go/eventservices/` (35 files)
- Contains: `PolygonOptionsClient` (options data), `PolygonTickDataMachine` (stock ticks), `PolygonCache` (3-bucket thread-safe cache), Tradier order/quote fetching, market calendar
- Depends on: External HTTP APIs, `eventmodels`

**Shared Event Models:**
- Purpose: Domain types shared across all Go packages
- Location: `src/go/eventmodels/` (100+ files)
- Contains: `StockSymbol`, `OptionSymbol`, `OptionContractV3`, `Candle`, `FIFOQueue`, `GlobalResponseDispatcher`, event name constants, request/response DTOs
- Used by: All Go packages

**Python Strategy Clients:**
- Purpose: Implement trading strategies that drive the backtester loop
- Location: `src/clients/python/`
- Contains: `BacktesterPlaygroundClient` (Twirp RPC wrapper), `TradingEngine` (main engine), strategy implementations, visualization tools
- Depends on: Go server via Twirp RPC on port 5051

## Data Flow

**Backtester Loop (Python -> Go -> Python):**

1. Python calls `CreatePlayground` RPC with symbol, date range, balance, and candle repository config
2. Go server fetches historical data from Polygon API, builds `CandleRepository` objects, creates `Playground` in memory
3. Python enters tick loop: calls `NextTick` RPC with playground ID and time duration
4. Go server advances the `Clock`, returns new candles and any filled trades as `TickDelta`
5. Python strategy evaluates indicators on new candle data, decides on trades
6. Python calls `PlaceOrder` RPC with order details (symbol, side, quantity, price)
7. Go server validates the order, adds it to the playground, and simulates fill via tick matching in `Playground.simulateTick()`
8. On next `NextTick`, filled trades are returned in the `TickDelta` response
9. Loop continues until `isBacktestComplete` is true (clock reaches stop date)

**Live Trading Flow:**

1. Python calls `CreateLivePlayground` RPC with broker connection details
2. Go server creates playground linked to a `LiveAccount` and real `TradierBroker`
3. `TradierApiWorker` (event consumer) polls Tradier API for order status updates
4. Order updates are pushed to `liveOrdersUpdateQueue` (FIFO queue)
5. `handleLiveOrders` goroutine drains the queue via `services.DrainTradierOrderQueue`
6. Order fills are recorded as `TradeRecord` objects in the database

**REST API Request Flow:**

1. HTTP request arrives at Gorilla mux router
2. Route handler creates a request event with a UUID
3. Event is published to the in-process pub/sub bus
4. `GlobalResponseDispatcher` registers a result callback channel
5. Consumer processes the event, publishes result back to bus
6. Dispatcher routes result to the waiting HTTP handler via channel
7. Handler serializes response and returns to client

**State Management:**
- `DatabaseService` holds in-memory maps of all active playgrounds, orders, trades, and live accounts
- Playgrounds are loaded from PostgreSQL on server startup via `loadData()`
- `Playground` struct maintains its own position cache, open orders cache, and FIFO queues for new candles/trades
- Thread safety via `sync.Mutex` on `DatabaseService`, individual playground mutexes, and per-bucket mutexes on `PolygonCache`

## Key Abstractions

**Playground:**
- Purpose: A trading session (backtesting, live, or reconciliation)
- Examples: `src/go/backtester-api/models/playground.go`
- Pattern: UUID primary key, GORM model with embedded `Meta`, contains `BacktesterAccount`, `Clock`, `CandleMasterRepository`, order queues
- Environments: `simulator` (backtesting), `live` (real broker), `reconcile` (cross-checking)

**OrderRecord / TradeRecord:**
- Purpose: Represent orders and their fills
- Examples: `src/go/backtester-api/models/order_record.go`, `src/go/backtester-api/models/trade_record.go`
- Pattern: GORM models with M2M relationships (Closes, ClosedBy, Reconciles); status lifecycle (pending -> filled/rejected/canceled)

**CandleRepository:**
- Purpose: Time-series container for OHLCV candle data per symbol and timeframe
- Examples: `src/go/backtester-api/models/candle_repository.go`, `src/go/backtester-api/models/candle_master_repository.go`
- Pattern: Holds candle data fetched from Polygon; supports indicator computation; master repo manages multiple symbol/period repos

**IBroker Interface:**
- Purpose: Abstract broker operations for live and mock trading
- Examples: `src/go/backtester-api/models/broker_interface.go`
- Pattern: Interface with implementations `TradierBroker` (live), `MockBroker` (testing/simulation)
- Implementations: `src/go/backtester-api/services/tradier_broker.go`, `src/go/backtester-api/models/mock_broker.go`

**FIFOQueue:**
- Purpose: Generic thread-safe queue for async event processing
- Examples: `src/go/eventmodels/fifo_queue.go`
- Pattern: Used for order updates, new candles, new trades between goroutines

## Entry Points

**Server Main (`cmd/main.go`):**
- Location: `cmd/main.go` (465 lines)
- Triggers: `task app:dev` or `go run ./cmd/main.go`
- Responsibilities:
  1. Load environment variables via `utils.InitEnvironmentVariables()`
  2. Initialize pub/sub bus via `eventpubsub.Init()`
  3. Connect to PostgreSQL via `dbutils.InitPostgres()` (auto-migrates schema)
  4. Set up Google Sheets client
  5. Configure Gorilla mux router with REST API routes (`/trades`, `/accounts`, `/datafeeds`, `/alerts`, `/signals`, `/data`, `/version`, `/playground`, `/debug/pprof`)
  6. Initialize Tradier brokers (paper + margin accounts) and mock broker
  7. Set up backtester router (loads persisted playgrounds from DB)
  8. Start event consumers: Slack notifier, Slack command handler, global dispatcher, account worker, Tradier API worker
  9. Start HTTP server on `PORT` env var (default 8080)
  10. Start Twirp server on port 5051
  11. Wait for SIGTERM/SIGINT for graceful shutdown

**Dev Runner (`cmd/run-dev.sh`):**
- Location: `cmd/run-dev.sh`
- Sets `GO_ENV=development`, resolves `OPTIONS_CONFIG_PATH` for worktree support
- Runs `go run ./main.go`

**Python Trading Engine (`src/clients/python/trading_engine.py`):**
- Location: `src/clients/python/trading_engine.py`
- Triggers: `task sim` or `task metrics:simulation`
- Responsibilities: Creates playground, runs tick loop with configurable strategies, collects metrics

## Error Handling

**Strategy:** Errors are returned as Go `error` values up the call stack. Twirp translates these to RPC error responses. REST API uses a global dispatcher pattern for async error routing.

**Patterns:**
- Twirp RPC: Errors returned from `Server` methods are automatically serialized as Twirp error responses
- Panic recovery middleware wraps the Twirp handler (`src/go/backtester-api/rpc/twirp.go`)
- REST API: `eventpubsub.PublishRequestError()` sends errors through the event bus to the `GlobalDispatchWorker`, which routes to the waiting HTTP handler
- `eventmodels.WebError` wraps errors with HTTP status codes for REST responses
- Database operations use GORM error handling with `fmt.Errorf` wrapping

## Cross-Cutting Concerns

**Logging:** `github.com/sirupsen/logrus` (Go), `loguru` (Python). OpenTelemetry hooks added via `otellogrus`. Log level configurable via `LOG_LEVEL` env var.

**Validation:** Request validation happens in RPC handler methods (`src/go/backtester-api/router/grpc.go`). Playground environment validated via `playgroundEnvironment.Validate()`. Order validation in `Playground.PlaceOrder()`.

**Authentication:** No authentication layer on the server. Tradier API uses bearer tokens stored in environment variables. Google Sheets uses base64-encoded service account key.

**Caching:** `PolygonCache` (`src/go/eventservices/polygon_cache.go`) provides 3-bucket thread-safe in-memory cache with optional disk persistence for aggregate bars. `DatabaseService` maintains in-memory caches for playgrounds, orders, and trades. `RequestCache` in `Server` deduplicates concurrent `NextTick` calls.

**Profiling:** pprof endpoints registered at `/debug/pprof/*` for runtime profiling.

---

*Architecture analysis: 2026-03-25*
