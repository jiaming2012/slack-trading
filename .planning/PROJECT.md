# Live Simulation Observability

## What This Is

An observability layer for the slack-trading platform's live simulation mode. Surfaces real-time visibility into order lifecycle, strategy decisions, market data flow, and system health — built on OpenTelemetry, visualized in Grafana with Loki for logs and Tempo for traces. Deployed to a DigitalOcean droplet running the full trading stack with observability.

## Core Value

When a live simulation is running, the operator can always tell whether the system is alive and what it's doing — even when no trades are being placed.

## Current State

**v2.0 shipped 2026-03-30.** Metabase analytics layer deployed alongside Grafana.

- 5 Metabase dashboards (Trading Performance, Slippage, Portfolio, Strategy Comparison, Spread Analytics) provisioned programmatically via `infra/provision-metabase.py`
- 8 SQL analytics views (P&L, slippage, stats, spreads) in `infra/analytics-schema.sql`
- Backtest persistence with `--save-to-db` flag, `backtest_runs` summary table
- Spread grouping: Go server auto-injects `spread_group_key` on PlaceMultiLegOrder
- Metabase on Windows desktop, Postgres on DO droplet with read-only `metabase_ro` user

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
- ✓ Metabase infrastructure (docker-compose, metabase_ro, cloud firewall) — v2.0 (Phase 12)
- ✓ Analytics schema (composite indexes, P&L/slippage/stats SQL views) — v2.0 (Phase 13)
- ✓ Core performance dashboards (Trading, Slippage, Portfolio) — v2.0 (Phase 14)
- ✓ Simulator persistence + backtest comparison dashboard — v2.0 (Phase 15)
- ✓ Spread analytics (grouping, views, dashboard) — v2.0 (Phase 16)

### Active

(None — start next milestone with `/gsd:new-milestone`)

**Target features:**
- Metabase deployed on DO droplet via docker-compose (Postgres connection)
- Trading performance dashboards: P&L, profit factor, win rate, slippage, trade duration
- Spread-aware analytics: multi-leg options grouped as single trades
- Strategy optimization: compare backtests and strategy types
- Simulator playground persistence: save backtest results to Postgres
- Portfolio analytics with per-symbol and per-asset-class breakdown

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
*Last updated: 2026-03-30 after v2.0 milestone start*
