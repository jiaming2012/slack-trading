---
phase: 03-go-telemetry-instrumentation
plan: 01
subsystem: telemetry
tags: [opentelemetry, metrics, structured-logging, order-lifecycle]

requires:
  - phase: 02-go-otel-foundation-local-backend
    provides: "SetupOTelSDK function with TracerProvider and MeterProvider"
provides:
  - "telemetry package with Init(), ShouldEmitOrderTelemetry(), PlaygroundAttrs()"
  - "8 OTel metric instruments (counters and gauges)"
  - "Order lifecycle structured logs (placed, filled, rejected)"
  - "Environment-based telemetry filtering (live/reconcile only)"
affects: [03-02-PLAN, 03-03-PLAN]

tech-stack:
  added: []
  patterns: ["telemetry gating via ShouldEmitOrderTelemetry", "PlaygroundAttrs for consistent OTel dimensions"]

key-files:
  created:
    - "src/go/telemetry/metrics.go"
    - "src/go/telemetry/metrics_test.go"
  modified:
    - "cmd/main.go"
    - "src/go/data/database_service.go"
    - "src/go/backtester-api/services/order_queue.go"

key-decisions:
  - "Placed order telemetry inside DatabaseService.PlaceOrders rather than grpc.go PlaceOrder handler, since playground is already fetched and environment check exists"
  - "Nil-guarded counter increments (if telemetry.OrdersPlaced != nil) to handle case where Init() not called"
  - "Used context.Background() for counter Add calls since order operations don't carry request context"

patterns-established:
  - "Telemetry gating: always check telemetry.ShouldEmitOrderTelemetry(env) before emitting"
  - "Structured log format: event field + domain fields + environment/account_type dimensions"
  - "Counter increment: nil-guard + PlaygroundAttrs for consistent attribute dimensions"

requirements-completed: [ORD-01, ORD-02, ORD-03, ORD-04]

duration: 6min
completed: 2026-03-26
---

# Phase 03 Plan 01: Telemetry Metrics Package + Order Lifecycle Instrumentation Summary

**Centralized OTel metrics registry with 8 instruments, plus structured logs and counters for order placed/filled/rejected events gated to live/reconcile playgrounds**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-26T21:08:03Z
- **Completed:** 2026-03-26T21:14:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Created telemetry package with Init(), ShouldEmitOrderTelemetry(), and PlaygroundAttrs() helpers
- Defined 8 metric instruments: 5 counters (orders placed/filled/rejected, candles processed, signals generated) and 3 gauges (active playgrounds, open orders, uptime)
- Instrumented order lifecycle with structured logs containing playground_id, order_id, symbol, environment, account_type
- Simulator playgrounds excluded from all order telemetry via ShouldEmitOrderTelemetry filter

## Task Commits

Each task was committed atomically:

1. **Task 1: Create telemetry package with metrics registry and environment filter** - `dd0fe24` (test: RED), `786c5f0` (feat: GREEN)
2. **Task 2: Instrument order lifecycle with structured logs and counters** - `e3b9f3f` (feat)

## Files Created/Modified
- `src/go/telemetry/metrics.go` - Centralized metrics package with Init(), ShouldEmitOrderTelemetry(), PlaygroundAttrs(), and 8 metric instrument vars
- `src/go/telemetry/metrics_test.go` - Unit tests for environment filter, Init() instrument creation, and PlaygroundAttrs
- `cmd/main.go` - Added telemetry.Init() call after SetupOTelSDK
- `src/go/data/database_service.go` - Order placed and rejected structured logs + OTel counters
- `src/go/backtester-api/services/order_queue.go` - Order filled structured log + OTel counter

## Decisions Made
- Placed order telemetry inside DatabaseService.PlaceOrders rather than grpc.go PlaceOrder handler, since playground is already fetched there and environment check already exists at line 1259
- Nil-guarded counter increments to handle the case where telemetry.Init() was not called (e.g., OTel SDK init failed)
- Used context.Background() for counter Add calls since order operations in DatabaseService/services don't carry a request context through

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing test failures in `src/go/backtester-api/models/` package (TestSymbol, TestCalendar, TestOpenOrdersCache, TestLiquidation, TestFeed) -- verified these exist before any changes and are unrelated to telemetry work

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Telemetry package ready for 03-02 (market data instrumentation) and 03-03 (heartbeat)
- All metric instruments pre-created and available for import
- ShouldEmitOrderTelemetry pattern established for consistent environment filtering

## Self-Check: PASSED

- FOUND: src/go/telemetry/metrics.go
- FOUND: src/go/telemetry/metrics_test.go
- FOUND: commit dd0fe24 (test RED)
- FOUND: commit 786c5f0 (feat GREEN)
- FOUND: commit e3b9f3f (feat Task 2)

---
*Phase: 03-go-telemetry-instrumentation*
*Completed: 2026-03-26*
