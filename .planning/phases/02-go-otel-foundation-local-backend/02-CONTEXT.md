# Phase 2: Go OTel Foundation & Local Backend - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Initialize TracerProvider and MeterProvider in the Go server (activating ~15 files of existing spans), set up structured logging conventions, configure OTel via standard environment variables, and stand up a local observability backend using the grafana/otel-lgtm Docker image.

</domain>

<decisions>
## Implementation Decisions

### Docker Compose Setup
- **D-01:** Separate compose file at `observability/docker-compose.yaml` — independent from `eventstoredb/docker-compose.yaml`. Keeps database infra and monitoring infra as separate concerns.
- **D-02:** Use `grafana/otel-lgtm` all-in-one image for local dev. Single container, zero config, fast to iterate. Production deployment (Phase 7) will use individual containers on Digital Ocean.
- **D-03:** EventStoreDB is NOT used for observability — it stays for event sourcing only. Loki (logs), Tempo (traces), Prometheus (metrics) handle observability.

### Structured Logging Conventions
- **D-04:** Log output format is **logfmt** (`key=value` pairs). Human-readable in terminal and machine-parseable by Loki.
- **D-05:** Field naming uses **snake_case**: `playground_id`, `order_id`, `symbol`, `environment`, `duration_ms`.
- **D-06:** Existing logrus stays as the logging library. Configure logrus formatter to output logfmt.

### OTel Configuration
- **D-07:** OTel configured via standard **OTEL_* environment variables** (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME, etc.). No hardcoded values in code beyond the `utils.GetEnv()` call to read them.
- **D-08:** **Always-on sampling** (100%) in dev. Every trace is captured. Production sampling strategy deferred to Phase 7.
- **D-09:** Reference implementation in `deprecated/go/cmd/telemetry/quickstart.go` is the starting point for TracerProvider/MeterProvider initialization — adapt, don't copy blindly.

### Trace ID Propagation via Proto
- **D-10:** Every tick must have a `trace_id`. Add `string trace_id` field to `NextTickRequest` in `playground.proto`.
- **D-11:** The `trace_id` must appear on any signals or errors emitted during that tick — include it in structured log fields and span attributes.
- **D-12:** The `trace_id` must appear on any order. Add `string trace_id` field to `PlaceOrderRequest` in `playground.proto`.
- **D-13:** Regenerate protobuf stubs (Go + Python) after proto changes via `task gen:proto`.
- **D-14:** Go server handlers extract `trace_id` from requests and attach to span context / log fields. Python client sets `trace_id` from the active OTel span (or generates one if no span active).

### Claude's Discretion
- How to structure the OTel initialization code (single file vs split)
- Logrus formatter configuration details
- OTel Collector pipeline config within otel-lgtm
- Whether to add otelhttp middleware in this phase or defer to Phase 3
- Grafana data source provisioning approach
- Whether trace_id in proto should be W3C traceparent format or just the 32-hex-char trace ID

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Files
- `src/go/playground.proto` — Protobuf service definition (needs trace_id fields on NextTickRequest, PlaceOrderRequest)
- `cmd/main.go` — Server entrypoint, where TracerProvider/MeterProvider init goes (currently only has otellogrus hook)
- `deprecated/go/cmd/telemetry/quickstart.go` — Complete reference implementation (249 lines) with OTLP exporters, trace/metrics SDK, resource attributes, runtime instrumentation
- `deprecated/go/cmd/telemetry/telemetry.go` — Jaeger-specific OTLP setup (72 lines)
- `go.mod` — OTel packages already imported at v1.27.0
- `.env` — Environment variables (no OTEL_* vars currently)

### Existing OTel Usage (spans that will activate)
- `src/go/eventconsumers/tracker_consumer_v3.go` — `otel.Tracer("tracker_v3_consumer")`
- `src/go/eventconsumers/process_signals.go` — Tracer spans
- `src/go/eventconsumers/trade_ev.go` — Tracer usage
- `src/go/eventproducers/esdb_producer.go` — Trace context propagation
- `src/go/eventproducers/signalapi/process_signal_executor.go` — Tracer spans
- `src/go/eventmodels/read_option_chain_request_executor.go` — Tracer spans
- `src/go/eventmodels/new_signal_request_event_v1.go` — Tracer usage
- `src/go/eventservices/trade.go` — Multiple tracer spans with attributes
- `src/go/eventservices/exec_statistics.go` — Tracer spans with attributes
- `src/go/eventservices/fetch_option_chain_with_params.go` — Tracer spans
- `src/go/eventservices/polygon.go` — Tracer spans
- `src/go/utils/telemetry.go` — Trace context serialization/deserialization utilities
- `src/go/eventmodels/trace_context.go` — TraceContext DTO

### Infrastructure
- `eventstoredb/docker-compose.yaml` — Existing Docker Compose (EventStoreDB + Postgres) — do NOT modify
- `taskfile.yml` — Task runner (may need new tasks for observability stack)
- `cmd/run-dev.sh` — Dev runner script

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `quickstart.go` has complete TracerProvider + MeterProvider + resource setup — adapt this
- otellogrus hook already registered in `cmd/main.go` — keep it, it bridges logrus → OTel logs
- `utils.GetEnv()` pattern for environment variables — use same pattern for OTEL_* vars

### Patterns to Follow
- Server startup in `cmd/main.go` loads .env via godotenv, then initializes services
- `defer` pattern for cleanup (use for TracerProvider/MeterProvider shutdown)
- logrus is imported as `log` everywhere — don't change this convention

### Known Gotchas
- 61 `log.Fatal`/`panic` calls across 35 files will kill the OTel pipeline before flush — ForceFlush hook addresses this for graceful shutdown, but Fatal calls bypass defer
- Port 8080 may conflict — REST server logs error but doesn't crash
- Existing spans use `otel.Tracer("component-name")` but no global provider is set — they currently produce no-ops

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-go-otel-foundation-local-backend*
*Context gathered: 2026-03-26*
