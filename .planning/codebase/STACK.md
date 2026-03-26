# Technology Stack

**Analysis Date:** 2026-03-25

## Languages

**Primary:**
- Go 1.22.4 - Server, backtester engine, event consumers/producers, all backend logic
- Python 3.10 - Strategy clients, backtesting orchestration, optimization, metrics visualization

**Secondary:**
- Protobuf (proto3) - RPC interface definition (`src/go/playground.proto`)
- YAML - Configuration (`src/go/options-config.yaml`, `grodt.yml`, `taskfile.yml`)
- Bash - Dev scripts (`cmd/run-dev.sh`, `deploy-app.sh`)

## Runtime

**Go Environment:**
- Go 1.22.4 (installed in Docker via `Dockerfile.base2`)
- Module: `github.com/jiaming2012/slack-trading`
- Module path: `go.mod` at repo root

**Python Environment:**
- Conda environment `grodt` defined in `grodt.yml`
- Python 3.10 (conda-forge channel)
- Alternative: pip virtualenv in `src/clients/python/env/` with `src/clients/python/requirements.txt`
- Conda binary: `/Users/jamal/miniconda3/envs/grodt/bin/python`

**Package Managers:**
- Go modules - `go.mod` / `go.sum` (lockfile present)
- Conda - `grodt.yml` for Python env management
- pip - `src/clients/python/requirements.txt` for virtualenv fallback

## Frameworks

**Core:**
- Twirp RPC v8.1.3 (`github.com/twitchtv/twirp`) - Primary RPC framework (JSON over HTTP, port 5051)
- Gorilla Mux v1.8.1 (`github.com/gorilla/mux`) - REST HTTP router (port 8080)
- GORM v1.25.12 (`gorm.io/gorm`) - ORM for PostgreSQL
- EventBus (`github.com/asaskevich/EventBus`) - In-process pub/sub event bus

**Testing:**
- Testify v1.9.0 (`github.com/stretchr/testify`) - Go test assertions and mocking
- TestContainers v0.35.0 (`github.com/testcontainers/testcontainers-go`) - Docker-based integration tests
- pytest (Python) - Strategy client tests

**Build/Dev:**
- Task (Taskfile v3) - Task runner (`taskfile.yml`)
- protoc + twirp plugin - Protobuf code generation for Go
- protoc + twirpy plugin - Protobuf code generation for Python
- Docker - Container builds (`Dockerfile`, `Dockerfile.base`, `Dockerfile.base2`)

**Observability:**
- OpenTelemetry v1.27.0 - Tracing and metrics (OTLP HTTP exporters)
- Logrus v1.9.3 (`github.com/sirupsen/logrus`) - Structured logging
- otellogrus (`github.com/uptrace/opentelemetry-go-extra/otellogrus`) - OTel-Logrus bridge
- net/http/pprof - Go profiling endpoints (imported in `cmd/main.go`)

## Key Dependencies

**Critical (Go):**
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

**Critical (Python):**
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

**Infrastructure (Go):**
- `github.com/jinzhu/copier` v0.4.0 - Deep copy for cache safety
- `github.com/patrickmn/go-cache` v2.1.0 - In-memory caching
- `github.com/go-resty/resty/v2` v2.15.2 - HTTP client
- `github.com/go-playground/validator/v10` v10.22.1 - Input validation
- `github.com/olekukonko/tablewriter` v0.0.5 - CLI table formatting

## Configuration

**Environment:**
- `.env` file loaded via `godotenv` (at `$PROJECT_DIR/.env`)
- `GO_ENV` controls environment: `development` | `production`
- `PROJECT_DIR` points to repo root
- `OPTIONS_CONFIG_FILE` / `OPTIONS_CONFIG_PATH` for options trading config
- `LOG_LEVEL` controls logrus verbosity

**Build:**
- `taskfile.yml` - All build/test/deploy tasks
- `Dockerfile` - Production image (FROM `grodt-base-image-2:3.9.0`)
- `Dockerfile.base` - Base image with Python 3.10, Conda, TA-Lib C library (Ubuntu 20.04)
- `Dockerfile.base2` - Second-layer base with Go 1.22.4, go mod download, conda env update
- `grodt.yml` - Conda environment specification

**Proto Generation:**
- Command: `task gen:proto`
- Input: `src/go/playground.proto`
- Go output: `src/go/playground/playground.pb.go`, `playground.twirp.go`
- Python output: `src/clients/python/rpc/playground_pb2.py`, `playground_twirp.py`

## Platform Requirements

**Development:**
- macOS (darwin) - Primary dev platform
- Go 1.22.4
- Conda (miniconda3) with `grodt` environment
- Docker for local databases (`task db:start`)
- PostgreSQL 13 (via Docker or port-forward to cluster)
- EventStoreDB 24.2.0 (via Docker or port-forward to cluster)
- protoc compiler with twirp and twirpy plugins

**Production:**
- Kubernetes on Vultr
- Container registry: `ewr.vultrcr.com/grodt/`
- Flux CD for GitOps deployment
- Sealed Secrets for secret management (`.clusters/production/sealedsecret.yaml`)
- Multi-layer Docker images: `grodt-base-image` -> `grodt-base-image-2` -> app image

**Ports:**
- 8080 - REST API (Gorilla Mux)
- 5051 - Twirp RPC server
- 5432 - PostgreSQL
- 2113 - EventStoreDB HTTP
- 1113 - EventStoreDB TCP

---

*Stack analysis: 2026-03-25*
