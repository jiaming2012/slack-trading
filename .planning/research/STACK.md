# Stack Research: Observability for Go + Python Trading Platform

**Domain:** Full-stack observability (OpenTelemetry + Grafana ecosystem)
**Researched:** 2026-03-25
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| OpenTelemetry Go SDK | v1.27.0 (existing) | Traces + metrics from Go server | Already imported in go.mod with spans in ~15 files. Upgrading to v1.42.0 is possible but unnecessary -- v1.27.0 is stable and the existing reference code (`quickstart.go`) targets it. Upgrade later when there is a reason. |
| OpenTelemetry Python SDK | 1.40.0 | Traces + metrics from Python strategy clients | Latest stable. Python 3.10 compatible. Pairs with structlog already in the conda env. |
| OTel Collector (contrib) | 0.148.0 | Telemetry routing hub: receives OTLP from Go + Python, exports to Loki/Tempo/Prometheus | The contrib distribution includes all needed exporters (otlphttp for Loki, otlp for Tempo). Acts as the single funnel so apps never talk directly to backends. |
| Grafana Loki | 3.6.0 | Log aggregation backend | Native OTLP ingestion (no deprecated lokiexporter needed). Lightweight, pairs naturally with Grafana. Structured metadata enabled by default in 3.x. |
| Grafana Tempo | 2.10.3 | Distributed trace backend | Native OTLP ingestion, purpose-built for traces. No indexing required (traces by ID). Pairs with Loki for trace-to-log correlation. |
| Prometheus | 2.54+ | Metrics backend | OTel Collector exports metrics via prometheusremotewrite. Grafana has first-class Prometheus data source. Needed because Loki is logs-only and Tempo is traces-only. |
| Grafana | 12.1 (OSS) | Dashboards, alerting, explore | Unified UI across all three backends. Built-in alerting replaces need for external tools. User has some experience with it. |

### Supporting Libraries

#### Go (already in go.mod -- wire up, don't install)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.opentelemetry.io/otel` | v1.27.0 | Core OTel API (tracer, meter) | Already imported. Initialize TracerProvider/MeterProvider in `cmd/main.go`. |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.27.0 | OTLP HTTP trace exporter | Send traces to OTel Collector. Already in go.mod. Reference in `quickstart.go`. |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` | v1.27.0 | OTLP HTTP metric exporter | Send metrics to OTel Collector. Already in go.mod. |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.52.0 | Auto-instrument HTTP handlers | Wrap Gorilla mux and Twirp handlers for request latency/count metrics. Already in go.mod. |
| `go.opentelemetry.io/contrib/instrumentation/runtime` | v0.52.0 | Go runtime metrics (GC, goroutines, memory) | Start in `cmd/main.go` alongside providers. Already in go.mod. |
| `github.com/uptrace/opentelemetry-go-extra/otellogrus` | v0.3.1 | Bridge logrus logs to OTel spans | Already imported and hooked in `cmd/main.go`. Logs get trace context automatically. |

#### Go (new -- install)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` | v0.3.0+ | OTLP HTTP log exporter | Send structured logs to OTel Collector as OTLP logs (not just trace-correlated). Needed for log pipeline completeness. |

#### Python (new -- install in conda env)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `opentelemetry-api` | 1.40.0 | OTel API for manual instrumentation | Trace strategy decisions, order placement, tick loop. |
| `opentelemetry-sdk` | 1.40.0 | OTel SDK (TracerProvider, MeterProvider) | Initialize in trading engine startup. |
| `opentelemetry-exporter-otlp-proto-http` | 1.40.0 | OTLP HTTP exporter | Send traces/metrics/logs to OTel Collector. Use HTTP (not gRPC) to avoid grpcio dependency complexity. |
| `opentelemetry-instrumentation-logging` | 0.61b0 | Bridge Python logging to OTel | Auto-inject trace_id/span_id into log records. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `grafana/otel-lgtm` Docker image | All-in-one dev backend (Collector + Loki + Tempo + Prometheus + Grafana) | Single container for local dev. Zero config. Grafana at :3000, OTLP at :4318. Use this for iteration speed. |
| Docker Compose (observability stack) | Production-like local setup | Separate containers for each component. Use when debugging inter-service config or preparing for deployment. |

## Architecture: How Components Connect

```
Go Server (:5051/:8080)          Python Client
  |  OTLP/HTTP traces+metrics+logs    |  OTLP/HTTP traces+metrics+logs
  |                                    |
  +-----------> OTel Collector (:4318) <-----------+
                    |
        +-----------+-----------+
        |           |           |
   Loki (:3100) Tempo (:3200) Prometheus (:9090)
   (logs)       (traces)      (metrics)
        |           |           |
        +-----------+-----------+
                    |
              Grafana (:3000)
              (dashboards + alerts)
```

## OTel Collector Configuration

Use the **contrib** distribution (`otel/opentelemetry-collector-contrib`), not core. Core lacks the `prometheusremotewrite` exporter.

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024
  resource:
    attributes:
      - key: service.namespace
        value: grodt
        action: upsert

exporters:
  otlphttp/logs:
    endpoint: "http://loki:3100/otlp"
    tls:
      insecure: true
  otlp/traces:
    endpoint: "tempo:4317"
    tls:
      insecure: true
  prometheusremotewrite:
    endpoint: "http://prometheus:9090/api/v1/write"
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [otlp/traces]
    metrics:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [prometheusremotewrite]
    logs:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [otlphttp/logs]
```

## Docker Compose Configuration

```yaml
# observability/docker-compose.yaml
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.148.0
    command: ["--config=/etc/otelcol/config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol/config.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    depends_on:
      - loki
      - tempo
      - prometheus

  loki:
    image: grafana/loki:3.6.0
    command: -config.file=/etc/loki/local-config.yaml
    volumes:
      - ./loki-config.yaml:/etc/loki/local-config.yaml
      - loki-data:/loki
    ports:
      - "3100:3100"

  tempo:
    image: grafana/tempo:2.10.3
    command: ["-config.file=/etc/tempo/config.yaml"]
    volumes:
      - ./tempo-config.yaml:/etc/tempo/config.yaml
      - tempo-data:/var/tempo
    ports:
      - "3200:3200"   # Tempo HTTP
      - "4327:4317"   # Tempo OTLP gRPC (remapped to avoid collision)

  prometheus:
    image: prom/prometheus:v2.54.0
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--web.enable-remote-write-receiver"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana-oss:12.1.0
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - grafana-data:/var/lib/grafana
    ports:
      - "3000:3000"
    depends_on:
      - loki
      - tempo
      - prometheus

volumes:
  loki-data:
  tempo-data:
  prometheus-data:
  grafana-data:
```

## Installation

### Go (no new packages needed for MVP)

The existing go.mod already has all required OTel packages. The only work is initializing providers in `cmd/main.go` using the pattern from `deprecated/go/cmd/telemetry/quickstart.go`.

If OTLP log export is needed later:
```bash
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
```

### Python (conda env)

```bash
conda activate grodt
pip install \
  opentelemetry-api==1.40.0 \
  opentelemetry-sdk==1.40.0 \
  opentelemetry-exporter-otlp-proto-http==1.40.0 \
  opentelemetry-instrumentation-logging==0.61b0
```

### Quick Start (dev mode)

```bash
# Fastest way to get a backend running for development:
docker run --name otel-lgtm -p 3000:3000 -p 4317:4317 -p 4318:4318 -d grafana/otel-lgtm:latest
```

Then configure Go/Python to export OTLP to `http://localhost:4318`.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| OTel Collector (contrib) | Grafana Alloy | If you want a single collector for logs+metrics+traces with Grafana-native config syntax. Alloy is Grafana's OTel Collector distribution. Consider for production if you want tighter Grafana ecosystem integration, but OTel Collector is more vendor-neutral and has broader community docs. |
| Loki (native OTLP) | ELK Stack | Never for this project. ELK is heavyweight, requires Elasticsearch cluster, and the team has no ELK experience. |
| Loki (native OTLP) | OTel Collector lokiexporter | Never. The lokiexporter is deprecated. Loki 3.x accepts OTLP natively via `otlphttp` exporter. |
| Tempo | Jaeger | If you need trace search by arbitrary fields (Tempo only searches by trace ID natively). For this project, Tempo is better because it pairs with Grafana and requires no indexing infrastructure. |
| Prometheus | Mimir | If you need long-term metric storage or multi-tenant metrics. Overkill for a single trading platform. |
| `grafana/otel-lgtm` (dev) | Full Docker Compose | When debugging collector config or testing production-like setup. Use otel-lgtm for daily dev work. |
| OTLP/HTTP exporters | OTLP/gRPC exporters | If you have gRPC infrastructure. HTTP is simpler (no grpcio dependency in Python, works through proxies). For this project, HTTP is the right choice -- matches existing Twirp HTTP transport. |
| structlog (Python logs) | loguru | loguru is already in requirements but structlog has better OTel integration and is already available in conda env. Use structlog for new instrumented code. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Promtail | Deprecated as of Feb 2025, EOL Feb 2026. Grafana replaced it with Alloy. | OTel Collector (for this project) or Grafana Alloy |
| lokiexporter (OTel Collector) | Deprecated in collector-contrib. Loki 3.x has native OTLP endpoint. | `otlphttp` exporter pointing at `http://loki:3100/otlp` |
| Datadog / New Relic / Honeycomb | Vendor lock-in, ongoing cost. OSS stack is free and the project already chose this direction. | Grafana + Loki + Tempo + Prometheus |
| opentelemetry-exporter-otlp (gRPC variant) for Python | Pulls in `grpcio` which is heavy, has build issues on some platforms, and conflicts with numpy pinning. | `opentelemetry-exporter-otlp-proto-http` (HTTP variant) |
| OpenCensus (`go.opencensus.io`) | Already in go.mod as indirect dependency but OpenCensus is archived/deprecated. Merged into OpenTelemetry. | OpenTelemetry (already primary) |
| Grafana Agent (static/flow) | Superseded by Grafana Alloy. No longer maintained. | OTel Collector or Grafana Alloy |

## Stack Patterns by Variant

**For local development (daily work):**
- Use `grafana/otel-lgtm` all-in-one Docker image
- Zero config, single container, all backends included
- Because iteration speed matters more than production fidelity

**For production (Digital Ocean):**
- Use separate containers: OTel Collector + Loki + Tempo + Prometheus + Grafana
- Each component independently scalable and configurable
- Because production needs persistence, resource limits, and independent restarts

**For testing OTel instrumentation without any backend:**
- Set `OTEL_TRACES_EXPORTER=console` and `OTEL_METRICS_EXPORTER=console`
- OTel SDK prints telemetry to stdout
- Because you can verify instrumentation works before setting up infrastructure

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| OTel Go SDK v1.27.0 | Go 1.22.4 | Current project Go version is supported. v1.42.0 drops Go 1.24+ support eventually but v1.27.0 is fine. |
| OTel Python SDK 1.40.0 | Python 3.9+ | Python 3.10 (conda env) is supported. |
| OTel Python SDK 1.40.0 | numpy 1.26.4 | No conflict -- OTel Python has no numpy dependency. |
| OTel Collector contrib 0.148.0 | Loki 3.6.0 | Uses native OTLP endpoint. Structured metadata enabled by default. |
| OTel Collector contrib 0.148.0 | Tempo 2.10.3 | Native OTLP gRPC ingestion. |
| Grafana 12.1 | Loki 3.6.0, Tempo 2.10.3, Prometheus 2.54 | All data sources natively supported. |
| Loki 3.6.0 | Docker (no shell) | Since Loki 3.5.8, busybox removed from Docker image. Cannot exec into container with /bin/sh. |

## Existing Codebase Assets (Don't Rebuild)

These already exist and should be leveraged, not replaced:

| Asset | Location | Status |
|-------|----------|--------|
| OTel Go packages | `go.mod` (9 OTel imports) | Imported but TracerProvider/MeterProvider never initialized |
| Tracer spans | ~15 files in eventservices, eventconsumers, eventproducers | Created but go nowhere (no provider) |
| otellogrus hook | `cmd/main.go` | Hooked but ineffective without provider |
| Reference OTel setup | `deprecated/go/cmd/telemetry/quickstart.go` | Complete working example of TracerProvider + MeterProvider + otellogrus + otelhttp |
| structlog | conda env (`grodt.yml`) | Available for Python structured logging |
| logrus (JSON) | Go server | Already the primary Go logger, OTel bridge exists |
| pprof endpoints | `cmd/main.go` at `/debug/pprof/*` | Runtime profiling (keep alongside OTel) |

## Environment Variables

The OTel SDK respects standard environment variables. Configure these instead of hardcoding endpoints:

| Variable | Value (local dev) | Purpose |
|----------|-------------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | Where Go/Python send OTLP data |
| `OTEL_SERVICE_NAME` | `grodt-server` / `grodt-strategy` | Identifies service in traces/metrics |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment=development` | Tags all telemetry with environment |
| `OTEL_TRACES_EXPORTER` | `otlp` (default) | Can set to `console` for debugging |
| `OTEL_METRICS_EXPORTER` | `otlp` (default) | Can set to `console` for debugging |
| `OTEL_LOGS_EXPORTER` | `otlp` | Enable OTLP log export |

## Sources

- [OpenTelemetry Go releases](https://github.com/open-telemetry/opentelemetry-go/releases) -- v1.42.0 latest, v1.27.0 in project (HIGH confidence)
- [OpenTelemetry Python SDK on PyPI](https://pypi.org/project/opentelemetry-sdk/) -- v1.40.0 latest (HIGH confidence)
- [OTel Collector releases](https://github.com/open-telemetry/opentelemetry-collector-releases/releases) -- v0.148.0 latest (HIGH confidence)
- [Grafana Loki releases](https://github.com/grafana/loki/releases) -- v3.6.0 (HIGH confidence)
- [Grafana Tempo releases](https://github.com/grafana/tempo/releases) -- v2.10.3 (HIGH confidence)
- [Grafana releases](https://hub.docker.com/r/grafana/grafana) -- 12.1 latest (MEDIUM confidence, Docker Hub tags)
- [Loki OTLP ingestion docs](https://grafana.com/docs/loki/latest/send-data/otel/) -- native OTLP, no lokiexporter needed (HIGH confidence)
- [OTel Collector + Loki tutorial](https://grafana.com/docs/loki/latest/send-data/otel/otel-collector-getting-started/) -- collector config pattern (HIGH confidence)
- [grafana/docker-otel-lgtm](https://github.com/grafana/docker-otel-lgtm) -- all-in-one dev image (HIGH confidence)
- [Promtail deprecation](https://community.grafana.com/t/how-does-alloy-relate-to-otel-collector/134548) -- deprecated Feb 2025, EOL Feb 2026 (HIGH confidence)
- [structlog OTel integration](https://johal.in/structlog-json-logs-middleware-opentelemetry-python-2026/) -- trace context in structlog (MEDIUM confidence, blog post)
- Existing codebase: `go.mod`, `deprecated/go/cmd/telemetry/quickstart.go` -- verified by direct inspection (HIGH confidence)

---
*Stack research for: Observability (OpenTelemetry + Grafana ecosystem) for Go + Python trading platform*
*Researched: 2026-03-25*
