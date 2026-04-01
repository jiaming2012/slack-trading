# Phase 3: Go Telemetry Instrumentation - Research

**Researched:** 2026-03-26
**Domain:** OpenTelemetry metrics + structured logging in Go server
**Confidence:** HIGH

## Summary

Phase 3 instruments the Go server with structured logs and OTel metrics for three domains: order lifecycle (live/reconcile playgrounds only), market data flow counters, and server heartbeat. The OTel SDK foundation from Phase 2 is already in place -- `utils.SetupOTelSDK` initializes both TracerProvider and MeterProvider with OTLP HTTP exporters. The MeterProvider is globally accessible via `otel.GetMeterProvider()`.

The codebase uses logrus with logfmt formatting (Phase 2). All structured logs should use `log.WithFields(log.Fields{...})` with snake_case field names, consistent with the existing pattern in `grpc.go` PlaceOrder/NextTick handlers. OTel metrics should use `otel.GetMeterProvider().Meter("grodt")` to obtain a meter, then register Int64Counter, Float64Gauge, or Int64UpDownCounter instruments.

**Primary recommendation:** Use the OTel metrics API directly (already in go.mod at v1.27.0) rather than Prometheus client library. The MeterProvider exports to OTLP, which the Collector routes to Prometheus -- no need for a separate Prometheus client dependency. Structured logs go through logrus with otellogrus hook (already configured in cmd/main.go).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Order placed/filled/rejected events emit structured logs with playground_id, order_id, symbol, side, quantity, order_type, environment="live", and trace_id.
- D-02: Only live playgrounds (Meta.Environment == "live") emit order telemetry. Simulator playgrounds are excluded. Reconcile playgrounds ARE included (they place real trades to the broker).
- D-03: Order rejection logs include the rejection reason as a structured field.
- D-04: Go server heartbeat emits every 30 seconds via background goroutine with ticker.
- D-05: Heartbeat is a metric gauge (for Grafana) + structured log (for forensics).
- D-06: Heartbeat stats: active playground count, last tick time, open order count, uptime.
- D-07: Heartbeat reports live playground stats only (consistent with order telemetry filter).
- D-08: Heartbeat metrics must be segmented by playground type and account type. Dimensions: environment (live, reconcile, simulator), account type (paper, margin). Live and reconcile get granular per-playground metrics; simulator gets aggregate count only.
- D-09: Keep a counter of candles successfully processed per strategy/playground (not individual candle logs).
- D-10: Keep a counter of signals generated and processed.
- D-11: Signal decision logging (why trade was/wasn't placed after signal) is Phase 4 (Python client) -- NOT this phase.
- D-12: Data gaps (missing candles, stale data) emit warning-level logs.

### Claude's Discretion
- Metric label/tag taxonomy for playground environment + account type segmentation
- Where exactly to hook into order lifecycle (grpc.go PlaceOrder handler vs models vs services)
- Heartbeat implementation details (background goroutine, ticker interval, gauge registration)
- Which candle/tick processing points to instrument for counters
- Whether to use OTel metrics API or Prometheus client directly

### Deferred Ideas (OUT OF SCOPE)
- Signal decision logging -- why a trade was/wasn't placed after a signal. This happens in the Python strategy client, not the Go server. Deferred to Phase 4.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ORD-01 | Order placement emits structured log with playground_id, symbol, side, quantity, order_type, environment="live" | Hook into `DatabaseService.PlaceOrders()` or `grpc.go PlaceOrder` handler; playground Meta has Environment field |
| ORD-02 | Order fill emits structured log with fill price, quantity, and timestamp | Hook into `OrderRecord.Fill()` method or `services/order_queue.go` fill processing |
| ORD-03 | Order rejection emits structured log with rejection reason | Hook into `DatabaseService.RejectOrder()` and `OrderRecord.Reject()` |
| ORD-04 | Only live playgrounds emit order telemetry; simulator excluded | Filter on `playground.Meta.Environment != PlaygroundEnvironmentSimulator` |
| DATA-01 | Candle arrival events logged with symbol, timeframe, timestamp | Counter increment in `Playground.simulateTick()` when `newCandle != nil` |
| DATA-02 | Tick processing events logged with processing latency | Timer around `simulateTick` or `NextTick` handler |
| DATA-03 | Data gaps emit warning-level logs | Already partially present (`log.Warnf` in simulateTick for missing candles); formalize with structured fields |
| BEAT-01 | Go server emits heartbeat metric gauge every 30s | Background goroutine in cmd/main.go with `time.NewTicker(30 * time.Second)` |
| BEAT-02 | Go server emits periodic structured heartbeat log with active playground count and last tick time | Same goroutine emits `log.WithFields` heartbeat entry |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go.opentelemetry.io/otel | v1.27.0 | OTel API (Tracer, Meter) | Already in go.mod, global MeterProvider initialized |
| go.opentelemetry.io/otel/metric | v1.27.0 | Metric instruments (Counter, Gauge) | Already in go.mod as indirect; promote to direct |
| go.opentelemetry.io/otel/attribute | v1.27.0 | Metric attributes/labels | Already used in codebase for span attributes |
| github.com/sirupsen/logrus | (existing) | Structured logging | Already the project's logger; otellogrus hook bridges to OTel |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go.opentelemetry.io/otel/sdk/metric | v1.27.0 | MeterProvider SDK (already init) | Already configured in utils.SetupOTelSDK |
| github.com/uptrace/opentelemetry-go-extra/otellogrus | (existing) | Logrus-to-OTel bridge | Already configured in cmd/main.go |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| OTel metrics API | prometheus/client_golang | Would add a separate dependency and require /metrics endpoint; OTel already exports to Prometheus via Collector |

**Installation:**
```bash
# No new packages needed -- all OTel packages already in go.mod
# go.opentelemetry.io/otel/metric may need promoting from indirect to direct
go mod tidy
```

## Architecture Patterns

### Recommended Project Structure
```
src/go/
├── backtester-api/
│   ├── router/grpc.go           # Add order lifecycle logs to PlaceOrder handler
│   ├── models/playground.go     # Add candle counter increment in simulateTick
│   └── services/order_queue.go  # Add fill/rejection logs in order processing
├── telemetry/                   # NEW: Centralized metric definitions
│   └── metrics.go               # Meter instance, all metric instruments, label constants
├── data/
│   └── database_service.go      # Add order fill/rejection telemetry in PlaceOrders/RejectOrder
└── cmd/
    └── main.go                  # Start heartbeat goroutine
```

### Pattern 1: Centralized Metrics Registry
**What:** A single `telemetry/metrics.go` file that creates the Meter and all metric instruments at package init time.
**When to use:** Always -- avoids scattered `otel.GetMeterProvider().Meter()` calls and ensures consistent naming.
**Example:**
```go
// src/go/telemetry/metrics.go
package telemetry

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

var (
    Meter = otel.GetMeterProvider().Meter("grodt")

    // Order lifecycle counters
    OrdersPlaced   metric.Int64Counter
    OrdersFilled   metric.Int64Counter
    OrdersRejected metric.Int64Counter

    // Market data counters
    CandlesProcessed metric.Int64Counter
    SignalsGenerated metric.Int64Counter
    SignalsProcessed metric.Int64Counter

    // Heartbeat gauges
    ActivePlaygrounds   metric.Int64Gauge
    OpenOrders          metric.Int64Gauge
    UptimeSeconds       metric.Float64Gauge
)

func Init() error {
    var err error

    OrdersPlaced, err = Meter.Int64Counter("grodt.orders.placed",
        metric.WithDescription("Number of orders placed"),
        metric.WithUnit("{order}"))
    if err != nil {
        return err
    }

    // ... similar for other instruments
    return nil
}
```

### Pattern 2: Environment-Filtered Telemetry
**What:** Check `playground.Meta.Environment` before emitting order telemetry. Include live + reconcile, exclude simulator.
**When to use:** Every order lifecycle log/metric emission point.
**Example:**
```go
func shouldEmitOrderTelemetry(env models.PlaygroundEnvironment) bool {
    return env == models.PlaygroundEnvironmentLive ||
           env == models.PlaygroundEnvironmentReconcile
}
```

### Pattern 3: Metric Label Taxonomy
**What:** Consistent OTel attribute set for all metrics, using environment and account_type as dimensions.
**When to use:** All metric emissions.
**Recommended labels:**
```go
import "go.opentelemetry.io/otel/attribute"

// Standard attributes for all playground-scoped metrics
func PlaygroundAttrs(env models.PlaygroundEnvironment, accountType models.LiveAccountType) attribute.Set {
    return attribute.NewSet(
        attribute.String("environment", string(env)),
        attribute.String("account_type", string(accountType)),
    )
}
```

### Pattern 4: Heartbeat Background Goroutine
**What:** A goroutine started from cmd/main.go that ticks every 30s, computes stats from DatabaseService, and emits gauge + log.
**When to use:** Server startup.
**Example:**
```go
func startHeartbeat(ctx context.Context, dbService *data.DatabaseService) {
    ticker := time.NewTicker(30 * time.Second)
    startTime := time.Now()
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            playgrounds := dbService.GetPlaygrounds()
            // Count by environment, compute stats
            // Emit gauge metrics + structured log
        }
    }
}
```

### Anti-Patterns to Avoid
- **Per-candle log entries:** Too noisy for live mode. Use counters instead (D-09).
- **Creating new Meter instances everywhere:** Use a single centralized Meter to ensure consistent naming.
- **Hardcoding metric attribute values:** Use the model constants (`PlaygroundEnvironmentLive`, etc.) to ensure consistency with the domain model.
- **Blocking heartbeat on DB queries:** GetPlaygrounds() already iterates an in-memory map, so it is fast. Do not add DB queries to the heartbeat tick.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Metric instruments | Custom counter struct | `otel/metric` Int64Counter, Int64Gauge | Thread-safe, auto-exported via OTLP |
| Periodic background tasks | Manual goroutine with sleep | `time.NewTicker` | Proper cleanup, no drift |
| Structured logging | fmt.Printf with JSON | logrus `WithFields` | Already standardized, bridges to OTel via otellogrus |
| Metric export to Prometheus | /metrics endpoint + prom client | OTel Collector routing | Collector already configured in Phase 2 to route metrics to Prometheus |

## Common Pitfalls

### Pitfall 1: OTel Meter Not Ready at Init Time
**What goes wrong:** If `telemetry.Init()` is called before `utils.SetupOTelSDK()`, the global MeterProvider is a no-op and metrics silently disappear.
**Why it happens:** Go package init() runs before main(), and `otel.GetMeterProvider()` returns a no-op provider until SetupOTelSDK is called.
**How to avoid:** Call `telemetry.Init()` AFTER `utils.SetupOTelSDK()` in cmd/main.go. Do NOT use package-level `var Meter = otel.GetMeterProvider().Meter("grodt")` -- it will capture the no-op provider.
**Warning signs:** Metrics show up in logs but not in Prometheus/Grafana.

### Pitfall 2: High Cardinality Metric Labels
**What goes wrong:** Using playground_id as a metric label creates a time series per playground. With 37+ playgrounds, this is manageable but could grow.
**Why it happens:** Each unique label combination creates a separate time series in Prometheus.
**How to avoid:** Use playground_id in logs (unlimited cardinality) but NOT in metrics. Metric labels should be low-cardinality: environment (3 values), account_type (5 values). Per-playground detail goes in structured logs only.
**Warning signs:** Prometheus memory usage spikes, cardinality warnings.

### Pitfall 3: Missing Environment Filter on Simulator Playgrounds
**What goes wrong:** Simulator playgrounds flood logs with order telemetry during backtesting.
**Why it happens:** Forgetting the environment check before logging.
**How to avoid:** Use the `shouldEmitOrderTelemetry()` helper consistently at every instrumentation point.
**Warning signs:** Log volume spikes during backtesting.

### Pitfall 4: Heartbeat Goroutine Leak
**What goes wrong:** Heartbeat goroutine continues running after context cancellation.
**Why it happens:** Not checking ctx.Done() in the select loop, or not deferring ticker.Stop().
**How to avoid:** Always use `select { case <-ctx.Done(): return; case <-ticker.C: ... }` pattern. cmd/main.go already cancels context on SIGTERM.
**Warning signs:** Goroutine count grows in runtime metrics.

### Pitfall 5: Fill Telemetry in Wrong Place
**What goes wrong:** Instrumenting `OrderRecord.Fill()` directly would trigger for simulator fills too, and the method is a model method without access to playground environment.
**Why it happens:** The Fill method is on the OrderRecord, which does not know its playground's environment.
**How to avoid:** Instrument at the DatabaseService level or order_queue service level where playground context is available. The `services/order_queue.go` `UpdatePendingMarginOrders` function processes fills for live/reconcile orders specifically.
**Warning signs:** Simulator order fills appearing in telemetry.

## Code Examples

### Existing PlaceOrder Handler (instrumentation target)
```go
// src/go/backtester-api/router/grpc.go:1087
func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.Order, error) {
    logger := log.WithFields(log.Fields{
        "trace_id":      req.TraceId,
        "playground_id": req.PlaygroundId,
        "symbol":        req.Symbol,
        "side":          req.Side,
    })
    logger.Info("PlaceOrder:start")
    // ... existing code
}
```
The existing handler already has trace_id and basic fields. Enhancement: add quantity, order_type, environment, and filter by environment.

### Existing Order Status Lifecycle
```go
// src/go/backtester-api/models/backtester_order_status.go
const (
    OrderRecordStatusNew             = "new"
    OrderRecordStatusPending         = "pending"
    OrderRecordStatusPartiallyFilled = "partially_filled"
    OrderRecordStatusFilled          = "filled"
    OrderRecordStatusExpired         = "expired"
    OrderRecordStatusCanceled        = "canceled"
    OrderRecordStatusRejected        = "rejected"
)
```

### Existing Environment and Account Type Enums
```go
// PlaygroundEnvironment: "simulator", "live", "reconcile"
// LiveAccountType: "paper", "margin", "reconcilation", "mock", "simulator"
```

### Key Instrumentation Points
1. **Order Placed:** `grpc.go:PlaceOrder` handler (line 1087) -- already has logger, add environment check
2. **Order Filled:** `services/order_queue.go:UpdatePendingMarginOrders` -- processes fills for live/reconcile
3. **Order Rejected:** `data/database_service.go:RejectOrder` (line 1183) and `services/order_queue.go` rejection handling
4. **Candle Processed:** `models/playground.go:simulateTick` (line 1594-1615) -- when `newCandle != nil`
5. **Data Gaps:** `models/playground.go:simulateTick` (line 1505) -- already has `log.Warnf` for "no candles found"
6. **Heartbeat:** New goroutine in `cmd/main.go`, uses `dbService.GetPlaygrounds()` to iterate in-memory map

### OTel Int64Counter Usage (v1.27.0)
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

// After SetupOTelSDK has been called:
meter := otel.GetMeterProvider().Meter("grodt")

counter, _ := meter.Int64Counter("grodt.orders.placed",
    metric.WithDescription("Number of orders placed"),
    metric.WithUnit("{order}"),
)

counter.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("environment", "live"),
        attribute.String("account_type", "paper"),
        attribute.String("symbol", "AAPL"),
    ),
)
```

### OTel Int64Gauge Usage (v1.27.0)
```go
// Int64Gauge was added in OTel Go v1.27.0
gauge, _ := meter.Int64Gauge("grodt.heartbeat.active_playgrounds",
    metric.WithDescription("Number of active playgrounds"),
    metric.WithUnit("{playground}"),
)

gauge.Record(ctx, int64(count),
    metric.WithAttributes(
        attribute.String("environment", "live"),
        attribute.String("account_type", "paper"),
    ),
)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Prometheus client library | OTel metrics API | OTel Go 1.0+ (2023) | Unified API, Collector handles routing |
| Custom gauge with callback | Int64Gauge.Record() | OTel Go v1.27.0 (2024) | Simpler synchronous API, no need for Observable* |
| fmt.Println for logs | logrus + otellogrus | Phase 2 | Logs bridge to OTel Collector, structured fields |

**Note:** OTel Go v1.27.0 introduced synchronous `Int64Gauge` and `Float64Gauge`. Prior versions required `Observable` (callback-based) gauges. Since this project uses v1.27.0, synchronous gauges are available.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (assert/require) |
| Config file | none (standard `go test`) |
| Quick run command | `go test -count=1 ./... -run TestTelemetry` |
| Full suite command | `cd src/go/backtester-api && go test -count=1 ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ORD-01 | Order placement log fields | unit | `go test -count=1 -run TestOrderPlacedLog ./src/go/backtester-api/router/` | No - Wave 0 |
| ORD-02 | Order fill log fields | unit | `go test -count=1 -run TestOrderFilledLog ./src/go/backtester-api/...` | No - Wave 0 |
| ORD-03 | Order rejection log with reason | unit | `go test -count=1 -run TestOrderRejectedLog ./src/go/backtester-api/...` | No - Wave 0 |
| ORD-04 | Simulator excluded from telemetry | unit | `go test -count=1 -run TestSimulatorExcluded ./src/go/...` | No - Wave 0 |
| DATA-01 | Candle counter increments | unit | `go test -count=1 -run TestCandleCounter ./src/go/...` | No - Wave 0 |
| DATA-02 | Tick processing latency | unit | `go test -count=1 -run TestTickLatency ./src/go/...` | No - Wave 0 |
| DATA-03 | Data gap warning logs | unit | Already has `log.Warnf` -- verify with structured fields | No - Wave 0 |
| BEAT-01 | Heartbeat gauge emits every 30s | unit | `go test -count=1 -run TestHeartbeat ./src/go/...` | No - Wave 0 |
| BEAT-02 | Heartbeat structured log | unit | `go test -count=1 -run TestHeartbeatLog ./src/go/...` | No - Wave 0 |

### Sampling Rate
- **Per task commit:** `cd src/go/backtester-api && go test -count=1 ./...`
- **Per wave merge:** `task test`
- **Phase gate:** `task test` green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `src/go/telemetry/metrics_test.go` -- covers metric registration and counter increment verification
- [ ] Test helper for capturing logrus output (logrus hook or buffer) to verify structured fields
- [ ] Test helper for in-memory OTel metric reader (use `sdkmetric.NewManualReader` for metric assertions)

## Open Questions

1. **Where exactly should fill telemetry hook in?**
   - What we know: `OrderRecord.Fill()` is the model method, `services/order_queue.go` handles live fills, `data/database_service.go` has `SaveOrderRecord`. Multiple paths lead to fills.
   - What's unclear: The cleanest single point to instrument without missing any fill path.
   - Recommendation: Instrument in `DatabaseService.SaveOrderRecord` or add a post-fill callback. Alternatively, instrument `UpdatePendingMarginOrders` in `services/order_queue.go` since it specifically handles live/reconcile fills and already has playground context.

2. **Should candle counter be in-process or OTel metric?**
   - What we know: D-09 says "counter of candles processed per playground." This could be a logrus field on periodic logs or an OTel counter.
   - Recommendation: OTel counter with playground environment + account_type labels (not playground_id to avoid cardinality). Periodic heartbeat log can include the total count as a structured field.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `src/go/utils/otel.go`, `cmd/main.go`, `src/go/backtester-api/router/grpc.go`
- Codebase analysis: `go.mod` confirms OTel v1.27.0 with metric, trace, and runtime packages
- Codebase analysis: `src/go/backtester-api/models/playground_environment.go`, `live_account_type.go`, `playground_meta.go`

### Secondary (MEDIUM confidence)
- OTel Go SDK v1.27.0 API -- Int64Gauge synchronous API available (verified from go.mod version)
- otellogrus bridge already configured in cmd/main.go line 287

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all packages already in go.mod, no new dependencies
- Architecture: HIGH - clear instrumentation points identified in codebase
- Pitfalls: HIGH - based on direct code analysis of environment filtering, model vs service boundaries

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable OTel Go SDK, no upcoming breaking changes)
