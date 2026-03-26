# Live Simulation Observability

## What This Is

An observability layer for the slack-trading platform's live simulation mode. Surfaces real-time visibility into order lifecycle, strategy decisions, market data flow, and system health — built on OpenTelemetry, visualized in Grafana with Loki for logs. Solves the core problem: "is the strategy actually running?" especially during quiet periods with no trades.

## Core Value

When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

## Requirements

### Validated

- ✓ OTel Go packages imported (v1.27.0) — existing
- ✓ Tracer spans in ~15 Go source files (eventservices, eventconsumers, eventproducers) — existing
- ✓ otellogrus hook in cmd/main.go — existing
- ✓ Reference implementation in deprecated/go/cmd/telemetry/quickstart.go — existing
- ✓ structlog available in Python conda env — existing

### Active

- [ ] Initialize TracerProvider and MeterProvider in Go server (wire up existing spans)
- [ ] Instrument order lifecycle (placed, filled, rejected) with traces and structured logs
- [ ] Instrument strategy decision flow in Python client with OTel
- [ ] Instrument market data flow (candle arrival, tick processing, data gaps)
- [ ] Heartbeat: periodic metric gauge + log line from strategy and server
- [ ] Local Grafana + Loki + OTel Collector via Docker Compose
- [ ] Grafana dashboard: live sim activity, strategy state, heartbeat indicator
- [ ] Grafana alerts: heartbeat stale, error spike
- [ ] Go infrastructure metrics (CPU, memory, request latency via otelhttp + runtime)
- [ ] Python OTel instrumentation for strategy clients
- [ ] Deploy observability stack to Digital Ocean (via DO MCP server)

### Out of Scope

- Replacing existing Slack notifications — they continue to work alongside
- Distributed tracing across external APIs (Polygon, Tradier) — instrument our side only
- Custom Grafana plugins — use built-in panels and Loki/Tempo data sources
- Alerting to PagerDuty or other incident tools — Grafana native alerts only for now

## Context

- **Existing OTel foundation**: Go packages imported, spans created in ~15 files, but TracerProvider never initialized — all spans currently go nowhere. A complete reference setup exists in `deprecated/go/cmd/telemetry/quickstart.go`.
- **Two-process architecture**: Go server (orders, market data, state) + Python client (strategy decisions). Both need instrumentation for full visibility.
- **Live simulation mode**: The primary use case. Backtesting has its own feedback loop, but live sim runs continuously and needs monitoring.
- **User's Grafana experience**: Some dashboard experience but not extensive — dashboards should be straightforward to understand and extend.
- **Production target**: Digital Ocean for observability infra (separate from Vultr Kubernetes where the trading app runs).

## Constraints

- **Tech stack**: OpenTelemetry (already partially adopted) → Loki (logs) + Grafana (dashboards/alerts)
- **Python compatibility**: numpy pinned to 1.26.4 (pandas_ta compat) — OTel Python packages must be compatible
- **Local dev first**: Docker Compose for local iteration, then Digital Ocean for production
- **Infrastructure provisioning**: Digital Ocean MCP server for creating cloud resources

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| OpenTelemetry over custom logging | Already partially adopted, industry standard, vendor-neutral | — Pending |
| Loki + Grafana over ELK/Datadog | Lightweight, OSS, good OTel integration, user has some Grafana experience | — Pending |
| Heartbeat as metric gauge + log line | Metric for dashboard/alerting, log for forensic debugging | — Pending |
| Digital Ocean for observability infra | Separate from trading app (Vultr K8s), MCP server for easy provisioning | — Pending |
| Full stack instrumentation (Go + Python + infra) | Need end-to-end visibility across both processes | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-25 after initialization*
