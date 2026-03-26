# Domain Pitfalls: Observability for Trading Platform

**Domain:** Full-stack observability (OTel + Grafana ecosystem)
**Researched:** 2026-03-25

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

### Pitfall 1: OTel Providers Never Initialized (EXISTING BUG)
**What goes wrong:** All OTel Go packages are imported, spans are created in ~15 files, otellogrus hook is registered -- but TracerProvider and MeterProvider are never initialized in `cmd/main.go`. Every span is a no-op. Every metric is silently dropped.
**Why it happens:** The quickstart reference code is in `deprecated/` and was never integrated into the main server startup.
**Consequences:** Zero observability despite having instrumentation code everywhere. Developers think tracing is "set up" when it isn't.
**Prevention:** First task of Phase 1: copy `setupOTelSDK()` from `quickstart.go` into `cmd/main.go` and call it before any server startup.
**Detection:** Check if `otel.GetTracerProvider()` returns a no-op provider. Or simply check if any traces appear in Tempo.

### Pitfall 2: Using lokiexporter Instead of otlphttp
**What goes wrong:** The OTel Collector's `lokiexporter` (in collector-contrib) is deprecated. Tutorials from 2023-2024 still reference it. Using it means building on deprecated code that will be removed.
**Why it happens:** Stale documentation, blog posts, and Stack Overflow answers.
**Consequences:** Exporter removed in future collector versions. Migration required under pressure.
**Prevention:** Use the standard `otlphttp` exporter pointing at Loki's native `/otlp` endpoint. Loki 3.x supports OTLP natively.
**Detection:** Grep collector config for `lokiexporter`. If present, replace with `otlphttp/logs`.

### Pitfall 3: Python grpcio Dependency Conflict
**What goes wrong:** Installing `opentelemetry-exporter-otlp` (the gRPC variant) pulls in `grpcio`, which has complex native build requirements and can conflict with the numpy 1.26.4 pin required by pandas_ta.
**Why it happens:** The default OTel Python exporter package bundles gRPC. Most tutorials show the gRPC variant.
**Consequences:** Broken conda environment, hours debugging native extension builds, potential numpy version bump that breaks pandas_ta.
**Prevention:** Use `opentelemetry-exporter-otlp-proto-http` exclusively. HTTP transport has no native dependencies and works identically for this use case.
**Detection:** Check `pip list | grep grpcio` in the conda env. If present and not explicitly needed, investigate.

### Pitfall 4: Missing TracerProvider Shutdown on Exit
**What goes wrong:** The OTel SDK batches telemetry for efficiency. If the application exits without calling `tracerProvider.Shutdown()`, the last batch is lost.
**Why it happens:** Easy to forget in Go where `defer` is per-function, not per-application. Trading platforms may exit on SIGTERM during critical events.
**Consequences:** Crash diagnostics are missing. The most important telemetry (what happened right before the crash) is lost.
**Prevention:** Register shutdown in a signal handler or top-level defer, exactly as `quickstart.go` demonstrates.
**Detection:** Kill the server and check if the final traces appear in Tempo. If not, shutdown isn't working.

## Moderate Pitfalls

### Pitfall 5: Metric Cardinality Explosion from Playground IDs
**What goes wrong:** Attaching `playground.id` (a UUID) as a metric label creates a new time series for every playground. With 37+ playgrounds, this creates hundreds of series per metric.
**Prevention:** Use `playground.id` as a span attribute (traces) and log field (logs), but NOT as a metric label. For metrics, use `playground.environment` (simulator/live/reconcile) which has only 3 values.

### Pitfall 6: Loki Structured Metadata Not Enabled
**What goes wrong:** OTLP log ingestion to Loki uses structured metadata for resource attributes. If `allow_structured_metadata` is false, Loki rejects the payload.
**Prevention:** Use Loki 3.x (3.6.0 recommended) which enables structured metadata by default. If using Loki 2.x (don't), set `allow_structured_metadata: true` in limits_config.

### Pitfall 7: OTel Collector Core vs Contrib Confusion
**What goes wrong:** Deploying the "core" OTel Collector (`otel/opentelemetry-collector`) instead of the "contrib" distribution (`otel/opentelemetry-collector-contrib`). Core lacks `prometheusremotewrite` exporter and other commonly needed components.
**Prevention:** Always use the contrib Docker image: `otel/opentelemetry-collector-contrib:0.148.0`.

### Pitfall 8: Promtail in New Setups
**What goes wrong:** Using Promtail for log collection in a new observability setup. Promtail was deprecated in Feb 2025 and reaches EOL in Feb 2026.
**Prevention:** Don't use Promtail. Use OTel Collector for log collection (applications export OTLP logs directly).

### Pitfall 9: Trace Context Not Propagated Across Twirp RPC
**What goes wrong:** Go server traces and Python client traces appear as separate, unlinked traces because the W3C `traceparent` header isn't propagated through Twirp HTTP calls.
**Why it happens:** Twirp uses HTTP transport, which supports header propagation, but the Python client doesn't inject trace context into headers by default.
**Prevention:** In Python, use `opentelemetry.propagate.inject(headers)` before making Twirp calls. In Go, ensure `otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(...))` is called and `otelhttp` wraps the Twirp handler.

## Minor Pitfalls

### Pitfall 10: Grafana Data Source Provisioning Order
**What goes wrong:** Grafana starts before Loki/Tempo/Prometheus are ready. Provisioned data sources show errors on first load.
**Prevention:** Add `depends_on` with health checks in Docker Compose. Or just refresh Grafana after all services are up -- data sources will reconnect automatically.

### Pitfall 11: OTLP Port Collision
**What goes wrong:** Both OTel Collector and Tempo listen on port 4317 (OTLP gRPC) by default. If running in Docker Compose with host networking, they collide.
**Prevention:** Remap Tempo's OTLP port in Docker Compose (e.g., `4327:4317`). The Collector connects to Tempo's container name, not the host port.

### Pitfall 12: Console Exporter Left in Production
**What goes wrong:** `OTEL_TRACES_EXPORTER=console` is set for debugging and forgotten. All traces go to stdout, nothing reaches the backend.
**Prevention:** Use environment-specific .env files. `console` exporter only in development when explicitly debugging OTel itself.

### Pitfall 13: Loki 3.5.8+ Docker Image Has No Shell
**What goes wrong:** Trying to `docker exec -it loki /bin/sh` fails because busybox was removed from the Docker image starting in Loki 3.5.8.
**Prevention:** Use `docker logs` to view container output. For config debugging, mount configs as volumes and validate before starting.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Go OTel init | Pitfall 1: Providers not initialized | Copy setupOTelSDK from quickstart.go |
| Docker Compose setup | Pitfall 7: Core vs Contrib collector | Use `otel/opentelemetry-collector-contrib` |
| Docker Compose setup | Pitfall 11: Port collision | Remap Tempo OTLP port |
| Loki integration | Pitfall 2: lokiexporter (deprecated) | Use otlphttp exporter to /otlp |
| Loki integration | Pitfall 6: Structured metadata | Use Loki 3.x (enabled by default) |
| Python instrumentation | Pitfall 3: grpcio dependency | Use otlp-proto-http, not otlp |
| Cross-process tracing | Pitfall 9: traceparent not propagated | inject() in Python, otelhttp wrap in Go |
| Domain metrics | Pitfall 5: Cardinality explosion | playground.id in traces/logs only, not metrics |
| Server shutdown | Pitfall 4: Missing shutdown | Defer shutdown in signal handler |
| Production deploy | Pitfall 12: Console exporter | Environment-specific config |

## Sources

- [Loki OTLP docs](https://grafana.com/docs/loki/latest/send-data/otel/) -- lokiexporter deprecation, structured metadata requirement (HIGH confidence)
- [Promtail deprecation](https://community.grafana.com/t/how-does-alloy-relate-to-otel-collector/134548) -- EOL timeline (HIGH confidence)
- [OTel Collector distributions](https://opentelemetry.io/docs/collector/) -- core vs contrib (HIGH confidence)
- Existing codebase: `go.mod`, `cmd/main.go`, `quickstart.go` -- verified provider not initialized (HIGH confidence)
- [OpenTelemetry Python exporters](https://opentelemetry.io/docs/languages/python/exporters/) -- HTTP vs gRPC variants (HIGH confidence)
- [Loki 3.5.8 release notes](https://grafana.com/docs/loki/latest/release-notes/) -- busybox removal (MEDIUM confidence)

---
*Pitfalls research for: Observability on Go + Python trading platform*
*Researched: 2026-03-25*
