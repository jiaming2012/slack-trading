# Project: slack-trading (grodt)

## Overview
Event-driven trading platform with a Go backend server and Python ML/strategy clients. Communicates via gRPC/Twirp. Deploys to a Vultr Kubernetes cluster.

## Go Module
`github.com/jiaming2012/slack-trading` — Go 1.22.4

## Repository Layout

```
cmd/
  main.go                  # Primary entrypoint (package main) — the trading server
src/
  go/                      # All Go packages
    backtester-api/        # Backtester service (models, router, rpc, services, playground)
    coinbase/              # Coinbase integration
    coingecko/             # CoinGecko integration
    data/                  # Database service layer
    dbutils/               # Postgres init helpers
    eventconsumers/        # Event-driven consumer workers (Slack, Tradier, ESDB, etc.)
    eventdto/              # Event DTOs
    eventmain/             # (removed — replaced by cmd/main.go)
    eventmodels/           # Core domain models and types
    eventproducers/        # Event producers (Slack, ESDB, API handlers)
    eventpubsub/           # In-process pub/sub system
    eventservices/         # Service layer (Polygon, Tradier, export, etc.)
    eventstore/            # EventStoreDB abstractions
    handler/               # HTTP handler utilities
    indicators/            # Technical indicators
    logger/                # Logging setup
    models/                # Shared model types
    playground/            # Protobuf-generated Go stubs (playground.pb.go, playground.twirp.go)
    sheets/                # Google Sheets integration
    slack/                 # Slack message formatting
    strategy/              # Trading strategy logic
    testing/               # Test helpers
    utils/                 # Utility functions (env, conversion, streams, Tradier, Polygon, etc.)
    worker/                # Background worker infrastructure
    playground.proto       # Protobuf definition (generates Go + Python stubs)
    options-config.yaml    # Options trading config (dev)
    options-config-prod.yaml
  clients/
    python/                # Python trading client and strategies
      trading_engine.py              # Main trading engine
      trading_engine_optimizer.py    # Bayesian optimizer (scikit-optimize)
      trading_engine_types.py        # Shared types
      backtester_playground_client_grpc.py  # gRPC client to Go server
      playground_types.py            # Python playground types
      playground_metrics.py          # Metrics fetcher
      generate_signals.py            # Signal generation from tick data
      options_strategy_basic_v7.py   # Active options strategy
      simple_open_strategy_v4.py     # Open strategy
      simple_close_strategy.py       # Close strategy
      base_open_strategy.py          # Base open strategy
      base_open_strategy_v2.py       # Base open strategy v2
      simple_stack_open_strategy_v2.py
      simple_stack_close_strategy.py
      stack_close_strategy_psar.py
      plot_*.py                      # Visualization scripts
      renko.py                       # Renko chart utilities
      utils.py                       # Python utilities
      requirements.txt               # Python dependencies
      rpc/                           # Generated protobuf stubs
        playground_pb2.py
        playground_twirp.py
integration_testing/       # Go integration/e2e tests
deprecated/                # Archived code (see deprecated/README.md)
  go/algos/                # Old trendline algo (separate go.mod)
  go/src/                  # Old Go entry points and utilities
  go/cmd/                  # Old CLI tools (sandbox, telemetry, stats tools)
  python/stats/            # Old Python strategies (v1-v6) and distribution tools
  python/backtester/       # PPO/RL scripts
```

## Key Architecture

### Go Import Path Convention
All internal imports use: `github.com/jiaming2012/slack-trading/src/go/<package>`

### Communication
- **Go ↔ Python**: Twirp RPC (protobuf over HTTP) via `src/go/playground.proto`
- **Protobuf codegen**: `task gen:proto` generates both Go (`src/go/playground/`) and Python (`src/clients/python/rpc/`) stubs

### Event System
The server is event-driven with producers, consumers, and an in-process pub/sub (`eventpubsub`). Key event flows:
- Slack commands → eventproducers → eventpubsub → eventconsumers
- API requests → eventproducers → handlers
- ESDB (EventStoreDB) for persistence of event streams

### External Services
- **Tradier**: Options/stock quotes, order execution, positions, market calendar
- **Polygon.io**: Tick data, option chains
- **Slack**: Command interface and notifications
- **Google Sheets**: Data export
- **EventStoreDB**: Event stream persistence
- **PostgreSQL**: Relational data (orders, playgrounds, etc.)

## Build & Development

### Build
```bash
go build ./cmd/main.go          # Build the server
go build ./src/go/...            # Build all Go packages
docker build -t app -f Dockerfile .  # Docker build (uses ./cmd/main.go)
```

### Test
```bash
task test                        # Unit tests (src/go/backtester-api)
task test:e2e                    # Full e2e test suite
task test:integration            # Integration tests (src/go/eventservices)
go build ./integration_testing/... # Verify integration test compilation
```

### Task Runner
Uses [go-task](https://taskfile.dev/) — see `taskfile.yml`. Key tasks:
- `task test` — run unit tests
- `task app:build` / `task app:deploy` — Docker build and deploy
- `task gen:proto` — regenerate protobuf stubs
- `task python:install` / `task python:upgrade` — Python venv management
- `task metrics:simulation` — run trading simulation
- `task optimize` — run Bayesian optimization
- `task sim` — run a simulation

### Environment
- `PROJECTS_DIR` env var points to the parent of the repo (e.g., `/Users/jamal/projects`)
- Options config loaded from `${PROJECTS_DIR}/slack-trading/src/go/options-config.yaml`
- Python venv at `src/clients/python/env/`

## Worktree Notes
- This branch (`claude/nifty-diffie`) uses a git worktree at `.claude/worktrees/nifty-diffie`
- `${PROJECTS_DIR}/slack-trading` resolves to the **main** repo, not the worktree
- Task `dir:` paths using `${PROJECTS_DIR}/slack-trading/...` won't resolve inside the worktree — use relative paths instead
- The `test` task uses a relative `dir: src/go/backtester-api` for this reason

## Deploy
- Kubernetes on Vultr (namespace: default for app, `eventstoredb` for ESDB, `database` for Postgres)
- Docker images pushed to `ewr.vultrcr.com/grodt/`
- Flux CD for GitOps (`.clusters/production/flux-system/`)
- Sealed secrets for config (`sealedsecret.yaml`)
