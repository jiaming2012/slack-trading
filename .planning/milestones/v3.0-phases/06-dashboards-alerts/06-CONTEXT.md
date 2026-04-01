# Phase 6: Dashboards & Alerts - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a Grafana dashboard for live simulation monitoring (aggregate + per-playground views) with position summary, and configure heartbeat staleness and error rate alerts with notifications to both Grafana UI and Slack webhook.

</domain>

<decisions>
## Implementation Decisions

### Dashboard Layout
- **D-01:** Dashboard organized by **rows of concern**:
  - Row 1: System Health — Go server heartbeat, Python strategy heartbeat, uptime
  - Row 2: Order Activity — orders placed/filled/rejected (live+reconcile only)
  - Row 3: Market Data — candles processed, signals generated, tick latency
  - Row 4: Positions — open positions across live playgrounds
- **D-02:** Playground dropdown filter implementation is Claude's discretion (Grafana template variable recommended).
- **D-03:** Default view shows aggregate across all live playgrounds; dropdown filters to specific playground_id.

### Alert Thresholds
- **D-04:** Heartbeat stale alert threshold is **configurable** — default 2 minutes, adjustable via Grafana alert rule parameter.
- **D-05:** Error rate spike alert threshold is Claude's discretion.
- **D-06:** Alerts notify via **both** Grafana built-in UI and Slack webhook. The platform already has `SLACK_OPTION_ALERTS_WEBHOOK_URL` configured — use it or a new dedicated observability webhook.

### Dashboard Provisioning
- **D-07:** Dashboard JSON model files stored in `observability/dashboards/` and auto-loaded by Grafana provisioning. Version-controlled, reproducible across environments.
- **D-08:** Alert rules also provisioned via JSON/YAML in `observability/alerting/` alongside dashboards.
- **D-09:** Docker Compose updated to mount provisioning directories into the otel-lgtm container.

### Claude's Discretion
- Playground dropdown filter implementation (Grafana variable from Loki labels or Prometheus metrics)
- Error rate spike threshold and evaluation window
- Grafana panel types for each metric (stat, gauge, time series, table, etc.)
- LogQL and PromQL query design for each panel
- Alert evaluation interval and pending period
- Whether to use Grafana's unified alerting or classic alerting
- Slack notification template format

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Observability Stack
- `observability/docker-compose.yaml` — otel-lgtm container (needs volume mounts for provisioning)
- `taskfile.yml` — observability:start/stop/logs tasks

### Metrics Available (from Phase 3)
- `src/go/telemetry/metrics.go` — Go OTel instruments: OrderPlaced, OrderFilled, OrderRejected counters; CandlesProcessed counter; TickLatency histogram; SignalsGenerated counter
- `src/go/telemetry/heartbeat.go` — Go heartbeat gauge with environment/account_type labels
- `src/go/data/database_service.go` — GetHeartbeatStats() for playground counts

### Metrics Available (from Phase 4)
- `src/clients/python/engine/heartbeat.py` — Python StrategyHeartbeat gauge
- `src/clients/python/engine/otel.py` — Python OTel setup (service name: grodt-strategy)

### Structured Logs Available
- Go server logs: logfmt format with playground_id, order_id, symbol, trace_id, environment fields
- Python strategy logs: signal decisions with signal_type, direction, decision, reason fields

### Existing Slack Integration
- `.env` — SLACK_OPTION_ALERTS_WEBHOOK_URL (existing webhook)

</canonical_refs>

<code_context>
## Existing Code Insights

### Grafana Provisioning
- otel-lgtm container auto-provisions Loki, Tempo, Prometheus data sources
- Dashboard provisioning needs a `dashboards.yaml` provisioning config + JSON model files
- Alert provisioning via Grafana's file-based provisioning or API

### Available Data Sources
- **Prometheus** — heartbeat gauges, order counters, candle counters, tick latency
- **Loki** — structured logs (order lifecycle, signal decisions, data gaps)
- **Tempo** — distributed traces (Python → Go tick/order traces)

### Integration Points
- Docker Compose volumes mount `observability/dashboards/` into Grafana's provisioning path
- Slack webhook URL from .env or hardcoded in alert notification channel config

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-dashboards-alerts*
*Context gathered: 2026-03-27*
