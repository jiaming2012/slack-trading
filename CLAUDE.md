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
