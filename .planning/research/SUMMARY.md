# Project Research Summary

**Project:** slack-trading (grodt) -- Observability Layer
**Domain:** Full-stack observability for event-driven Go + Python trading platform
**Researched:** 2026-03-25
**Confidence:** HIGH

## Executive Summary

This project adds observability (logs, traces, metrics) to an existing Go + Python trading platform that already has partial OpenTelemetry instrumentation -- Go SDK packages are imported, spans exist in ~15 files, and an otellogrus hook is registered -- but none of it works because the TracerProvider and MeterProvider were never initialized. The recommended approach is the standard Grafana ecosystem stack: OpenTelemetry Collector routing telemetry to Loki (logs), Tempo (traces), and Prometheus (metrics), with Grafana as the unified dashboard. This is the dominant pattern for OSS observability and is thoroughly documented.

The key insight is that this is NOT a greenfield instrumentation effort. Roughly 60% of the Go-side plumbing already exists (OTel imports, span creation, logrus hook, a complete reference implementation in `deprecated/go/cmd/telemetry/quickstart.go`). The critical first step is simply initializing the SDK providers in `cmd/main.go` -- this single change lights up all existing instrumentation. The Python side needs new OTel SDK packages installed, but the HTTP-based exporter avoids the dangerous grpcio dependency that would break the numpy pin required by pandas_ta.

The primary risk is premature complexity. This is a single-operator platform. The temptation to build distributed tracing across processes, complex alerting pipelines, and auto-instrumentation should be resisted in favor of getting the basics working first: provider initialization, local backend via Docker, and a strategy heartbeat metric that answers the core question -- "is my strategy actually running?"

## Key Findings

### Recommended Stack

The entire Go-side OTel SDK (v1.27.0) is already in go.mod and does not need upgrading. The only new Go dependency is the optional OTLP log exporter. Python needs four new packages installed via pip in the conda env, using the HTTP exporter variant exclusively to avoid grpcio conflicts.

**Core technologies:**
- **OTel Go SDK v1.27.0**: Traces + metrics from Go server -- already imported, just needs provider initialization
- **OTel Python SDK 1.40.0**: Traces + metrics from Python strategy clients -- HTTP exporter, no grpcio
- **OTel Collector (contrib) 0.148.0**: Telemetry routing hub -- receives OTLP, exports to Loki/Tempo/Prometheus
- **Grafana Loki 3.6.0**: Log aggregation -- native OTLP ingestion, no deprecated lokiexporter
- **Grafana Tempo 2.10.3**: Trace storage -- native OTLP, no indexing required
- **Prometheus 2.54+**: Metrics backend -- receives via remote write from Collector
- **Grafana 12.1 OSS**: Unified dashboards and alerting across all three backends
- **grafana/otel-lgtm Docker image**: All-in-one local dev backend (zero config)

### Expected Features

**Must have (table stakes):**
- Structured log aggregation -- searchable logs from k8s pods
- Request tracing on Go RPC handlers -- traces CreatePlayground, NextTick, PlaceOrder
- Infrastructure metrics -- CPU, memory, goroutines, GC pressure
- HTTP request metrics -- latency, error rate, throughput per endpoint
- Log-trace correlation -- click from log line to trace
- Centralized Grafana dashboard

**Should have (differentiators):**
- Strategy heartbeat -- periodic gauge proving Python strategy is alive (the killer feature)
- Order lifecycle tracing -- trace from Python decision through Go fill/reject
- Strategy decision logging -- structured logs with indicator values and signal evaluation
- Quiet period indicator -- distinguish "no signals" from "strategy crashed"
- Error spike alerting -- LogQL alert on error rate threshold

**Defer (v2+):**
- Cross-process distributed tracing (requires trace context propagation setup)
- Production deployment to Vultr Kubernetes
- Grafana alerting rules (build dashboards first, understand data patterns)
- Full Python auto-instrumentation (manual instrumentation of key points is more valuable)

### Architecture Approach

The architecture follows the standard OTel Collector fan-out pattern: both Go and Python applications export OTLP/HTTP to a single Collector, which routes logs to Loki, traces to Tempo, and metrics to Prometheus. Grafana queries all three backends. Applications never talk directly to backends, and configuration is via environment variables so the same code works in dev and production.

**Major components:**
1. **Go Server (grodt-server)** -- emits traces, metrics, structured logs via OTLP/HTTP
2. **Python Client (grodt-strategy)** -- emits traces, metrics (including heartbeat), logs via OTLP/HTTP
3. **OTel Collector (contrib)** -- receives, batches, routes telemetry to backends
4. **Loki + Tempo + Prometheus** -- signal-specific storage backends
5. **Grafana** -- unified visualization, alerting, cross-signal correlation

**Key patterns:** Service resource identity on all telemetry, playground UUID as span/log attribute (NOT metric label), heartbeat as gauge metric, env-var-based OTel configuration, W3C traceparent propagation through Twirp HTTP headers.

### Critical Pitfalls

1. **OTel providers never initialized (EXISTING BUG)** -- All Go instrumentation is no-op today. Fix first by copying `setupOTelSDK()` from `quickstart.go` into `cmd/main.go`.
2. **Python grpcio dependency conflict** -- Installing the wrong OTel exporter package pulls grpcio, breaking the numpy 1.26.4 pin. Use `opentelemetry-exporter-otlp-proto-http` exclusively.
3. **Using deprecated lokiexporter** -- Stale tutorials reference it. Use `otlphttp` exporter to Loki's native `/otlp` endpoint instead.
4. **Missing TracerProvider shutdown** -- Last telemetry batch lost on exit (crash diagnostics). Register shutdown in signal handler.
5. **Metric cardinality explosion** -- Using playground UUID as a metric label creates unbounded time series. Use it in traces/logs only; use `playground.environment` (3 values) for metrics.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Go OTel Initialization + Local Backend

**Rationale:** Unlocks ALL existing instrumentation (spans in ~15 files, runtime metrics, otellogrus hook) with minimal code. This is the highest-value, lowest-effort phase. Must come first because everything else depends on a working telemetry pipeline.
**Delivers:** Working traces and metrics from Go server visible in Grafana. End-to-end pipeline proven.
**Addresses:** TracerProvider/MeterProvider init, infrastructure metrics, HTTP request metrics, log-trace correlation, centralized dashboard
**Avoids:** Pitfall 1 (providers not initialized), Pitfall 4 (missing shutdown), Pitfall 7 (core vs contrib collector), Pitfall 11 (port collision)

### Phase 2: Python Client Instrumentation + Strategy Heartbeat

**Rationale:** The core differentiator -- answering "is my strategy running?" -- requires Python-side instrumentation. Depends on Phase 1 proving the pipeline works. The heartbeat gauge is the single most valuable metric for this platform.
**Delivers:** Strategy heartbeat metric, structured decision logging from Python, tick processing metrics
**Addresses:** Strategy heartbeat, strategy decision logging, quiet period indicator, market data visibility
**Avoids:** Pitfall 3 (grpcio dependency), Pitfall 5 (metric cardinality from playground IDs)

### Phase 3: Cross-Process Tracing + Grafana Dashboards

**Rationale:** With both sides instrumented independently, link them via W3C traceparent propagation. Build purpose-built dashboards now that data patterns are understood from Phases 1-2. Add alerting rules based on observed baselines.
**Delivers:** Unified traces across Python-to-Go RPC calls, order lifecycle tracing, operational dashboards, error spike alerts
**Addresses:** Order lifecycle tracing, error spike alerting, playground session tracing
**Avoids:** Pitfall 9 (traceparent not propagated)

### Phase 4: Production Deployment

**Rationale:** Only deploy to Vultr Kubernetes after observability works locally. Production introduces persistence, resource limits, and Kubernetes-specific config (separate Docker Compose, Helm charts or Kustomize manifests).
**Delivers:** Observability running in production alongside the trading platform
**Addresses:** Production-grade log retention, persistent storage, environment-specific configuration
**Avoids:** Pitfall 12 (console exporter left in production), Pitfall 6 (Loki structured metadata)

### Phase Ordering Rationale

- Phase 1 before Phase 2 because you must prove the pipeline (Collector -> backends -> Grafana) works with the simpler Go side before adding Python.
- Phase 2 before Phase 3 because cross-process tracing is meaningless until both sides emit traces independently.
- Phase 3 before Phase 4 because dashboards and alerts should be designed locally where iteration is fast.
- Each phase is independently valuable -- if you stop after Phase 2, you still have a useful observability setup.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3:** Cross-process trace propagation through Twirp HTTP is straightforward in theory but the Python Twirp client (`backtester_playground_client_grpc.py`) uses raw `requests.post()`. Need to verify where to inject headers.
- **Phase 4:** Kubernetes deployment patterns for the Grafana stack (Helm charts vs manifests, persistent volume config for Vultr). The existing Flux CD GitOps setup may have opinions.

Phases with standard patterns (skip research-phase):
- **Phase 1:** The reference implementation exists in `quickstart.go`. Docker Compose config is provided in STACK.md. This is copy-paste-adapt work.
- **Phase 2:** Standard OTel Python SDK usage. Well-documented, pip install + init code.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against official releases and existing go.mod. No speculative choices. |
| Features | HIGH | Feature set derived from documented OTel capabilities and existing codebase gaps. Clear MVP prioritization. |
| Architecture | HIGH | Standard OTel Collector fan-out pattern. Official docs, reference implementations, and community consensus all agree. |
| Pitfalls | HIGH | Pitfall 1 (existing bug) verified by codebase inspection. Others sourced from official deprecation notices and documented incompatibilities. |

**Overall confidence:** HIGH

### Gaps to Address

- **Loki retention policy:** Research did not determine optimal retention period for trading logs. Default is fine for dev; production needs explicit configuration (7-30 days suggested but depends on storage budget).
- **Grafana provisioning:** The exact dashboard JSON and data source provisioning YAML were not specified. Phase 1 can use manual setup; Phase 3 should codify as provisioning files.
- **Kubernetes resource limits:** No research on memory/CPU requirements for Loki, Tempo, Prometheus in production. Needs sizing during Phase 4 planning.
- **Existing span quality:** The ~15 files with existing spans were identified but not audited for quality. Some may need span name cleanup or additional attributes during Phase 1.

## Sources

### Primary (HIGH confidence)
- OpenTelemetry Go SDK releases (v1.27.0 current, v1.42.0 latest)
- OpenTelemetry Python SDK on PyPI (v1.40.0)
- OTel Collector releases (v0.148.0 contrib)
- Grafana Loki OTLP ingestion docs -- native endpoint, lokiexporter deprecation
- Grafana Tempo docs -- native OTLP ingestion
- grafana/docker-otel-lgtm GitHub -- all-in-one dev image
- Existing codebase: `go.mod`, `cmd/main.go`, `deprecated/go/cmd/telemetry/quickstart.go`

### Secondary (MEDIUM confidence)
- Grafana Docker Hub tags (v12.1)
- structlog OTel integration (blog post)
- Loki 3.5.8 release notes (busybox removal)

---
*Research completed: 2026-03-25*
*Ready for roadmap: yes*
