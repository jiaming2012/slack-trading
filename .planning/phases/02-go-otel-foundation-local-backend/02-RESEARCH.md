# Phase 2: Go OTel Foundation & Local Backend - Research

**Researched:** 2026-03-26
**Domain:** OpenTelemetry Go SDK, Grafana LGTM observability stack, structured logging
**Confidence:** HIGH

## Summary

This phase initializes the OpenTelemetry TracerProvider and MeterProvider in the Go server entrypoint (`cmd/main.go`), activating ~24 existing tracer spans across 10 source files that currently produce no-ops. It also stands up a local observability backend using the `grafana/otel-lgtm` all-in-one Docker image, and configures logrus to output logfmt format.

The project already has all required OTel Go SDK packages at v1.27.0 in `go.mod`, a working reference implementation in `deprecated/go/cmd/telemetry/quickstart.go`, and the otellogrus hook wired in `cmd/main.go`. The primary work is: (1) extract and adapt the `setupOTelSDK` function into the server's startup path, (2) create the Docker Compose file for the observability stack, (3) configure logrus TextFormatter for logfmt, and (4) add OTEL_* environment variables.

**Primary recommendation:** Adapt `quickstart.go`'s `setupOTelSDK()` into a new `src/go/utils/otel.go` file, wire it early in `cmd/main.go` with deferred shutdown, create `observability/docker-compose.yaml` with `grafana/otel-lgtm`, and add logrus TextFormatter configuration.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Separate compose file at `observability/docker-compose.yaml` -- independent from `eventstoredb/docker-compose.yaml`. Keeps database infra and monitoring infra as separate concerns.
- **D-02:** Use `grafana/otel-lgtm` all-in-one image for local dev. Single container, zero config, fast to iterate. Production deployment (Phase 7) will use individual containers on Digital Ocean.
- **D-03:** EventStoreDB is NOT used for observability -- it stays for event sourcing only. Loki (logs), Tempo (traces), Prometheus (metrics) handle observability.
- **D-04:** Log output format is **logfmt** (`key=value` pairs). Human-readable in terminal and machine-parseable by Loki.
- **D-05:** Field naming uses **snake_case**: `playground_id`, `order_id`, `symbol`, `environment`, `duration_ms`.
- **D-06:** Existing logrus stays as the logging library. Configure logrus formatter to output logfmt.
- **D-07:** OTel configured via standard **OTEL_* environment variables** (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME, etc.). No hardcoded values in code beyond the `utils.GetEnv()` call to read them.
- **D-08:** **Always-on sampling** (100%) in dev. Every trace is captured. Production sampling strategy deferred to Phase 7.
- **D-09:** Reference implementation in `deprecated/go/cmd/telemetry/quickstart.go` is the starting point for TracerProvider/MeterProvider initialization -- adapt, don't copy blindly.

### Claude's Discretion
- How to structure the OTel initialization code (single file vs split)
- Logrus formatter configuration details
- OTel Collector pipeline config within otel-lgtm
- Whether to add otelhttp middleware in this phase or defer to Phase 3
- Grafana data source provisioning approach

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OTEL-01 | Go server initializes TracerProvider with OTLP HTTP exporter on startup | Adapt `setupOTelSDK()` from quickstart.go; SDK packages already in go.mod at v1.27.0 |
| OTEL-02 | Go server initializes MeterProvider with OTLP HTTP exporter on startup | Same function; `otlpmetrichttp` already imported in go.mod |
| OTEL-03 | Graceful shutdown calls ForceFlush on both providers before exit | Use shutdown closure pattern from quickstart.go; wire into existing signal handler in main.go |
| OTEL-04 | All existing tracer spans (~15 files) produce real traces after provider init | 24 tracer call sites across 10 files use `otel.Tracer()` / `otel.GetTracerProvider().Tracer()` -- all activate once global provider is set |
| OTEL-05 | Structured log fields follow consistent conventions | Configure logrus TextFormatter with logfmt output; document snake_case field naming conventions |
| OTEL-06 | OTel environment variables configurable per environment | Add OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME to .env; SDK reads them automatically via OTLP HTTP defaults |
| BACK-01 | Docker Compose starts OTel Collector, Loki, Grafana, Tempo, and Prometheus with single command | `grafana/otel-lgtm` bundles all five; single service in docker-compose.yaml |
| BACK-02 | OTel Collector config routes traces to Tempo, logs to Loki, metrics to Prometheus | Pre-configured inside otel-lgtm image; no custom collector config needed |
| BACK-03 | Grafana starts with Loki, Tempo, and Prometheus pre-configured as data sources | Auto-provisioned inside otel-lgtm image; no manual datasource config needed |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go.opentelemetry.io/otel | v1.27.0 | OTel API (tracers, meters) | Already in go.mod; project standard |
| go.opentelemetry.io/otel/sdk | v1.27.0 | TracerProvider, sampler config | Already in go.mod |
| go.opentelemetry.io/otel/sdk/metric | v1.27.0 | MeterProvider, periodic reader | Already in go.mod |
| go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp | v1.27.0 | OTLP HTTP trace exporter | Already in go.mod; matches D-07 env var config |
| go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp | v1.27.0 | OTLP HTTP metric exporter | Already in go.mod |
| go.opentelemetry.io/otel/propagation | (bundled) | W3C TraceContext propagation | Already used in quickstart.go |
| github.com/sirupsen/logrus | v1.9.3 | Structured logging | Already project-wide standard (D-06) |
| github.com/uptrace/opentelemetry-go-extra/otellogrus | v0.3.1 | Bridge logrus to OTel logs | Already in go.mod and wired in main.go |
| grafana/otel-lgtm | latest | All-in-one observability backend | D-02 locked decision |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go.opentelemetry.io/otel/sdk/resource | (bundled) | Service name/version resource attrs | Used in TracerProvider setup |
| go.opentelemetry.io/contrib/instrumentation/runtime | v0.52.0 | Go runtime metrics (goroutines, GC, memory) | Already in go.mod; enables runtime.Start() |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| OTLP HTTP exporter | OTLP gRPC exporter | HTTP is simpler, no proto dependency at runtime; gRPC marginally faster but unnecessary for single-service dev |
| grafana/otel-lgtm | Separate Grafana + Tempo + Loki + Prometheus containers | More production-like but much more config; deferred to Phase 7 per D-02 |
| logrus TextFormatter | slog (Go 1.21+) | slog is newer standard but project is deeply invested in logrus (D-06 locks this) |

**Installation:**
```bash
# No new Go packages needed -- all already in go.mod
# Docker image pull:
docker pull grafana/otel-lgtm
```

## Architecture Patterns

### Recommended Project Structure
```
src/go/utils/otel.go            # New: SetupOTelSDK() function
cmd/main.go                     # Modified: call SetupOTelSDK(), configure logrus formatter
observability/
  docker-compose.yaml           # New: grafana/otel-lgtm service
.env                            # Modified: add OTEL_* variables
taskfile.yml                    # Modified: add observability:start/stop tasks
```

### Pattern 1: OTel SDK Initialization (Single File)
**What:** All OTel setup (TracerProvider, MeterProvider, propagator, resource) in one function returning a shutdown closure.
**When to use:** Service startup, before any traced code runs.
**Recommendation:** Single file `src/go/utils/otel.go` -- keeps OTel initialization colocated, matches project convention of utils as shared infrastructure.
**Example:**
```go
// Source: adapted from deprecated/go/cmd/telemetry/quickstart.go
package utils

import (
    "context"
    "errors"
    "fmt"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// SetupOTelSDK initializes TracerProvider and MeterProvider with OTLP HTTP exporters.
// Returns a shutdown function that must be called before process exit.
// Configuration is read from OTEL_* environment variables (standard OTel env var convention).
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string) (func(context.Context) error, error) {
    var shutdownFuncs []func(context.Context) error

    shutdown := func(ctx context.Context) error {
        var err error
        for _, fn := range shutdownFuncs {
            err = errors.Join(err, fn(ctx))
        }
        return err
    }

    // Set W3C TraceContext + Baggage propagator
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    // Build resource with service identity
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion(serviceVersion),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("creating resource: %w", err)
    }

    // TracerProvider
    traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
    if err != nil {
        return nil, fmt.Errorf("creating trace exporter: %w", err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(res),
    )
    shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
    otel.SetTracerProvider(tp)

    // MeterProvider
    metricExporter, err := otlpmetrichttp.New(ctx)
    if err != nil {
        return nil, fmt.Errorf("creating metric exporter: %w", err)
    }

    mp := metric.NewMeterProvider(
        metric.WithReader(metric.NewPeriodicReader(metricExporter)),
        metric.WithResource(res),
    )
    shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
    otel.SetMeterProvider(mp)

    return shutdown, nil
}
```

### Pattern 2: Wiring into main.go
**What:** Call SetupOTelSDK early in main(), defer shutdown, handle the 61 log.Fatal calls that bypass defer.
**When to use:** Server entrypoint.
**Example:**
```go
// In cmd/main.go, after InitEnvironmentVariables and before any traced code:

// Configure logrus for logfmt output
log.SetFormatter(&log.TextFormatter{
    DisableColors:    true,
    FullTimestamp:     true,
    TimestampFormat:  time.RFC3339,
})

// Initialize OTel SDK
otelShutdown, err := utils.SetupOTelSDK(ctx, "grodt", "1.0.0")
if err != nil {
    log.Fatalf("failed to initialize OTel SDK: %v", err)
}
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := otelShutdown(shutdownCtx); err != nil {
        log.Errorf("OTel shutdown error: %v", err)
    }
}()
```

### Pattern 3: Docker Compose for Observability
**What:** Separate compose file with grafana/otel-lgtm.
**When to use:** Local development observability.
**Example:**
```yaml
# observability/docker-compose.yaml
version: "3.4"

services:
  otel-lgtm:
    image: grafana/otel-lgtm
    ports:
      - "3000:3000"    # Grafana UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    volumes:
      - otel-lgtm-data:/data

volumes:
  otel-lgtm-data:
```

### Pattern 4: Environment Variable Configuration
**What:** OTEL_* env vars control SDK behavior without code changes.
**When to use:** All environments.
**Key variables:**
```bash
# .env additions
OTEL_SERVICE_NAME=grodt
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```
**Important:** The OTLP HTTP exporter reads `OTEL_EXPORTER_OTLP_ENDPOINT` automatically -- no code needed to parse it. The SDK's `otlptracehttp.NewClient()` and `otlpmetrichttp.New()` both honor standard OTEL_* env vars.

### Anti-Patterns to Avoid
- **Hardcoding OTLP endpoints in Go code:** The SDK reads OTEL_* env vars automatically. Do NOT pass endpoint options to `otlptracehttp.NewClient()` -- let env vars control it (D-07).
- **Using `otel.Tracer()` before provider is set:** All 24 existing call sites use `otel.Tracer()` which returns a no-op until `otel.SetTracerProvider()` is called. Ensure SetupOTelSDK runs before any consumer/producer starts.
- **Calling runtime.Start() inside SetupOTelSDK:** The quickstart.go reference calls `runtime.Start()` for Go runtime metrics, but this is a BEAT-01 concern (Phase 3). Defer it to keep this phase focused.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OTLP export | Custom HTTP POST to collector | otlptracehttp / otlpmetrichttp | Handles retries, batching, compression, content negotiation |
| Observability backend | Individual Grafana + Tempo + Loki + Prometheus Docker setup | grafana/otel-lgtm | Pre-configured routing, data source provisioning, zero-config |
| Logfmt formatting | Custom logrus Formatter | logrus.TextFormatter with DisableColors:true | Built-in, produces compliant logfmt in non-TTY and when colors disabled |
| Trace context propagation | Manual header parsing | propagation.TraceContext{} | W3C standard, handles traceparent/tracestate correctly |
| Resource attributes | Manual attribute map | resource.New() with semconv | Follows OTel semantic conventions, interoperable |

**Key insight:** The OTel Go SDK is designed so that configuration happens through environment variables and the SDK handles the rest. Almost no custom plumbing is needed beyond calling the initialization functions.

## Common Pitfalls

### Pitfall 1: log.Fatal Bypasses Defer (OTel Shutdown)
**What goes wrong:** 61 `log.Fatal` / `log.Panic` calls across 35 files call `os.Exit(1)` immediately, bypassing `defer otelShutdown()`. Traces in flight are lost.
**Why it happens:** logrus Fatal calls os.Exit under the hood.
**How to avoid:** For this phase, accept the limitation. The otellogrus hook (already wired) will attempt to flush on Fatal/Panic levels. The graceful shutdown via signal handler (SIGTERM/SIGINT) works correctly with defer. A future phase could replace Fatal calls with error returns.
**Warning signs:** Missing trace data for crash scenarios.

### Pitfall 2: OTEL_EXPORTER_OTLP_ENDPOINT Must Not Have Trailing Path
**What goes wrong:** Setting endpoint to `http://localhost:4318/v1/traces` causes double-pathing. The SDK appends `/v1/traces` automatically for HTTP.
**Why it happens:** OTLP HTTP exporter convention appends signal-specific paths.
**How to avoid:** Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` (base URL only).
**Warning signs:** 404 errors in exporter logs.

### Pitfall 3: Port Conflicts with Existing Services
**What goes wrong:** Port 3000 (Grafana) or 4317/4318 (collector) may conflict with other local services.
**Why it happens:** Common ports used by other dev tools.
**How to avoid:** Document port requirements. Use `docker compose` port mapping to remap if needed. Port 8080 already has a known conflict (CLAUDE.md gotcha).
**Warning signs:** Container starts but services unreachable.

### Pitfall 4: Logrus TextFormatter Behavior Differs in TTY vs Non-TTY
**What goes wrong:** Without `DisableColors: true`, TextFormatter auto-detects TTY and outputs colored, non-logfmt output in terminals.
**Why it happens:** logrus defaults to human-friendly colored output when TTY is detected.
**How to avoid:** Always set `DisableColors: true` to get consistent logfmt output regardless of environment.
**Warning signs:** Loki can't parse log lines; fields appear garbled in Grafana.

### Pitfall 5: godotenv Loads .env Before OTel SDK Reads Env Vars
**What goes wrong:** If OTEL_* vars are in `.env` but `godotenv.Load()` isn't called before OTel SDK init, the SDK won't see them.
**Why it happens:** The OTel SDK reads env vars at client creation time.
**How to avoid:** In `cmd/main.go`, `utils.InitEnvironmentVariables()` (which calls godotenv) already runs early. Place `SetupOTelSDK()` AFTER this call.
**Warning signs:** Traces going to wrong endpoint or not appearing.

### Pitfall 6: Service Name Not Set
**What goes wrong:** Without `OTEL_SERVICE_NAME`, traces appear as "unknown_service" in Tempo/Grafana, making them hard to find.
**Why it happens:** SDK defaults to "unknown_service:binary_name" if env var not set and no resource attribute provided.
**How to avoid:** Set both `OTEL_SERVICE_NAME=grodt` in `.env` AND pass service name via resource attributes as fallback.
**Warning signs:** Traces visible but labeled "unknown_service".

## Code Examples

### Existing Span Pattern (already in codebase)
```go
// Source: src/go/eventservices/trade.go
tracer := otel.GetTracerProvider().Tracer("getTradeComponents")
ctx, span := tracer.Start(ctx, "getTradeComponents")
defer span.End()
// ... span.SetAttributes(attribute.String("key", "value"))
```

### Existing Span Pattern - Alternate Form
```go
// Source: src/go/eventconsumers/tracker_consumer_v3.go
ctx, span := otel.Tracer("tracker_v3_consumer").Start(ctx, "checkSupertrendH1StochRsiDown")
defer span.End()
```

### Logrus TextFormatter for Logfmt
```go
// Source: logrus documentation
log.SetFormatter(&log.TextFormatter{
    DisableColors:    true,
    FullTimestamp:     true,
    TimestampFormat:  time.RFC3339,
    FieldMap: log.FieldMap{
        log.FieldKeyTime:  "ts",
        log.FieldKeyLevel: "level",
        log.FieldKeyMsg:   "msg",
    },
})
```

### otellogrus Hook (already wired in main.go)
```go
// Source: cmd/main.go lines 260-266 -- already done, keep as-is
log.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
    log.PanicLevel,
    log.FatalLevel,
    log.ErrorLevel,
    log.WarnLevel,
    log.InfoLevel,
)))
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Jaeger all-in-one | grafana/otel-lgtm | 2024 | Unified logs+traces+metrics in one image |
| Manual OTel Collector config | otel-lgtm built-in routing | 2024 | No collector YAML needed for dev |
| logrus JSONFormatter | logrus TextFormatter (logfmt) | D-04 decision | Logfmt is human-readable + Loki-parseable |
| Hardcoded OTLP endpoints | OTEL_* env var convention | OTel standard | Zero-code endpoint configuration |

**Deprecated/outdated:**
- `deprecated/go/cmd/telemetry/telemetry.go`: Uses Jaeger-specific OTLP setup with auth headers -- not needed for local dev
- `deprecated/go/cmd/telemetry/quickstart.go`: Uses JSONFormatter -- switch to TextFormatter per D-04

## Open Questions

1. **Whether to add otelhttp middleware in this phase**
   - What we know: `otelhttp` package is already in go.mod (v0.52.0). quickstart.go demonstrates usage. TRACE-01 is v2 requirement.
   - What's unclear: Whether wrapping the gorilla mux router adds value before Phase 5 (tick tracing).
   - Recommendation: Defer to Phase 3 or later. This phase is about foundation -- getting spans visible. HTTP middleware adds request-level spans but is not required by any Phase 2 requirement.

2. **Whether to call `runtime.Start()` for Go runtime metrics**
   - What we know: Already in go.mod (`go.opentelemetry.io/contrib/instrumentation/runtime` v0.52.0). quickstart.go calls it.
   - What's unclear: Whether BEAT-01 (Phase 3) subsumes this or whether it's a separate concern.
   - Recommendation: Include it in SetupOTelSDK since it's trivial (one line) and provides immediate value in Grafana (goroutine count, GC stats, memory). Does not conflict with BEAT-01 which adds custom heartbeat metrics.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | BACK-01, BACK-02, BACK-03 | Yes | 20.10.14 | -- |
| Docker Compose | BACK-01 | Yes | v2.5.1 | -- |
| Go | OTEL-01 through OTEL-06 | Yes | 1.25.5 | -- |
| OTel Go SDK packages | OTEL-01, OTEL-02 | Yes (go.mod) | v1.27.0 | -- |
| logrus | OTEL-05 | Yes (go.mod) | v1.9.3 | -- |
| otellogrus | OTEL-05 | Yes (go.mod) | v0.3.1 | -- |
| grafana/otel-lgtm | BACK-01 | Pull required | latest | -- |

**Missing dependencies with no fallback:** None

**Missing dependencies with fallback:** None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.9.0 |
| Config file | None (Go convention) |
| Quick run command | `go test -count=1 ./src/go/utils/...` |
| Full suite command | `cd src/go/backtester-api && go test -count=1 ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OTEL-01 | TracerProvider initializes with OTLP HTTP exporter | unit | `go test -run TestSetupOTelSDK -count=1 ./src/go/utils/...` | No -- Wave 0 |
| OTEL-02 | MeterProvider initializes with OTLP HTTP exporter | unit | Same as OTEL-01 (combined setup) | No -- Wave 0 |
| OTEL-03 | Graceful shutdown flushes providers | unit | `go test -run TestOTelShutdown -count=1 ./src/go/utils/...` | No -- Wave 0 |
| OTEL-04 | Existing spans produce real traces | smoke | Start server + observability stack, check Grafana Tempo | Manual |
| OTEL-05 | Logfmt output format | unit | `go test -run TestLogfmtFormat -count=1 ./src/go/utils/...` | No -- Wave 0 |
| OTEL-06 | OTEL_* env vars configurable | unit | Verify env vars read by SDK (implicit, tested by OTEL-01) | N/A |
| BACK-01 | Docker Compose starts full stack | smoke | `docker compose -f observability/docker-compose.yaml up -d && curl -s http://localhost:3000/api/health` | No -- manual |
| BACK-02 | Collector routes signals correctly | smoke | Send trace, verify in Tempo via Grafana API | Manual |
| BACK-03 | Grafana has pre-configured data sources | smoke | `curl -s http://localhost:3000/api/datasources` | No -- manual |

### Sampling Rate
- **Per task commit:** `cd src/go/backtester-api && go test -count=1 ./...`
- **Per wave merge:** Full unit test suite + manual smoke test of observability stack
- **Phase gate:** Unit tests pass + traces visible in local Grafana Tempo

### Wave 0 Gaps
- [ ] `src/go/utils/otel_test.go` -- covers OTEL-01, OTEL-02, OTEL-03 (test that SetupOTelSDK returns valid shutdown, providers are set globally)
- [ ] Smoke test script or documentation for verifying BACK-01, BACK-02, BACK-03

## Project Constraints (from CLAUDE.md)

- **Go module path:** `github.com/jiaming2012/slack-trading`
- **Import convention:** `github.com/jiaming2012/slack-trading/src/go/<package>`
- **logrus imported as `log`:** Do not change this convention
- **Environment variables via `utils.GetEnv()`:** Use same pattern for any new env vars
- **godotenv loads .env:** `utils.InitEnvironmentVariables()` handles this
- **Port 8080 conflict:** Known gotcha -- does not affect OTel ports (3000, 4317, 4318)
- **`gh` alias:** User's shell aliases `gh` to `git checkout`. Use `/usr/local/bin/gh` or `command gh`.
- **Existing Docker Compose at `eventstoredb/docker-compose.yaml`:** Do NOT modify (D-01 locks separate file)

## Sources

### Primary (HIGH confidence)
- `deprecated/go/cmd/telemetry/quickstart.go` -- Complete reference implementation, verified in codebase
- `cmd/main.go` -- Current server entrypoint, verified line-by-line
- `go.mod` -- All OTel packages at v1.27.0 confirmed
- Existing span usage: 24 call sites across 10 files confirmed via grep

### Secondary (MEDIUM confidence)
- [grafana/docker-otel-lgtm GitHub](https://github.com/grafana/docker-otel-lgtm) -- Ports, env vars, data source auto-provisioning
- [Docker Hub grafana/otel-lgtm](https://hub.docker.com/r/grafana/otel-lgtm) -- Image availability, basic usage
- [logrus GitHub](https://github.com/sirupsen/logrus) -- TextFormatter configuration for logfmt

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all packages already in go.mod, versions confirmed
- Architecture: HIGH -- reference implementation exists in codebase, pattern well-understood
- Pitfalls: HIGH -- identified from codebase analysis (Fatal calls, port conflicts) and OTel SDK documentation

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable -- OTel Go SDK v1.27.0 is established, grafana/otel-lgtm is stable)
