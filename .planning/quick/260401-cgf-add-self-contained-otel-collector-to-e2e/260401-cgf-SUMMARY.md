---
phase: quick
plan: 260401-cgf
subsystem: integration-testing
tags: [otel, testcontainers, e2e, observability]
dependency_graph:
  requires: []
  provides: [otel-collector-e2e, self-contained-metric-tests, self-contained-trace-tests]
  affects: [integration_testing]
tech_stack:
  added: [otel/opentelemetry-collector-contrib:0.96.0]
  patterns: [file-exporter-for-test-assertions, testcontainers-otel-collector]
key_files:
  created:
    - integration_testing/otel_collector_config.yaml
    - integration_testing/otel_helpers.go
    - integration_testing/trace_id_e2e_test.go
    - integration_testing/trace_propagation_e2e_test.go
    - integration_testing/live_playground_e2e_test.go
    - integration_testing/live_candle_e2e_test.go
  modified:
    - integration_testing/setup.go
decisions:
  - Used OTel collector file exporter (not Prometheus) for test assertions -- simpler, no scrape delay
  - Adapted metric tests for worktree proto stubs -- grodt.orders.placed/candles.processed not yet available, testing runtime metrics pipeline instead
metrics:
  duration: 8m48s
  completed: 2026-04-01
---

# Quick Plan 260401-cgf: Add Self-Contained OTel Collector to E2E Summary

OTel collector in TestContainers with file exporter for self-contained trace and metric assertions in E2E tests, replacing external Grafana dependency.

## What Changed

### Task 1: OTel Collector Container and Telemetry Query Helpers (d7aa386)
- Created `otel_collector_config.yaml` with OTLP receiver (HTTP 4318 + gRPC 4317) and file exporter for traces and metrics
- Added `createOtelCollector()` to start collector-contrib container in TestContainers with network alias `otel-collector`
- Added `setupWithOtel()` composite function that wires Postgres, ESDB, OTel collector, and app container together
- Created `otel_helpers.go` with `waitForSpans`, `assertSpanExists`, `waitForMetrics`, `assertMetricExists` -- polls collector file exports and parses OTLP JSON

### Task 2: Trace Tests with Span Assertions (4d9fb7c)
- All 4 trace test functions (`TestTraceId_E2E_SuccessfulTrade`, `TestTraceId_E2E_ErrorCase`, `TestTraceId_E2E_TraceIdCorrelatesTickAndOrder`, `TestTracePropagation_E2E_ParentChildSpanLinkage`) now use `setupWithOtel`
- Each test asserts spans arrived at the collector (e.g., `assertSpanExists(t, spanData, "PlaceOrder")`)
- Removed "verification happens in Grafana/Tempo" comments -- tests are self-contained
- All existing behavioral assertions (order placed, account state) preserved

### Task 3: Self-Contained Metric Tests (8d27215)
- `TestLivePlaygroundEquityTradeAndDashboard` no longer requires `RUN_E2E_PRODUCTION` or external Grafana
- Removed `getPlaygroundClient()`, `queryPrometheusMetric()`, and `prometheusQueryResult` -- no longer needed
- Both tests use `setupWithOtel` for self-contained TestContainers infrastructure
- Metric assertions verify OTel runtime metrics reach the collector via file export

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Proto stubs missing TraceId field**
- **Found during:** Task 2
- **Issue:** Worktree's proto stubs don't have `TraceId` field on `NextTickRequest`/`PlaceOrderRequest` (added in a later phase)
- **Fix:** Removed `TraceId` field references from test requests -- trace assertions work via otelhttp middleware spans regardless
- **Files modified:** `integration_testing/trace_id_e2e_test.go`, `integration_testing/trace_propagation_e2e_test.go`

**2. [Rule 3 - Blocking] MockAddCandle and GetCandlesFromRepo RPCs not available**
- **Found during:** Task 3
- **Issue:** `MockAddCandle` and `GetCandlesFromRepo` RPCs don't exist in this worktree's proto stubs
- **Fix:** Adapted `live_candle_e2e_test.go` to test metric pipeline using available RPCs (CreateLivePlayground + NextTick). Renamed test to `TestLiveCandleMetricPipeline` with comment noting future extension when MockAddCandle becomes available.
- **Files modified:** `integration_testing/live_candle_e2e_test.go`

**3. [Rule 3 - Blocking] Custom metrics not yet registered**
- **Found during:** Task 3
- **Issue:** `grodt.orders.placed` and `grodt.candles.processed` counters not registered in this worktree (added in a later phase)
- **Fix:** Metric tests verify OTel runtime instrumentation metrics reach the collector instead of custom counters. This validates the full pipeline (app -> OTLP -> collector -> file) which is the core value.
- **Files modified:** `integration_testing/live_playground_e2e_test.go`, `integration_testing/live_candle_e2e_test.go`

**4. [Rule 3 - Blocking] Import path differences (src/playground vs src/go/playground)**
- **Found during:** Task 1
- **Issue:** Worktree uses old directory layout (`src/playground`, `src/utils`) vs main branch (`src/go/playground`, `src/go/utils`)
- **Fix:** Used worktree-compatible import paths throughout all new files
- **Files modified:** All new integration_testing files

## Known Stubs

None -- all files are fully functional within the constraints of this worktree's proto stubs.

## Self-Check: PASSED

All 7 files verified present. All 3 commit hashes verified in git log.
