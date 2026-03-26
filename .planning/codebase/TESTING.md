# Testing Patterns

**Analysis Date:** 2026-03-25

## Test Framework

**Go Runner:**
- Standard `testing` package (Go 1.22.4)
- No test config file (uses `go test` defaults)

**Assertion Library:**
- `github.com/stretchr/testify v1.9.0`
- Primarily uses `require` (fail-fast) over `assert`
- Import pattern: `"github.com/stretchr/testify/require"`

**Python Runner:**
- `pytest` for unit tests (`test_kelly_sizing.py`, `test_partial_exit_manager.py`)
- `unittest` for integration/regression tests (`test_demo_covered_call.py`)

**Run Commands:**
```bash
task test                    # Unit tests (backtester-api only)
task test:e2e                # All E2E tests (Docker-based, testcontainers)
task test:integration        # Integration tests (eventservices)
```

**Underlying commands:**
```bash
# Unit tests
PROJECT_DIR={{.ROOT_DIR}} go test -count=1 ./...    # in src/go/backtester-api

# E2E (each test individually)
go test -timeout 60s -count=1 -run ^TestName$ github.com/jiaming2012/slack-trading/integration_testing

# Integration
go test ./...    # in src/go/eventservices/integration_tests

# Python unit tests
/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_kelly_sizing.py -v
```

## Test File Organization

**Location:** Co-located with source (same package)

**Naming (Go):**
- `*_test.go` suffix in same directory as source code
- Test function names: `TestFeatureName(t *testing.T)` with subtests via `t.Run("description", ...)`

**Naming (Python):**
- `test_*.py` prefix in `src/clients/python/`

**Structure:**
```
src/go/backtester-api/
  models/
    playground.go
    playground_test.go          # Unit tests for playground
    mock_database.go            # Mock for IDatabaseService
    mock_broker.go              # Mock for IBroker
    mock_options_broker.go      # Mock for IOptionsBroker
    mock_live_account.go        # Mock for ILiveAccount
    mock_reconcile_playground.go
    errors.go                   # Sentinel errors
  services/
    order_queue.go
    order_queue_test.go         # 1166 lines - largest test file
  mock/
    mock_backtester_data_feed.go

src/go/eventmodels/
  account_test.go
  price_level_test.go
  utils_test.go
  error.go                      # Shared sentinel errors

src/go/eventservices/
  polygon_cache_test.go
  account_test.go
  tradier_test.go               # Empty/stub
  integration_tests/
    polygon_client_test.go      # Live API test (intentionally fails)

integration_testing/            # E2E tests with testcontainers
  setup.go                      # Docker container setup
  utils.go                      # Shared test helpers
  postgres_test.go
  live_account_*_test.go        # 13 live account scenario tests
  simulate_options_test.go
  tradier_place_spread_test.go

src/clients/python/
  test_kelly_sizing.py          # Pure unit tests (pytest)
  test_partial_exit_manager.py  # Pure unit tests (pytest)
  test_demo_covered_call.py     # Integration test (requires running server)
  test_credit_spread_strategy.py
  test_deviation_levels.py
  test_mean_reversion_*.py
  test_pdf_*.py
  test_return_models.py
```

## Test Structure

**Go suite organization pattern:**
```go
func TestFeatureName(t *testing.T) {
    // Shared setup (fixtures, variables)
    symbol := eventmodels.StockSymbol("AAPL")
    startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

    t.Run("descriptive scenario name", func(t *testing.T) {
        // Arrange
        order, err := NewOrderRecord(1, nil, nil, uuid.Nil, ...)
        require.NoError(t, err)

        // Act
        trade := NewTradeRecord(order, now, 10, 1)
        _, err = order.Fill(trade)

        // Assert
        require.NoError(t, err)
        require.Equal(t, OrderRecordStatusFilled, order.GetStatus())
    })

    t.Run("another scenario", func(t *testing.T) {
        // ...
    })
}
```

**Python test organization (pytest):**
```python
class TestFeatureName:
    def test_specific_case(self):
        result = function_under_test(args)
        assert result == pytest.approx(expected, abs=1e-6)

    def test_edge_case(self):
        assert function_under_test(edge_input) == 0.0
```

**Python test organization (unittest):**
```python
class TestDemoCoveredCall(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        """Run expensive setup once."""
        # Run external process, capture output

    def test_final_balance(self):
        self.assertAlmostEqual(actual, expected, delta=TOLERANCE)
```

**Patterns:**
- Setup: Shared variables declared at top of parent `Test*` function, reused across subtests
- Teardown: Not commonly used; testcontainers handle cleanup via `testcontainers.CleanupContainer(t, container)`
- Assertion: Almost exclusively `require.*` (fail-fast), not `assert.*`

## Mocking

**Framework:** Hand-rolled mocks (no mockgen, gomock, or code generation)

**Mock implementations live in production source directories:**
- `src/go/backtester-api/models/mock_database.go` -- implements `IDatabaseService`
- `src/go/backtester-api/models/mock_broker.go` -- implements `IBroker`
- `src/go/backtester-api/models/mock_options_broker.go` -- implements `IOptionsBroker`
- `src/go/backtester-api/models/mock_live_account.go` -- implements `ILiveAccount`
- `src/go/backtester-api/models/mock_reconcile_playground.go` -- implements `IReconcilePlayground`
- `src/go/backtester-api/models/mock_live_account_source.go` -- implements `ILiveAccountSource`
- `src/go/backtester-api/mock/mock_backtester_data_feed.go` -- implements data feed interface

**Mock pattern (Go):**
```go
type MockDatabase struct {
    orderRecords         map[uuid.UUID][]*OrderRecord
    playgrounds          map[uuid.UUID]*Playground
    reconcilePlaygrounds map[CreateAccountRequestSource]IReconcilePlayground
    liveAccounts         map[CreateAccountRequestSource]ILiveAccount
    orderNounce          uint
    tradeNounce          uint
}

// Implements IDatabaseService methods with in-memory storage
func (m *MockDatabase) SavePlaygroundSession(playground *Playground) error {
    // store in maps
}
```

**Constructor pattern for mocks:**
```go
mockDB := NewMockDatabase()
err = mockDB.SavePlaygroundSession(playground)
```

**What to mock:**
- Database service (`IDatabaseService`) -- always mocked in unit tests
- Broker (`IBroker`) -- mocked for order filling simulation
- Options broker (`IOptionsBroker`) -- mocked for option price data
- Live account source -- mocked for account configuration

**What NOT to mock:**
- Domain models (`Playground`, `OrderRecord`, `CandleRepository`) -- constructed directly
- Clock -- constructed with `NewClock(startTime, endTime, calendar)`
- Value objects (`StockSymbol`, `OptionSymbol`) -- used directly

## Fixtures and Factories

**Test data (Go):**
```go
// Inline candle data construction
candles := []*eventmodels.PolygonAggregateBarV2{
    {Timestamp: startTime, Close: 210},
    {Timestamp: t1, Close: 220},
}

// Repository factory from candles
repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0,
    eventmodels.CandleRepositorySource{Type: "test"})

// Playground factory
playground, err := NewPlayground(nil, nil, nil, balance, balance, clock, nil,
    PlaygroundEnvironmentSimulator, startTime, []string{}, nil, repo1, repo2)
```

**Test data (Python):**
```python
REFERENCE = {
    "symbol": "AAPL",
    "start": "2025-01-01",
    "end": "2025-03-01",
    "balance": 200_000,
    "final_balance": 198_916.41,
}
TOLERANCE_DOLLARS = 0.02
```

**Calendar mock helper:**
- `MockFetchCalendarMap()` in `src/go/backtester-api/models/clock_test.go` -- returns hardcoded JSON calendar data

**Location:**
- No separate fixtures directory; all test data constructed inline
- Mock calendar data as JSON strings in test files

## Coverage

**Requirements:** None enforced. No coverage thresholds configured.

**View Coverage:**
```bash
cd src/go/backtester-api && go test -cover ./...
```

## Test Types

**Unit Tests:**
- Scope: Individual model/service methods (order filling, position tracking, cache operations, clock advancement)
- Location: `src/go/backtester-api/models/*_test.go`, `src/go/backtester-api/services/*_test.go`
- Total: ~7,846 lines across backtester-api unit tests
- Run: `task test`
- Dependencies: Only in-memory mocks, no external services

**Integration Tests (Go - eventservices):**
- Scope: External API calls (Polygon.io)
- Location: `src/go/eventservices/integration_tests/polygon_client_test.go`
- Run: `task test:integration`
- Note: Contains intentionally-failing stub test (`require.Fail(t, "finish the test")`)
- Requires: `PROJECT_DIR` and `.env` with API keys

**E2E Tests (Docker-based):**
- Scope: Full server lifecycle -- create playground, place orders, verify fills, check reconciliation
- Location: `integration_testing/`
- Total: ~2,628 lines across 16 test files
- Run: `task test:e2e` (runs all), or individual: `task test:e2e:live-account-filled`
- Infrastructure: `testcontainers-go v0.35.0`
  - Spins up Docker containers: app server, Postgres, EventStoreDB
  - Uses shared Docker network
  - Waits for `"Main: init complete"` log message
  - Timeout per test: 60-120 seconds
- Setup helper: `integration_testing/setup.go`
  - `setupDatabases(t, ctx, goEnv)` -- creates Postgres + EventStoreDB containers
  - `createPlaygroundServerAndClient(ctx, t, projectDir, networkName)` -- creates app container, returns Twirp client
- Utility helper: `integration_testing/utils.go`
  - `waitUntilOrderStatus()` -- polls order status with 1s interval, 60s timeout

**Python Unit Tests:**
- Scope: Pure functions (Kelly criterion, partial exit logic, deviation levels)
- Location: `src/clients/python/test_*.py`
- Run: `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest test_*.py -v`
- No external dependencies required

**Python Integration Tests:**
- Scope: Full backtest workflow validation
- Location: `src/clients/python/test_demo_covered_call.py`
- Run: Requires live Go server on `http://127.0.0.1:5051`
- Validates deterministic output against reference values

**Python Integration Tests (legacy):**
- Location: `src/go/testing/backtester_api_simulation_*_integration_test.py`
- Script-style (not pytest/unittest), require live server

## Common Patterns

**Async testing (Go):**
- Not heavily used; tests are synchronous
- E2E tests use polling: `waitUntilOrderStatus()` with `time.Ticker` + `time.After` timeout

**Error testing (Go):**
```go
t.Run("Filled - invalid price", func(t *testing.T) {
    order, err := NewOrderRecord(1, nil, nil, uuid.Nil, ...)
    require.NoError(t, err)

    trade := NewTradeRecord(order, now, 10, 0)  // invalid price
    _, err = order.Fill(trade)
    require.Error(t, err)
})
```

**Subtest naming convention:**
- Descriptive phrases: `"assign option early - fails when no option position"`
- Status-oriented: `"New"`, `"PartiallyFilled"`, `"Filled"`
- Error scenarios: `"Filled - invalid price"`, `"Filled - quantity exceeds order quantity"`

**Environment setup in tests:**
```go
// Some tests require PROJECT_DIR and env initialization
projectDir, err := utils.GetEnv("PROJECT_DIR")
require.NoError(t, err)

err = utils.InitEnvironmentVariables(projectDir, "test")
require.NoError(t, err)
```

**Playground test lifecycle:**
1. Create candle repositories with test data
2. Create mock database, broker, options broker
3. Construct playground with mocks
4. Place orders
5. Call `playground.Tick()` to advance time and fill orders
6. Assert on positions, balances, order status

**E2E test lifecycle:**
1. `setupDatabases(t, ctx, goEnv)` -- spin up Postgres + EventStoreDB
2. `createPlaygroundServerAndClient()` -- spin up app container
3. Create playground via RPC
4. Place orders via RPC
5. Advance ticks or wait for order status
6. Verify account state via `GetAccount` RPC
7. Containers auto-cleanup via `testcontainers.CleanupContainer(t, ...)`

## Test Gaps and Considerations

**Well-tested areas:**
- Order lifecycle (place, fill, partial fill, cancel, reject) -- extensive unit + E2E
- Position tracking and P&L calculation
- Cache operations (PolygonCache)
- Clock/calendar advancement
- Live account scenarios (13 E2E tests)

**Areas with minimal/no test coverage:**
- `src/go/eventservices/tradier_test.go` -- empty file (1 line: package declaration)
- `src/go/eventconsumers/` -- only 2 test files
- `src/go/eventproducers/` -- no test files detected
- `src/go/data/database_service.go` -- no unit tests (tested indirectly via E2E)
- `src/go/backtester-api/router/grpc.go` -- no unit tests (1248 lines, tested via E2E)
- REST API handlers in `eventproducers/` sub-packages -- no tests

**Intentionally-failing test:**
- `src/go/eventservices/integration_tests/polygon_client_test.go` ends with `require.Fail(t, "finish the test")` -- stub for future completion

---

*Testing analysis: 2026-03-25*
