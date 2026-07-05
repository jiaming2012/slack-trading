# External Integrations

**Analysis Date:** 2026-03-25

## APIs & External Services

**Market Data:**
- Polygon.io - Stock and options market data (candles, aggregate bars, option chains, daily ticker summaries)
  - SDK/Client: `github.com/polygon-io/client-go` v1.16.6 (Go), `polygon-api-client` 0.2.11 (Python)
  - Custom HTTP client forces HTTP/1.1 to avoid GOAWAY errors (`src/go/marketdata/polygon.go`)
  - Auth: `POLYGON_API_KEY` env var
  - Cache: `PolygonCache` in `src/go/marketdata/polygon_cache.go` - 3-bucket thread-safe in-memory cache (contracts, stock ticks, aggregate bars) with optional disk persistence
  - Tick data machine: `src/go/marketdata/polygon_tick_data_machine.go`
  - API base: `https://api.polygon.io/v2/aggs/ticker/...`
  - Known issue: Some symbol/date combos return 403 (e.g., MSFT 2024)

- Tradier - Broker API for live trading and market data
  - Custom HTTP client (no SDK, direct REST calls)
  - Implementation: `src/go/backtester/services/tradier_broker.go` (broker interface)
  - Worker: `src/go/workers/tradier_api_worker.go` (polling, order sync)
  - Auth: Multiple bearer tokens for sandbox vs. live:
    - `TRADIER_SANDBOX_TRADES_BEARER_TOKEN` / `TRADIER_LIVE_TRADES_BEARER_TOKEN`
    - `TRADIER_SANDBOX_NON_TRADES_BEARER_TOKEN` / `TRADIER_LIVE_NON_TRADES_BEARER_TOKEN`
  - Config env vars:
    - `TRADIER_STOCK_QUOTES_URL` - Stock quote endpoint
    - `TRADIER_MARKET_CALENDAR_URL` - Market calendar
    - `TRADIER_OPTION_CHAIN_URL` - Option chain data
    - `TRADIER_OPTION_EXPIRATIONS_URL` - Option expiration dates
    - `TRADIER_MARKET_TIMESALES_URL` - Intraday time & sales
    - `TRADIER_SANDBOX_TRADES_URL_TEMPLATE` / `TRADIER_LIVE_TRADES_URL_TEMPLATE` - Order placement
    - `TRADIER_SANDBOX_BALANCES_URL_TEMPLATE` / `TRADIER_LIVE_BALANCES_URL_TEMPLATE` - Account balances
    - `TRADIER_SANDBOX_POSITIONS_URL_TEMPLATE` / `TRADIER_LIVE_POSITIONS_URL_TEMPLATE` - Positions
    - `TRADIER_SANDBOX_ACCOUNT_ID` / `TRADIER_LIVE_ACCOUNT_ID` - Account identifiers
    - `TRADIER_SANDBOX_TRADES_ACCOUNT_ID` / `TRADIER_LIVE_TRADES_ACCOUNT_ID`
  - Account types: paper (sandbox), margin (live), pdt
  - Env var resolution: `src/go/backtester/models/live_account_variables.go`

- ORATS - Options analytics and historical data
  - Implementation: `src/go/marketdata/orats.go`
  - Data format: CSV (parsed with `gocsv`)
  - Models: `src/go/eventmodels/orats_option_data.go`

- CoinGecko - Cryptocurrency price data (BTC via Coinbase/gdax)
  - Implementation: `src/go/coingecko/service.go`
  - Used for: BTC price fetching

**Broker Operations:**
- Tradier Order Execution
  - Order placement: `TradierBroker.PlaceOrder()` in `src/go/backtester/services/tradier_broker.go`
  - Order monitoring: `TradierApiWorker` in `src/go/workers/tradier_api_worker.go`
  - Spread orders: `src/go/eventmodels/tradier_order_spread_dto.go`
  - Position tracking: `src/go/eventmodels/tradier_position.go`
  - Balance fetching: `TradierBroker.FetchBalances()` / `FetchEquity()`
  - Mock broker for testing: `src/go/backtester/models/mock_broker.go`

## Data Storage

**PostgreSQL:**
- Version: 13 (Docker image `postgres:13`)
- ORM: GORM v1.25.12 (`gorm.io/gorm`) with PostgreSQL driver (`gorm.io/driver/postgres`)
- Connection setup: `src/go/dbutils/postgres.go` (`InitPostgres`, `InitPostgresWithUrl`)
- Database name: `playground`
- Default credentials: user `grodt`, password `test747` (dev)
- Connection env vars: `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- Auto-migration on startup for all GORM models
- Core tables: playgrounds, order_records, trade_records, live_accounts, equity_plot_records
- Database service interface: `src/go/backtester/models/database_service_interface.go`
- Database service impl: `src/go/data/database_service.go`
- Docker Compose: `eventstoredb/docker-compose.yaml` (port 5432)
- K8s: `kubectl port-forward svc/postgres 5432:5432 -n database`

**EventStoreDB:**
- Version: 24.2.0 (Docker image `eventstore/eventstore:24.2.0-jammy`)
- Client: `github.com/EventStore/EventStore-Client-Go/v4` v4.1.0
- Connection: `EVENTSTOREDB_URL` env var
- Implementation: `src/go/eventstore/db.go` (event insertion)
- Consumer: `src/go/workers/esdb_consumer.go`, `esdb_consumer_stream.go`
- Producer: `src/go/api/esdb_producer.go`
- Docker Compose: `eventstoredb/docker-compose.yaml` (ports 1113 TCP, 2113 HTTP)
- Runs with projections enabled, insecure mode, Atom PUB over HTTP
- K8s: `kubectl port-forward svc/eventstoredb 2113:2113 -n eventstoredb`

**In-Memory:**
- `PolygonCache` - Thread-safe 3-bucket cache with optional disk persistence (`src/go/marketdata/polygon_cache.go`)
- `go-cache` - General purpose in-memory cache (`github.com/patrickmn/go-cache`)
- Request cache: `src/go/backtester/models/request_cache.go`
- Order cache: `src/go/backtester/models/order_cache.go`
- Positions cache: `src/go/backtester/models/positions_cache.go`

**File Storage:**
- Local filesystem only (cache persistence to disk for aggregate bars)
- CSV export: `src/go/marketdata/export_data.go`

## Authentication & Identity

**Service Account Auth:**
- Google Service Account - Base64-encoded JSON key for Sheets/Drive API
  - Env var: `GOOGLE_SECURITY_KEY_JSON_BASE64`
  - Implementation: `src/go/sheets/setup.go` - decodes base64, creates JWT config with Sheets + Drive scopes

**API Key Auth:**
- Polygon.io: API key passed as query parameter
- Tradier: Bearer tokens in Authorization header (separate tokens for trades vs. non-trades, sandbox vs. live)

**No User Auth:**
- The server exposes unauthenticated REST and Twirp endpoints
- Access control is assumed at the network level (internal cluster access)

## Monitoring & Observability

**Tracing:**
- OpenTelemetry with OTLP HTTP trace exporter
  - `go.opentelemetry.io/otel` v1.27.0
  - `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`
  - Tracer used in: `src/go/marketdata/fetch_option_chain_with_params.go` and other service files

**Metrics:**
- OpenTelemetry metrics with OTLP HTTP metric exporter
  - `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`
  - Runtime instrumentation: `go.opentelemetry.io/contrib/instrumentation/runtime`
  - HTTP instrumentation: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`

**Logging:**
- Logrus (`github.com/sirupsen/logrus`) - Go structured logging
- otellogrus hook bridges log levels (Panic, Fatal, Error, Warn, Info) to OpenTelemetry
- Loguru - Python structured logging
- `LOG_LEVEL` env var controls verbosity

**Profiling:**
- Go pprof endpoints registered in `cmd/main.go` via `net/http/pprof`

**Error Tracking:**
- No external error tracking service (Sentry, etc.)
- Errors flow through event pub/sub: `pubsub.PublishError()`, `PublishRequestError()`

## CI/CD & Deployment

**Hosting:**
- Vultr Kubernetes cluster
- Container registry: `ewr.vultrcr.com/grodt/`

**GitOps:**
- Flux CD
  - Git repository config: `.clusters/production/gitrepository.yaml`
  - Kustomization: `.clusters/production/kustomization.yaml`
  - Flux system: `.clusters/production/flux-system/`

**Kubernetes Resources (`.clusters/production/`):**
- `deployment.yaml` - Main app deployment
- `service.yaml` - App service
- `loadbalancer.yaml` - External load balancer
- `configmap.yaml` - App configuration
- `sealedsecret.yaml` - Encrypted secrets (Bitnami Sealed Secrets)
- `sealedsecret-flux-git-deploy.yaml` - Flux deploy key
- `secrets.yaml` - Secret references
- `postgres-deployment.yaml` / `postgres-service.yaml` / `postgres-pvc.yaml` - PostgreSQL in-cluster
- `eventstoredb-deployment.yaml` / `eventstoredb-service.yaml` / `eventstoredb-pvc.yaml` - EventStoreDB in-cluster
- `eventstoredb-config.yaml` / `postgres-configmap.yaml` / `postgres-secret.yaml` - Database configs

**Docker Build Chain:**
- `Dockerfile.base` (v1.0.24) - Ubuntu 20.04, Python 3.10.15, Conda, TA-Lib C library
- `Dockerfile.base2` (v1.0.6) - Go 1.22.4, go mod download, conda env update
- `Dockerfile` (v3.26.0) - Final app image, copies source, builds Go binary
- Registry images: `ewr.vultrcr.com/grodt/grodt-base-image`, `grodt-base-image-2`, `app`

**Deploy Process:**
- `task app:build` - Build Docker image tagged `latest`
- `task app:deploy` - Run `deploy-app.sh` with version tag
- `task app:update` - Rolling restart via `kubectl rollout restart`
- `task app:stop` / `task app:start` - Scale replicas to 0/1

## Event System (Internal)

**In-Process Pub/Sub:**
- Library: `github.com/asaskevich/EventBus`
- Setup: `src/go/pubsub/services.go` - global bus instance
- Pattern: Publish/Subscribe with event name topics
- Consumers subscribe at startup, producers publish events
- Event names defined in: `src/go/eventmodels/event_names.go`
- Request/response pattern via `PublishCompletedResponse()` with metadata correlation

**Event Consumers (`src/go/workers/`):**
- `tradier_api_worker.go` - Tradier order sync and candle fetching
- `slacknotifier.go` - Slack webhook notifications
- `googlesheets.go` - Google Sheets trade logging
- `option_alerts.go` - Option alert processing
- `esdb_consumer.go` / `esdb_consumer_stream.go` - EventStoreDB event consumption
- `process_signals.go` - Signal processing
- `tracker_consumer_v3.go` / `tracker_client_v3.go` - Price trackers
- `candle.go` - Candle processing
- `balance.go` / `account.go` - Account management
- `rsibot.go` - RSI-based bot
- `tradingbot.go` - General trading bot

**Event Producers (`src/go/api/`):**
- `tradeapi/handler.go` - Trade API endpoints
- `signalapi/handler.go` - Signal API endpoints
- `optionsapi/handler.go` - Options API endpoints
- `alertapi/handler.go` - Alert API endpoints
- `accountapi/handler.go` - Account API endpoints
- `datafeedapi/handler.go` - Data feed API endpoints
- `slack/handler.go` - Slack command handling
- `esdb_producer.go` - EventStoreDB event publishing
- `coinbase.go` - Coinbase integration
- `interactive_brokers.go` - Interactive Brokers integration

## Notification Systems

**Slack:**
- Webhook-based notifications
- Env var: `SLACK_OPTION_ALERTS_WEBHOOK_URL`
- Notifier: `src/go/workers/slacknotifier.go` (`SlackNotifierClient`)
- Sends: trade confirmations, open/close trade results, option alert signals
- Slack command handling: `src/go/api/slack/handler.go`
- Slack outgoing: `src/go/slack/outgoing.go`

**Google Sheets:**
- Trade and candle logging to spreadsheets
- Auth: Google Service Account (`GOOGLE_SECURITY_KEY_JSON_BASE64`)
- Client setup: `src/go/sheets/setup.go` (Sheets API v4 + Drive API v3)
- Operations: `src/go/sheets/trades.go` (append trades), `src/go/sheets/candles.go` (append candles)
- Consumer: `src/go/workers/googlesheets.go` (`GoogleSheetsClient`)

## Twirp RPC Interface

**Service:** `PlaygroundService` defined in `src/go/playground.proto`
- Server: `src/go/backtester/rpc/twirp.go` (port 5051)
- Handler: `src/go/backtester/router/grpc.go` (1248+ lines)
- Python client: `src/clients/python/backtester_playground_client_grpc.py`

**Key RPCs:**
- `CreatePlayground` / `CreateLivePlayground` - Initialize trading sessions
- `NextTick` - Advance simulation clock, return new candles/trades
- `PlaceOrder` / `PlaceMultiLegOrder` - Submit orders
- `GetAccount` / `GetAccountStats` - Account state and equity
- `GetPlaygrounds` - List all playgrounds
- `GetCandlesFromRepo` / `GetCandlesFromDataSource` - Candle data
- `GetOptionsLadder` - Options chain data
- `SavePlayground` / `DeletePlayground` - Persistence
- `MockFillOrder` - Test helper for simulating fills
- `GetReconciliationReport` / `GetEquityReport` - Reporting
- `GetDailyTickerSummaryFromPolygon` - Market data proxy

## Environment Configuration

**Required env vars (server startup in `cmd/main.go`):**
- `PROJECT_DIR` - Repository root path
- `GO_ENV` - Environment (`development` | `production`)
- `POLYGON_API_KEY` - Polygon.io API key
- `TRADIER_STOCK_QUOTES_URL` - Tradier quotes endpoint
- `TRADIER_MARKET_CALENDAR_URL` - Tradier market calendar
- `TRADIER_OPTION_CHAIN_URL` - Tradier option chain endpoint
- `TRADIER_OPTION_EXPIRATIONS_URL` - Tradier expirations endpoint
- `TRADIER_MARKET_TIMESALES_URL` - Tradier time & sales
- `SLACK_OPTION_ALERTS_WEBHOOK_URL` - Slack webhook
- `EVENTSTOREDB_URL` - EventStoreDB connection string
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- `OPTIONS_CONFIG_FILE` - Options config filename
- Tradier bearer tokens (sandbox or live depending on `GO_ENV`)

**Optional env vars:**
- `OPTIONS_CONFIG_PATH` - Full path override for options config
- `LOG_LEVEL` - Logging verbosity
- `GOOGLE_SECURITY_KEY_JSON_BASE64` - Google Sheets auth

**Secrets storage:**
- Development: `.env` file (loaded via `godotenv`)
- Production: Bitnami Sealed Secrets in `.clusters/production/sealedsecret.yaml`

---

*Integration audit: 2026-03-25*
