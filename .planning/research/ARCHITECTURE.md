# Architecture Patterns: Observability Layer

**Domain:** Observability for event-driven Go + Python trading platform
**Researched:** 2026-03-25

## Recommended Architecture

### Telemetry Pipeline

```
+-------------------+       +-------------------+
| Go Server         |       | Python Client     |
| (grodt-server)    |       | (grodt-strategy)  |
|                   |       |                   |
| OTel SDK v1.27.0  |       | OTel SDK 1.40.0   |
| - TracerProvider  |       | - TracerProvider  |
| - MeterProvider   |       | - MeterProvider   |
| - LogsProvider*   |       | - LogHandler      |
| - otellogrus hook |       | - structlog proc  |
| - otelhttp mw    |       |                   |
| - runtime metrics |       |                   |
+--------+----------+       +--------+----------+
         |  OTLP/HTTP (:4318)        |
         +------------+--------------+
                      |
              +-------v-------+
              | OTel Collector|
              | (contrib)     |
              | :4317 gRPC    |
              | :4318 HTTP    |
              +---+---+---+---+
                  |   |   |
         +--------+   |   +--------+
         |            |            |
   +-----v-----+ +---v---+ +-----v------+
   | Loki 3.6  | | Tempo | | Prometheus |
   | :3100     | | 2.10  | | 2.54       |
   | (logs)    | | :3200 | | :9090      |
   |           | |(trace)| | (metrics)  |
   +-----+-----+ +---+---+ +-----+------+
         |            |            |
         +------------+------------+
                      |
              +-------v-------+
              | Grafana 12.1  |
              | :3000         |
              | - Dashboards  |
              | - Alerts      |
              | - Explore     |
              +---------------+
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| Go Server (grodt-server) | Emit traces, metrics, structured logs via OTLP/HTTP | OTel Collector (:4318) |
| Python Client (grodt-strategy) | Emit traces, metrics, structured logs via OTLP/HTTP | OTel Collector (:4318) |
| OTel Collector | Receive OTLP, batch, route to backends | Loki (:3100), Tempo (:4317), Prometheus (:9090) |
| Loki | Store and index logs | Grafana (queried via LogQL) |
| Tempo | Store traces | Grafana (queried via TraceQL) |
| Prometheus | Store metrics | Grafana (queried via PromQL) |
| Grafana | Visualize, alert, correlate across all backends | Loki, Tempo, Prometheus |

### Data Flow

**Traces:**
1. Application code creates spans via `otel.Tracer("component").Start(ctx, "operation")`
2. OTel SDK batches spans and exports via OTLP/HTTP to Collector
3. Collector forwards to Tempo via OTLP/gRPC
4. Grafana queries Tempo by trace ID or via TraceQL

**Metrics:**
1. Application code records via `otel.Meter("component").Int64Counter("name")`
2. OTel SDK aggregates and exports via OTLP/HTTP to Collector
3. Collector forwards to Prometheus via remote write
4. Grafana queries Prometheus via PromQL

**Logs:**
1. Application logs via logrus (Go) or structlog (Python)
2. OTel bridge (otellogrus / structlog processor) enriches logs with trace_id, span_id
3. Logs exported via OTLP/HTTP to Collector
4. Collector forwards to Loki via otlphttp to `/otlp` endpoint
5. Grafana queries Loki via LogQL, links to traces via trace_id field

## Patterns to Follow

### Pattern 1: Service Resource Identity
**What:** Every telemetry signal includes `service.name` and `service.namespace` as resource attributes.
**When:** Always. Set once at SDK initialization.
**Example:**
```go
res, _ := resource.New(ctx,
    resource.WithAttributes(
        attribute.String("service.name", "grodt-server"),
        attribute.String("service.namespace", "grodt"),
        attribute.String("deployment.environment", os.Getenv("GO_ENV")),
    ),
)
tracerProvider := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(traceExporter),
    sdktrace.WithResource(res),
)
```

### Pattern 2: Playground Context Propagation
**What:** Attach playground UUID to all telemetry within a playground session.
**When:** Any RPC handler that operates on a specific playground.
**Example:**
```go
span.SetAttributes(
    attribute.String("playground.id", playground.ID.String()),
    attribute.String("playground.environment", string(playground.Environment)),
    attribute.String("playground.symbol", string(playground.Symbol)),
)
```

### Pattern 3: Heartbeat as Metric Gauge
**What:** A gauge metric that resets to current timestamp on every tick, plus a periodic log line.
**When:** Python strategy tick loop.
**Example:**
```python
heartbeat_gauge = meter.create_gauge(
    "grodt.strategy.heartbeat_timestamp",
    description="Unix timestamp of last strategy tick",
)
# In tick loop:
heartbeat_gauge.set(time.time(), {"playground.id": playground_id})
logger.info("heartbeat", playground_id=playground_id, tick_count=n)
```

### Pattern 4: OTel Environment Variable Configuration
**What:** Configure OTel SDK via environment variables, not hardcoded values.
**When:** Always. Enables switching between local dev and production without code changes.
**Example:**
```bash
# .env (local dev)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_SERVICE_NAME=grodt-server
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=development
```

### Pattern 5: Trace Context in Twirp HTTP Headers
**What:** Propagate W3C `traceparent` header through Twirp RPC calls for cross-process trace correlation.
**When:** Python client making Twirp calls to Go server.
**Example:**
```python
from opentelemetry.propagate import inject
headers = {"Content-Type": "application/json"}
inject(headers)  # Adds traceparent header
response = requests.post(twirp_url, json=payload, headers=headers)
```

Go side (automatic if `otel.SetTextMapPropagator()` is called and `otelhttp` wraps the handler).

## Anti-Patterns to Avoid

### Anti-Pattern 1: Trace Everything
**What:** Creating spans for every function call, including trivial getters and setters.
**Why bad:** Noise in trace view, performance overhead, storage costs. 100 spans per request makes traces unreadable.
**Instead:** Span at meaningful boundaries: RPC handlers, database calls, external API calls, strategy decisions. Not at utility functions.

### Anti-Pattern 2: Logs as Metrics
**What:** Counting log lines to derive metrics (e.g., counting "order placed" log lines for order rate).
**Why bad:** Logs can be sampled, dropped, or change format. Metrics are purpose-built for counting.
**Instead:** Emit a counter metric (`orders.placed`) alongside the log line. Use the log for context, the metric for the number.

### Anti-Pattern 3: Direct Backend Export
**What:** Configuring applications to export directly to Loki/Tempo/Prometheus, bypassing the Collector.
**Why bad:** Tight coupling to backend URLs. Can't add processing, sampling, or routing changes without redeploying apps.
**Instead:** Always export to OTel Collector. Apps only know `OTEL_EXPORTER_OTLP_ENDPOINT`.

### Anti-Pattern 4: Sensitive Data in Telemetry
**What:** Including API keys, passwords, or full order payloads in span attributes or log fields.
**Why bad:** Telemetry backends are often less secured than application databases.
**Instead:** Log order IDs and statuses, not full payloads. Never log API keys. Use the `resource` processor in OTel Collector to strip attributes if needed.

### Anti-Pattern 5: Ignoring Shutdown
**What:** Not calling `tracerProvider.Shutdown()` on application exit.
**Why bad:** The last batch of telemetry is lost. For a trading platform, this might be the most important data (crash diagnostics).
**Instead:** Register shutdown in a defer or signal handler, as shown in `quickstart.go`.

## Scalability Considerations

| Concern | Single operator (now) | 10 strategies | Production (cloud) |
|---------|----------------------|---------------|-------------------|
| Log volume | Negligible. Loki single-instance handles it. | Still fine. Hundreds of logs/sec is nothing. | Add retention policy (7-30 days). |
| Trace volume | Minimal. ~10 spans per tick, ticks every few seconds. | May want head-based sampling at Collector level. | Add tail-based sampling in Collector. |
| Metric cardinality | Low. Dozens of metrics with few label combos. | Watch for per-playground metric explosion. Use playground_id only on key metrics. | Consider metric aggregation at Collector. |
| Storage | Docker volumes, ephemeral. | Docker volumes with periodic cleanup. | Persistent volumes with S3/object storage backends for Loki and Tempo. |
| Grafana performance | No concern. | No concern. | Consider read replicas if query load is high (unlikely). |

## Sources

- [OTel Collector architecture docs](https://opentelemetry.io/docs/collector/) -- pipeline model (HIGH confidence)
- [Loki OTLP ingestion](https://grafana.com/docs/loki/latest/send-data/otel/) -- native endpoint (HIGH confidence)
- [Tempo OTel integration](https://grafana.com/docs/tempo/latest/) -- OTLP native (HIGH confidence)
- [grafana/docker-otel-lgtm](https://github.com/grafana/docker-otel-lgtm) -- all-in-one reference (HIGH confidence)
- Existing codebase: `deprecated/go/cmd/telemetry/quickstart.go` -- reference OTel setup (HIGH confidence)

---
*Architecture research for: Observability layer on Go + Python trading platform*
*Researched: 2026-03-25*
