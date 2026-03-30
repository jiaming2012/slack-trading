# Live Simulation Observability

## What This Is

An observability layer for the slack-trading platform's live simulation mode. Surfaces real-time visibility into order lifecycle, strategy decisions, market data flow, and system health — built on OpenTelemetry, visualized in Grafana with Loki for logs and Tempo for traces. Deployed to a DigitalOcean droplet running the full trading stack with observability.

## Core Value

When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

## Current State

**v1.1 shipped 2026-03-30.** Dashboard enhancements deployed to production.

- All OTel metrics include client_id label for per-playground filtering
- Grafana dashboard with client_id/playground_id dropdowns, enriched panels
- Per-strategy dashboards: Mean Reversion + Covered Call with strategy-specific panels
- Signal filtering by signal_type, candle filtering by symbol
- Per-playground Python heartbeat (not binary)
- trace_id in order event and position detail panels
- order_filled log bug fixed (was showing zero quantity)

<details>
<summary>v1.0 (shipped 2026-03-28)</summary>

- Go server instrumented with OTel traces, metrics, and structured logs
- Python strategy clients instrumented with OTel spans and heartbeat
- Grafana dashboard with live playground stats, order activity, heartbeat indicators
- Alerting rules for heartbeat staleness and error rate spikes
- End-to-end trace propagation from Python tick loop through Go Twirp RPC
- Logs flowing to Loki via OTLP log bridge (logrus → OTel Log SDK)

</details>

## Requirements

### Validated

- ✓ TracerProvider and MeterProvider initialized in Go server — v1.0 (Phase 2)
- ✓ Order lifecycle instrumented (placed, filled, rejected) with traces and structured logs — v1.0 (Phase 3)
- ✓ Strategy decision flow instrumented in Python with OTel — v1.0 (Phase 4)
- ✓ Market data flow instrumented (candle arrival, tick processing, data gaps) — v1.0 (Phase 3)
- ✓ Heartbeat: periodic metric gauge + log from strategy and server — v1.0 (Phase 3, 4)
- ✓ Local Grafana + Loki + OTel Collector via Docker Compose — v1.0 (Phase 2)
- ✓ Grafana dashboard: live sim activity, strategy state, heartbeat indicator — v1.0 (Phase 6)
- ✓ Grafana alerts: heartbeat stale, error spike — v1.0 (Phase 6)
- ✓ Go infrastructure metrics (CPU, memory, request latency) — v1.0 (Phase 2)
- ✓ Python OTel instrumentation for strategy clients — v1.0 (Phase 4)
- ✓ Deploy observability stack to Digital Ocean — v1.0 (Phase 7)
- ✓ Python codebase restructured into maintainable directory structure — v1.0 (Phase 1)
- ✓ End-to-end tick tracing from Python through Go and back — v1.0 (Phase 5)

### Active

(See REQUIREMENTS.md for v1.1 scoped requirements)

## Current Milestone: v1.1 Dashboard Enhancements

**Goal:** Improve Grafana dashboard usability with per-playground filtering, richer panel detail, client_id labels across all metrics, and per-strategy dashboards.

**Target features:**
- Playground filter showing client_id (fallback to playground_id)
- client_id as OTel metric label on all Go-side instruments
- Active playgrounds listing client_id/playground_id instead of count
- Python heartbeat per playground (not binary)
- Open orders showing symbol + quantity detail
- Signals filtering by signal_type, candles filtering by symbol
- Fix order_filled quantity in logs, add trace_id to order/position panels
- Per-strategy dashboards (separate dashboard per strategy type)

### Out of Scope

- Replacing existing Slack notifications — they continue to work alongside
- Distributed tracing across external APIs (Polygon, Tradier) — instrument our side only
- Custom Grafana plugins — use built-in panels and Loki/Tempo data sources
- Alerting to PagerDuty or other incident tools — Grafana native alerts only for now

## Context

- **Production running**: Full stack on DO droplet (s-2vcpu-4gb, nyc1) — Go server, PostgreSQL, EventStoreDB, otel-lgtm
- **Two-process architecture**: Go server + Python client, both instrumented with OTel
- **Python restructure complete**: 7 subpackages (engine/, strategies/, lib/, demos/, tests/, tools/, deprecated/)
- **Base Docker images**: Built on droplet from source (Vultr registry not accessible from DO). pandas-ta replaced by pandas_ta_classic.

## Constraints

- **Tech stack**: OpenTelemetry → Loki (logs) + Tempo (traces) + Prometheus (metrics) + Grafana
- **Python compatibility**: numpy pinned to 1.23.5 (pandas_ta compat)
- **Infrastructure**: DigitalOcean droplet for production, Docker Compose for local dev

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| OpenTelemetry over custom logging | Already partially adopted, industry standard, vendor-neutral | ✓ Good |
| Loki + Grafana over ELK/Datadog | Lightweight, OSS, good OTel integration | ✓ Good |
| Heartbeat as metric gauge + log line | Metric for dashboard/alerting, log for forensic debugging | ✓ Good |
| Digital Ocean for observability infra | Separate from trading app (Vultr K8s), simple provisioning | ✓ Good |
| grafana/otel-lgtm all-in-one image | Single container for Grafana+Loki+Tempo+Prometheus+OTel Collector | ✓ Good — simple, works well for single-server |
| OTel Log SDK bridge for logrus→Loki | otellogrus only adds span events; needed standalone log export | ✓ Good — logs now visible in Loki |
| pandas_ta_classic over pandas_ta | Original removed from PyPI/GitHub | ✓ Necessary |
| Cloud Firewall over UFW | Docker bypasses UFW; DO Cloud Firewall operates at network level | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

---
*Last updated: 2026-03-30 after v1.1 milestone*
