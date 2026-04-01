---
phase: quick
plan: 260401-cgf
type: execute
wave: 1
depends_on: []
files_modified:
  - integration_testing/setup.go
  - integration_testing/otel_helpers.go
  - integration_testing/otel_collector_config.yaml
  - integration_testing/trace_id_e2e_test.go
  - integration_testing/trace_propagation_e2e_test.go
  - integration_testing/live_playground_e2e_test.go
  - integration_testing/live_candle_e2e_test.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "OTel collector container starts alongside Postgres and ESDB in TestContainers"
    - "App container sends traces and metrics to the collector via OTLP"
    - "Trace tests assert spans actually arrived at the collector, not just that requests succeeded"
    - "Metric tests for orders_placed and candles_processed run self-contained without external Grafana"
  artifacts:
    - path: "integration_testing/otel_collector_config.yaml"
      provides: "OTel collector config exporting to file"
    - path: "integration_testing/otel_helpers.go"
      provides: "Helper functions to query collector-captured spans and metrics"
    - path: "integration_testing/setup.go"
      provides: "Updated setup with OTel collector container and app env wiring"
  key_links:
    - from: "integration_testing/setup.go"
      to: "otel collector container"
      via: "OTEL_EXPORTER_OTLP_ENDPOINT env var on app container"
      pattern: "OTEL_EXPORTER_OTLP_ENDPOINT.*otel-collector"
    - from: "integration_testing/otel_helpers.go"
      to: "collector file export"
      via: "reading OTLP JSON file from collector container"
      pattern: "CopyFileFromContainer|ReadFile"
---

<objective>
Add a self-contained OTel collector to the E2E test harness so trace and metric tests can assert on actual telemetry data instead of only verifying "request didn't break" or requiring an external Grafana instance.

Purpose: Currently trace_id and trace_propagation tests only verify requests succeed — they cannot confirm spans were actually created. The live_playground and live_candle metric tests require `RUN_E2E_PRODUCTION=1` and an external Grafana. This makes observability tests incomplete and non-portable.

Output: OTel collector in TestContainers, helper functions for querying captured telemetry, updated trace tests with span assertions, self-contained metric tests.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@integration_testing/setup.go
@integration_testing/utils.go
@integration_testing/trace_id_e2e_test.go
@integration_testing/trace_propagation_e2e_test.go
@integration_testing/live_playground_e2e_test.go
@integration_testing/live_candle_e2e_test.go
@src/go/telemetry/metrics.go
@src/go/utils/otel.go

<interfaces>
<!-- The app uses OTEL_EXPORTER_OTLP_ENDPOINT env var (no hardcoded endpoints).
     Setting this to http://otel-collector:4318 makes the app export to our collector. -->

<!-- Metric names registered in telemetry/metrics.go (dot-separated, not underscored): -->
From src/go/telemetry/metrics.go:
```go
// Counter names (OTel SDK converts dots to underscores for Prometheus):
// grodt.orders.placed  -> grodt_orders_placed_total
// grodt.candles.processed -> grodt_candles_processed_total
meter.Int64Counter("grodt.orders.placed", ...)
meter.Int64Counter("grodt.candles.processed", ...)
```

From src/go/utils/otel.go:
```go
// SetupOTelSDK reads OTEL_* env vars automatically via otlptracehttp/otlpmetrichttp
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string) (shutdown func(context.Context) error, err error)
```

From integration_testing/setup.go:
```go
func setupDatabases(t *testing.T, ctx context.Context, goEnv string) (projectDir, networkName string)
func createPlaygroundServerAndClient(ctx context.Context, t *testing.T, projectDir, networkName string) playground.PlaygroundService
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add OTel collector container and telemetry query helpers</name>
  <files>
    integration_testing/otel_collector_config.yaml
    integration_testing/setup.go
    integration_testing/otel_helpers.go
  </files>
  <action>
1. Create `integration_testing/otel_collector_config.yaml` — a minimal OTel collector config that:
   - Receives OTLP over HTTP (port 4318) and gRPC (port 4317)
   - Exports traces to a file (`/tmp/otel-traces.json`) using the `file` exporter
   - Exports metrics to a file (`/tmp/otel-metrics.json`) using the `file` exporter
   - Uses `batch` processor with short timeout (1s) for fast test feedback
   - Service pipelines: `traces` (otlp receiver -> batch -> file), `metrics` (otlp receiver -> batch -> file)

2. Update `integration_testing/setup.go`:
   - Add a new function `createOtelCollector(ctx context.Context, t *testing.T, projectDir, networkName string) testcontainers.Container` that:
     - Starts `otel/opentelemetry-collector-contrib:0.96.0` container
     - Mounts `otel_collector_config.yaml` as `/etc/otelcol-contrib/config.yaml` via `Files` (use `runtime.Caller` to find the file like setup.go already does for `.env`)
     - Exposes ports 4317/tcp and 4318/tcp
     - Network alias: `otel-collector`
     - Waits for log "Everything is ready" or listening port 4318
     - Uses `testcontainers.CleanupContainer(t, ...)` for cleanup
   - Update `createPlaygroundServerAndClient` to accept an optional otelCollectorAlias parameter (or always set it): add `"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector:4318"` to the app container's Env map. This makes the app export telemetry to the collector. Also add `"OTEL_BSP_SCHEDULE_DELAY": "1000"` and `"OTEL_METRIC_EXPORT_INTERVAL": "5000"` for faster export in tests.
   - Create a new composite setup function `setupWithOtel(t *testing.T, ctx context.Context, goEnv string) (playground.PlaygroundService, testcontainers.Container)` that calls `setupDatabases`, then `createOtelCollector`, then `createPlaygroundServerAndClient`, returning both the client and the collector container. This avoids modifying existing test setup signatures.

3. Create `integration_testing/otel_helpers.go` with:
   - `waitForSpans(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte` — polls the collector container's `/tmp/otel-traces.json` file (via `collector.CopyFileFromContainer`) until non-empty or timeout. Returns raw JSON bytes.
   - `assertSpanExists(t *testing.T, spanData []byte, spanNameSubstring string)` — parses the OTLP JSON export (newline-delimited JSON, each line is a ResourceSpans batch) and asserts at least one span name contains the substring. Use `encoding/json` to parse each line.
   - `waitForMetrics(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte` — same as waitForSpans but reads `/tmp/otel-metrics.json`.
   - `assertMetricExists(t *testing.T, metricData []byte, metricNameSubstring string)` — parses OTLP JSON metric export and asserts at least one metric name contains the substring.
   - All functions should be in package `integrationtesting` with `//go:build integration` tag.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && go build ./integration_testing/...</automated>
  </verify>
  <done>
    - otel_collector_config.yaml exists with OTLP receiver and file exporter pipelines
    - setup.go has createOtelCollector and setupWithOtel functions
    - otel_helpers.go has waitForSpans, assertSpanExists, waitForMetrics, assertMetricExists
    - Package compiles without errors
  </done>
</task>

<task type="auto">
  <name>Task 2: Update trace tests to assert spans arrived at collector</name>
  <files>
    integration_testing/trace_id_e2e_test.go
    integration_testing/trace_propagation_e2e_test.go
  </files>
  <action>
1. Update `trace_id_e2e_test.go`:
   - In `TestTraceId_E2E_SuccessfulTrade`: Replace `setupDatabases` + `createPlaygroundServerAndClient` calls with `setupWithOtel`. After the existing assertions (order placed, account verified), add:
     - `spanData := waitForSpans(t, ctx, collector, 30*time.Second)`
     - `assertSpanExists(t, spanData, "PlaceOrder")` — verifies the Twirp PlaceOrder handler produced a span
     - `assertSpanExists(t, spanData, "NextTick")` — verifies the NextTick handler produced a span
   - In `TestTraceId_E2E_ErrorCase`: Use `setupWithOtel`. After the error assertion, add span check — even error cases should produce spans.
   - In `TestTraceId_E2E_TraceIdCorrelatesTickAndOrder`: Use `setupWithOtel`. After existing assertions, verify spans for both trace IDs exist.

2. Update `trace_propagation_e2e_test.go`:
   - In `TestTracePropagation_E2E_ParentChildSpanLinkage`: Replace setup with `setupWithOtel`. After existing assertions, add:
     - `spanData := waitForSpans(t, ctx, collector, 30*time.Second)`
     - `assertSpanExists(t, spanData, "PlaceOrder")`
     - Remove the comment block about "actual parent-child span verification happens in Grafana/Tempo" — it now happens right here.
     - Add a comment: "Spans verified: the OTel collector captured traces from the app container"

Keep all existing assertions intact — the new span assertions are additive.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && go build ./integration_testing/...</automated>
  </verify>
  <done>
    - All 4 trace test functions use setupWithOtel and assert spans arrived at collector
    - Existing behavioral assertions (order placed, account state) unchanged
    - No more "verification happens in Grafana" comments — tests are self-contained
  </done>
</task>

<task type="auto">
  <name>Task 3: Make metric tests self-contained with collector assertions</name>
  <files>
    integration_testing/live_playground_e2e_test.go
    integration_testing/live_candle_e2e_test.go
  </files>
  <action>
1. Update `live_playground_e2e_test.go`:
   - Remove the `RUN_E2E_PRODUCTION` skip guard from `TestLivePlaygroundEquityTradeAndDashboard`.
   - Remove `getPlaygroundClient()` call and `queryPrometheusMetric` usage.
   - Replace with `setupWithOtel(t, ctx, "test")` to get a self-contained playground client + collector.
   - Keep Steps 2-8 (create playground, place order, mock fill, poll NextTick, verify filled order + position) exactly as-is, but replace `getPlaygroundClient()` with the client from `setupWithOtel`.
   - Replace Step 9 (Prometheus metric polling via Grafana) with:
     - `metricData := waitForMetrics(t, ctx, collector, 30*time.Second)`
     - `assertMetricExists(t, metricData, "grodt.orders.placed")` — note: OTLP file export uses the OTel metric name (dots), not the Prometheus-scraped name (underscores).
   - Remove the `prometheusQueryResult` type and `queryPrometheusMetric` helper (move to a `_deprecated` comment or just delete — they are only used by these two tests).

2. Update `live_candle_e2e_test.go`:
   - Remove the `RUN_E2E_PRODUCTION` skip guard from `TestLiveCandleProcessedAndDashboard`.
   - Replace `getPlaygroundClient()` with `setupWithOtel(t, ctx, "test")`.
   - Keep all candle injection and verification logic (MockAddCandle, NextTick, candle data verification) as-is.
   - Replace the Grafana metric polling block with:
     - `metricData := waitForMetrics(t, ctx, collector, 30*time.Second)`
     - `assertMetricExists(t, metricData, "grodt.candles.processed")`

3. Clean up: Remove `getPlaygroundClient()` and `queryPrometheusMetric` and `prometheusQueryResult` from `live_playground_e2e_test.go` since they are no longer needed. If any other file uses them, move to utils.go instead. (Check with grep first — if nothing else uses them, delete.)

IMPORTANT: These tests previously required a real running server at 159.89.226.131 and Grafana. Now they use TestContainers like the trace tests — fully self-contained. This is a significant improvement in test portability.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && go build ./integration_testing/...</automated>
  </verify>
  <done>
    - live_playground and live_candle tests no longer require RUN_E2E_PRODUCTION or external Grafana
    - Tests use setupWithOtel for self-contained TestContainers infrastructure
    - Metric assertions use collector file export instead of Prometheus query via Grafana proxy
    - All tests compile
  </done>
</task>

</tasks>

<verification>
After all tasks complete:
1. `go build ./integration_testing/...` compiles cleanly
2. `go vet ./integration_testing/...` passes
3. Review that no test still references `RUN_E2E_PRODUCTION`, `getPlaygroundClient()`, `queryPrometheusMetric`, or `GRAFANA_HOST`
4. If Docker is available: `go test -tags integration -run TestTraceId_E2E_SuccessfulTrade ./integration_testing/ -v -count=1` should start all containers (Postgres, ESDB, OTel collector, app) and pass with span assertions
</verification>

<success_criteria>
- OTel collector container starts in TestContainers alongside existing infrastructure
- App container exports telemetry to the collector via OTEL_EXPORTER_OTLP_ENDPOINT
- Trace tests assert actual spans arrived (not just "request succeeded")
- Metric tests run without external Grafana dependency
- All test files compile with `go build ./integration_testing/...`
</success_criteria>

<output>
After completion, create `.planning/quick/260401-cgf-add-self-contained-otel-collector-to-e2e/260401-cgf-SUMMARY.md`
</output>
