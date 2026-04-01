# Live Simulation Observability

## What This Is

An observability layer for the slack-trading platform's live simulation mode. Surfaces real-time visibility into order lifecycle, strategy decisions, market data flow, and system health — built on OpenTelemetry, visualized in Grafana with Loki for logs and Tempo for traces. Deployed to a DigitalOcean droplet running the full trading stack with observability.

## Core Value

When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

## Current State

**v3.0 shipped 2026-04-01.** TradeSignal Framework — decoupled signal production from strategy execution.

- Canonical TradeSignal type in Go + proto with signal_id linking every order to its originating signal
- ISignalRepository with InMemory (sim) and ESDB (live) implementations, clock-gated delivery
- WriteSignal, GetSignals, GetProcessedSignals Twirp RPCs callable from Python
- All 6 strategies migrated to V2 (signal-consuming), V1s in deprecated/, behavioral diff tests
- Standalone datasource scripts with DatasourceHeartbeat, OTel metrics, Grafana alerting
- Signal replay from ESDB streams with integration tests proving equivalence
- Self-contained OTel E2E tests via TestContainers (collector + Prometheus metrics verification)

<details>
<summary>v1.1 (shipped 2026-03-30)</summary>

- All OTel metrics include client_id label for per-playground filtering
- Grafana dashboard with client_id/playground_id dropdowns, enriched panels
- Per-strategy dashboards: Mean Reversion + Covered Call with strategy-specific panels
- Signal filtering by signal_type, candle filtering by symbol
- Per-playground Python heartbeat (not binary)
- trace_id in order event and position detail panels
- order_filled log bug fixed (was showing zero quantity)

</details>

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

- ✓ All v1.0 observability requirements — v1.0 (Phases 1-7)
- ✓ All v1.1 dashboard enhancement requirements — v1.1 (Phases 8-11)
- ✓ Metabase infrastructure, analytics, dashboards, spread grouping — v2.0 (Phases 12-16)
- ✓ TradeSignal type, repositories, RPC endpoints, strategy migrations, replay, observability — v3.0 (Phases 17-26)

### Active

(Planning next milestone)

### Out of Scope

- Composite signal framework (complex multi-signal aggregation)
- Signal backtesting optimizer (grid search over signal parameters)
- External signal sources (third-party APIs producing signals)
- Replacing existing Slack notifications
- Distributed tracing across external APIs (Polygon, Tradier)
- Custom Grafana plugins
- Alerting to PagerDuty or other incident tools

## Context

- **Production running**: Full stack on DO droplet (s-2vcpu-4gb, nyc1) — Go server, PostgreSQL, EventStoreDB, otel-lgtm
- **Two-process architecture**: Go server + Python client, both instrumented with OTel
- **Python restructure complete**: 7 subpackages (engine/, strategies/, lib/, demos/, tests/, tools/, deprecated/)
- **Base Docker images**: Built on droplet from source (Vultr registry not accessible from DO). pandas-ta replaced by pandas_ta_classic.

## Constraints

- **Tech stack**: OpenTelemetry → Loki (logs) + Tempo (traces) + Prometheus (metrics) + Grafana
- **Python compatibility**: pandas-ta-classic requires numpy>=2.0
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
| Single global signal stream | All signals in one ESDB stream, filter by attributes not partition | ✓ Good — simpler, proven in replay tests |
| Datasource dual-mode pattern | __main__ for live (WriteSignal RPC), direct import for sim | ✓ Good — clean separation |
| Behavioral diff tests for migration | V1/V2 metric comparison proves equivalence | ✓ Good — caught real bugs |
| Local Docker builds (no registry) | Vultr registry removed, 3-layer local build chain | ✓ Good — simpler, no external dependency |
| OTel collector in E2E tests | TestContainers + debug exporter + Prometheus /metrics | ✓ Good — self-contained telemetry verification |

## Evolution

This document evolves at phase transitions and milestone boundaries.

---
*Last updated: 2026-04-01 after v3.0 milestone*
