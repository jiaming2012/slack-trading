# Coding Conventions

**Analysis Date:** 2026-03-25

## Naming Patterns

**Files (Go):**
- Use `snake_case.go` for all Go source files: `playground.go`, `order_record.go`, `database_service.go`
- Test files: `*_test.go` co-located with source: `playground_test.go`
- Mock files: `mock_*.go` co-located with models: `mock_database.go`, `mock_broker.go`, `mock_options_broker.go`
- Interface files: `*_interface.go`: `database_service_interface.go`, `broker_interface.go`, `live_account_interface.go`
- Error files: `error.go` or `errors.go` per package

**Files (Python):**
- Use `snake_case.py` for source: `trading_engine.py`, `risk_management.py`
- Test files: `test_*.py` prefix: `test_kelly_sizing.py`, `test_partial_exit_manager.py`

**Packages (Go):**
- Lowercase, single-word where possible: `models`, `data`, `utils`, `router`
- Multi-word with hyphens in directory names: `backtester-api` (imported as `backtester_router` when aliased)
- Event-prefixed packages: `eventmodels`, `eventservices`, `eventconsumers`, `eventproducers`, `eventpubsub`

**Functions (Go):**
- PascalCase for exported: `NewPlayground()`, `PlaceOrder()`, `GetOrder()`
- camelCase for unexported: `commitTradableOrderToOrderQueue()`, `updateOpenOrdersCache()`
- Constructor pattern: `New<Type>()` returns `*Type` and `error`: `NewPlayground(...)`, `NewOrderRecord(...)`, `NewCandleRepository(...)`
- `Get` prefix for simple accessors: `GetSymbol()`, `GetTicker()`, `GetStatus()`
- `Fetch` prefix for operations that hit external resources (DB, API): `FetchEquity()`, `FetchPositions()`, `FetchCandles()`

**Variables (Go):**
- camelCase for local and unexported fields: `startTime`, `stockSymbol`, `playgroundClient`
- PascalCase for exported struct fields: `Balance`, `Orders`, `ClientID`

**Types (Go):**
- PascalCase structs: `Playground`, `OrderRecord`, `TradeRecord`, `DatabaseService`
- Interfaces prefixed with `I`: `IDatabaseService`, `IBroker`, `ILiveAccount`, `IOptionsBroker`, `IReconcilePlayground`
  - Exception: `Instrument` interface (no prefix)
- String-based enums as typed strings: `type StockSymbol string`, `type OptionSymbol string`
- Constants as PascalCase vars: `PlaygroundEnvironmentSimulator`, `PlaygroundEnvironmentLive`

**Functions (Python):**
- `snake_case` for functions and methods: `kelly_fraction()`, `compute_exit_plan()`, `check_exits()`
- Classes in PascalCase: `ExitPlan`, `ExitTier`, `BacktesterPlaygroundClient`

**Error Variables (Go):**
- Mixed convention -- two styles coexist:
  - Newer: `Err` prefix: `ErrDbOrderIsNotOpenOrPending`, `ErrDuplicateCloseTrade`, `ErrOptionContractIsExpired`
  - Older: `Err` suffix: `BalanceOutOfRangeErr`, `MaxLossPercentErr`, `PriceLevelsNotSortedErr`
- **Use the `Err` prefix style for new code** (matches Go convention and newer codebase patterns)

## Code Style

**Formatting:**
- Go: Standard `gofmt` (no custom formatter config detected)
- Python: No `.flake8`, `pyproject.toml`, or formatter config detected
- No `.editorconfig`, `.prettierrc`, or `.golangci.yml` present

**Linting:**
- No linter configuration files detected
- Rely on Go compiler warnings and `go vet` implicitly

## Import Organization

**Go imports use three groups separated by blank lines:**
1. Standard library imports
2. Third-party imports (alphabetically)
3. Internal project imports (`github.com/jiaming2012/slack-trading/...`)

**Example from `playground.go`:**
```go
import (
    "context"
    "errors"
    "fmt"
    "math"
    "sync"
    "time"

    "github.com/google/uuid"
    log "github.com/sirupsen/logrus"
    "gorm.io/gorm"

    "github.com/jiaming2012/slack-trading/src/go/eventmodels"
    "github.com/jiaming2012/slack-trading/src/go/models"
    "github.com/jiaming2012/slack-trading/src/go/utils"
)
```

**Import aliases used:**
- `log "github.com/sirupsen/logrus"` -- universal across the codebase
- `pb "github.com/jiaming2012/slack-trading/src/go/playground"` -- protobuf stubs
- `backtester_router "github.com/jiaming2012/slack-trading/src/go/backtester-api/router"` -- disambiguating

**Module path:** `github.com/jiaming2012/slack-trading`
- All internal imports: `github.com/jiaming2012/slack-trading/src/go/<package>`

**Path Aliases:**
- None (no `tsconfig.json` or Go module path aliasing beyond the module path)

## Error Handling

**Go patterns:**
- Wrap errors with context using `fmt.Errorf("...: %w", err)`:
  ```go
  return fmt.Errorf("failed to get order: %w: %w", err, ErrOptionAssignmentOrderNotFound)
  return fmt.Errorf("populateRepo: error getting candle repository: %w", err)
  ```
- Sentinel errors defined as package-level vars in `error.go` / `errors.go`:
  ```go
  var ErrOrderAlreadyFilled = fmt.Errorf("order is already filled")
  var ErrCurrentPriceNotSet = fmt.Errorf("current price is not set")
  ```
- Check errors with `errors.Is()` for sentinel errors
- Functions return `(result, error)` tuples consistently
- Validation errors returned from constructors (e.g., `NewOrderRecord` validates all params)

**Error DTO for HTTP responses:**
```go
type ErrorDTO struct {
    Msg string `json:"msg"`
}
```

## Logging

**Framework:** `github.com/sirupsen/logrus` (aliased as `log` everywhere)

**Patterns:**
- Always import as: `log "github.com/sirupsen/logrus"`
- Use level-specific methods: `log.Infof()`, `log.Debugf()`, `log.Errorf()`, `log.Warnf()`, `log.Fatalf()`
- Include context in log messages with format strings:
  ```go
  log.Infof("%v: PlaceOrder %d:end", req.ClientRequestId, order.ID)
  log.Debugf("Mock order %d filled, with delay", req.OrderId)
  log.Infof("CreatePlayground: id=%s env=%s balance=%.2f start=%s stop=%s", ...)
  ```
- Structured logging with `WithFields` used in the GORM logger adapter (`src/go/logger/logger.go`):
  ```go
  l.logger.WithContext(ctx).WithFields(logrus.Fields{
      "elapsed": elapsed,
      "rows":    rows,
      "sql":     sql,
  }).Warn("SLOW SQL >= 200ms")
  ```
- OpenTelemetry integration via `otellogrus` hook (configured in `cmd/main.go`)

**When to log:**
- `Info`: Significant state changes (order placed, playground created, loading operations)
- `Debug`: Internal flow tracing (cache hits, mock operations, request IDs)
- `Error`: Failed operations that don't crash the program
- `Fatal`: Startup failures only (missing config, DB connection failure)
- `Warn`: Degraded operations (slow SQL, skipping duplicates)

## Configuration

**Environment variables:**
- Loaded via `github.com/joho/godotenv` from `.env` files
- Accessed through `utils.GetEnv("VAR_NAME")` helper (returns value + error)
- Key vars: `PROJECT_DIR`, `GO_ENV`, `POLYGON_API_KEY`, `POSTGRES_HOST`, `LOG_LEVEL`
- `.env` files are gitignored

**YAML configuration:**
- Options config: `src/go/options-config.yaml` loaded via `gopkg.in/yaml.v3`
- Conda environment: `grodt.yml`

**Struct tags:**
- GORM: `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`, `gorm:"column:balance;type:numeric;not null"`
- JSON: `json:"msg"`, `json:"-"` for excluded fields
- Combined: fields can have both `json` and `gorm` tags

## Comments

**When to comment:**
- Comment blocks before complex test functions explaining the scenario being tested:
  ```go
  // This test verifies that when both an OptionAssigned and OptionExpired event
  // fire for the same order in a single tick, the order is only closed once.
  ```
- Inline comments for non-obvious logic or workarounds
- TODO comments for incomplete implementations: `// need to get the external order id. Maybe place it on the live order?`

**JSDoc/TSDoc:**
- Not applicable (no TypeScript)

**Go doc comments:**
- Minimal usage. Some types and exported functions lack doc comments.
- When present, follow standard Go convention: `// RouterSetupItem defines a single route handler configuration.`

**Python docstrings:**
- Module-level docstrings in test files explaining purpose and usage:
  ```python
  """
  Unit tests for Kelly criterion and position sizing.
  Run::
      cd src/clients/python
      /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_kelly_sizing.py -v
  """
  ```

## Function Design

**Go constructors:**
- Always return `(*Type, error)` even when error may be nil
- Validate all inputs in the constructor
- Many parameters (sometimes 15+) passed positionally -- no options pattern used
- Example: `NewOrderRecord(id, externalOrderID, clientRequestID, playgroundID, class, accountType, timestamp, symbol, side, quantity, orderType, duration, requestedPrice, ...)`

**Interfaces:**
- Defined in separate `*_interface.go` files in `src/go/backtester-api/models/`
- Used for dependency injection: `IDatabaseService`, `IBroker`, `IOptionsBroker`
- Mock implementations live alongside production code in the same package

## Module Design

**Exports:** Standard Go visibility rules (PascalCase = exported)

**Barrel files:** Not used. Each package exposes types directly.

**Package organization:**
- Domain types in `eventmodels/` (shared across packages)
- Service logic in `eventservices/`
- Consumer workers in `eventconsumers/`
- HTTP handler producers in `eventproducers/` with sub-packages per API domain
- Backtester core in `backtester-api/models/`, `backtester-api/services/`, `backtester-api/router/`

## Git Workflow

**Branch naming:**
- Feature branches: `claude/<descriptive-name>` (e.g., `claude/nifty-diffie`)
- Main branches: `main`, `dev`

**Commit message style:**
- Short imperative messages, often single-quoted: `'update readme'`, `'update for running in live mode'`
- No conventional commits prefix (no `feat:`, `fix:`, etc.)

---

*Convention analysis: 2026-03-25*
