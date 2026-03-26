---
phase: 03-go-telemetry-instrumentation
plan: 02
subsystem: telemetry
tags: [opentelemetry, metrics, structured-logging, logrus, candles, latency]

# Dependency graph
requires:
  - phase: 03-01
    provides: "telemetry package with CandlesProcessed, SignalsGenerated counters, ShouldEmitOrderTelemetry, PlaygroundAttrs"
provides:
  - "CandlesProcessed counter increments in simulateTick for live/reconcile playgrounds"
  - "Structured data_gap warnings with symbol, timeframe, timestamp fields"
  - "tick_latency_ms measurement in NextTick handler"
  - "SignalsGenerated counter increment in ProcessSignalTriggeredEvent"
affects: [03-go-telemetry-instrumentation, grafana-dashboards]

# Tech tracking
tech-stack:
  added: []
  patterns: ["string-typed telemetry params to avoid import cycles", "nil-safe counter increments for test compatibility"]

key-files:
  created: []
  modified:
    - src/go/backtester-api/models/playground.go
    - src/go/backtester-api/router/grpc.go
    - src/go/eventconsumers/process_signals.go
    - src/go/telemetry/metrics.go
    - src/go/telemetry/metrics_test.go
    - src/go/data/database_service.go
    - src/go/backtester-api/services/order_queue.go

key-decisions:
  - "Changed telemetry.ShouldEmitOrderTelemetry and PlaygroundAttrs to accept string params to break import cycle between telemetry and models packages"

patterns-established:
  - "Nil-safe counter pattern: check counter != nil before Add() to handle uninitialized telemetry in tests"
  - "Environment filter pattern: ShouldEmitOrderTelemetry gates counter increments to live/reconcile only"

requirements-completed: [DATA-01, DATA-02, DATA-03]

# Metrics
duration: 7min
completed: 2026-03-26
---

# Phase 03 Plan 02: Market Data Flow Instrumentation Summary

**Candle counter increments in simulateTick, tick latency measurement in NextTick, structured data gap warnings, and signal counter in ProcessSignalTriggeredEvent**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-26T21:18:12Z
- **Completed:** 2026-03-26T21:25:19Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- CandlesProcessed OTel counter increments in both simulateTick code paths (preview and non-preview) for live/reconcile playgrounds
- Structured data_gap warnings replace raw log.Warnf with queryable fields (symbol, timeframe, timestamp, environment, playground_id)
- Tick processing latency measured and logged as tick_latency_ms Debug field in NextTick handler
- SignalsGenerated counter incremented in ProcessSignalTriggeredEvent with nil-safe check

## Task Commits

Each task was committed atomically:

1. **Task 1: Add candle counter and structured data gap warnings in simulateTick** - `b6ed602` (feat)
2. **Task 2: Add tick processing latency measurement in NextTick handler** - `9bfc0e0` (feat)
3. **Task 3: Increment SignalsGenerated counter in signal processing** - `ddfaf7a` (feat)

## Files Created/Modified
- `src/go/backtester-api/models/playground.go` - CandlesProcessed counter in both newCandle paths, structured data_gap warning
- `src/go/backtester-api/router/grpc.go` - tick_latency_ms measurement around nextTick call
- `src/go/eventconsumers/process_signals.go` - SignalsGenerated counter increment
- `src/go/telemetry/metrics.go` - Changed ShouldEmitOrderTelemetry/PlaygroundAttrs to accept strings (break import cycle)
- `src/go/telemetry/metrics_test.go` - Updated tests for string params
- `src/go/data/database_service.go` - Updated callers for string params
- `src/go/backtester-api/services/order_queue.go` - Updated callers for string params

## Decisions Made
- Changed telemetry package to accept string parameters instead of model types to break import cycle (telemetry imported models, models now needs to import telemetry). This is a clean solution since PlaygroundEnvironment and LiveAccountType are both string type aliases.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Broke import cycle between telemetry and models packages**
- **Found during:** Task 1 (candle counter in playground.go)
- **Issue:** telemetry package imported backtester-api/models for type parameters; playground.go (in models) importing telemetry created a cycle
- **Fix:** Changed ShouldEmitOrderTelemetry(env models.PlaygroundEnvironment) to ShouldEmitOrderTelemetry(env string), same for PlaygroundAttrs. Updated all 4 caller sites (database_service.go, order_queue.go, playground.go, metrics_test.go)
- **Files modified:** src/go/telemetry/metrics.go, src/go/telemetry/metrics_test.go, src/go/data/database_service.go, src/go/backtester-api/services/order_queue.go
- **Verification:** go build ./src/go/... passes, go test ./src/go/telemetry/... passes
- **Committed in:** b6ed602 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Import cycle fix was necessary for correctness. String params are cleaner than the typed params since they avoid cross-package coupling. No scope creep.

## Issues Encountered
- Pre-existing test failures (TestSymbol, TestCalendar, id_remap tests) due to missing .env file and PostgreSQL -- not related to this plan's changes.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Market data flow instrumentation complete
- Candle counters, tick latency, data gap warnings, and signal counters all wired
- Ready for Phase 03 Plan 03 (heartbeat/infrastructure metrics)

---
*Phase: 03-go-telemetry-instrumentation*
*Completed: 2026-03-26*
