# Phase 6: Dashboards & Alerts - Research

**Researched:** 2026-03-26
**Domain:** Grafana dashboard provisioning, PromQL/LogQL alerting, otel-lgtm container configuration
**Confidence:** HIGH

## Summary

Phase 6 builds a Grafana dashboard and alert rules on top of the existing otel-lgtm container (Phase 2) and the OTel metrics/logs instrumented in Phases 3-5. The work is entirely configuration -- no application code changes are needed. All artifacts are JSON/YAML files mounted into the Grafana container via Docker Compose volumes.

The otel-lgtm image uses Grafana's standard file-based provisioning at `/otel-lgtm/grafana/conf/provisioning/`. Dashboard JSON models go in a `dashboards/custom/` subdirectory with a provider YAML; alert rules, contact points, and notification policies go in an `alerting/` subdirectory. OTel metrics are exposed in Prometheus with dots converted to underscores (e.g., `grodt_orders_placed_total`), and structured logs are queryable in Loki via LogQL.

**Primary recommendation:** Create four files -- a dashboard JSON model, a dashboards provider YAML, an alerting rules YAML (containing alert rules + contact point + notification policy), and update docker-compose.yaml with volume mounts. Use Grafana unified alerting (default since Grafana 9+).

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Dashboard organized by rows of concern: System Health, Order Activity, Market Data, Positions
- D-03: Default view shows aggregate across all live playgrounds; dropdown filters to specific playground_id
- D-04: Heartbeat stale alert threshold is configurable, default 2 minutes
- D-06: Alerts notify via both Grafana built-in UI and Slack webhook (use existing SLACK_OPTION_ALERTS_WEBHOOK_URL or new dedicated webhook)
- D-07: Dashboard JSON models stored in observability/dashboards/ and auto-loaded by Grafana provisioning
- D-08: Alert rules provisioned via YAML in observability/alerting/
- D-09: Docker Compose updated to mount provisioning directories into the otel-lgtm container

### Claude's Discretion
- Playground dropdown filter implementation (Grafana template variable from Prometheus labels or Loki labels)
- Error rate spike threshold and evaluation window
- Grafana panel types for each metric (stat, gauge, time series, table, etc.)
- PromQL and LogQL query design for each panel
- Alert evaluation interval and pending period
- Whether to use Grafana unified alerting or classic alerting
- Slack notification template format

### Deferred Ideas (OUT OF SCOPE)
None

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DASH-01 | Grafana dashboard shows aggregate live simulation activity (all live playgrounds) | Dashboard JSON model with PromQL queries against grodt_* metrics; default "All" template variable value |
| DASH-02 | Dashboard has dropdown filter to select a specific playground_id | Grafana template variable using `label_values()` on playground_id from Prometheus metrics or Loki labels |
| DASH-03 | Dashboard includes position summary panel showing open positions across live playgrounds | Table panel querying `grodt_heartbeat_open_orders` gauge or Loki structured log query for order state |
| ALERT-01 | Grafana alert fires when Go server heartbeat is stale (>2 minutes) | PromQL: `time() - grodt_heartbeat_uptime_seconds` or absent() on heartbeat gauge; unified alerting rule |
| ALERT-02 | Grafana alert fires when Python strategy heartbeat is stale (>2 minutes) | PromQL: absent_over_time on `grodt_strategy_heartbeat` gauge |
| ALERT-03 | Grafana alert fires when error log rate exceeds threshold | LogQL: `rate({service_name=~"grodt.*"} |= "level=error" [5m])` with threshold condition |

</phase_requirements>

## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Grafana (in otel-lgtm) | 11.x (bundled) | Dashboard UI & alerting engine | Already deployed via Phase 2; unified alerting is default |
| Prometheus (in otel-lgtm) | bundled | Metrics backend for PromQL queries | Already receiving OTel metrics via collector |
| Loki (in otel-lgtm) | bundled | Log backend for LogQL queries | Already receiving structured logs |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| Grafana file provisioning | Dashboard + alert config-as-code | All dashboard/alert definition -- no UI-based config |
| Docker Compose volumes | Mount provisioning files into container | Updated docker-compose.yaml |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| File provisioning | Grafana HTTP API | API requires running instance; files are version-controlled and reproducible |
| Grafana unified alerting | Classic alerting (legacy) | Classic is deprecated since Grafana 11; unified is the standard |
| Grafonnet (Jsonnet) | Hand-written JSON | Grafonnet is powerful but adds a build step; for 1 dashboard, raw JSON is simpler |

## Architecture Patterns

### Recommended File Structure
```
observability/
  docker-compose.yaml           # Updated: volume mounts for provisioning
  dashboards/
    dashboards-provider.yaml    # Grafana provisioning provider config
    grodt-live-simulation.json  # Dashboard JSON model
  alerting/
    alerting.yaml               # Alert rules + contact point + notification policy
```

### Pattern 1: Dashboard Provisioning Provider
**What:** A YAML file that tells Grafana where to find dashboard JSON files.
**When to use:** Always -- required for file-based dashboard provisioning.
**Example:**
```yaml
# Source: https://grafana.com/docs/grafana/latest/administration/provisioning/
apiVersion: 1
providers:
  - name: "grodt-dashboards"
    type: file
    options:
      path: /otel-lgtm/grafana/conf/provisioning/dashboards/custom
      foldersFromFilesStructure: false
```

### Pattern 2: Dashboard JSON Model with Template Variables
**What:** Dashboard JSON with a `templating.list` entry for playground_id dropdown.
**When to use:** DASH-02 requirement -- dropdown filter.
**Example (template variable section):**
```json
{
  "templating": {
    "list": [
      {
        "name": "playground_id",
        "type": "query",
        "datasource": { "type": "prometheus", "uid": "prometheus" },
        "query": "label_values(grodt_orders_placed_total, playground_id)",
        "includeAll": true,
        "allValue": ".*",
        "current": { "text": "All", "value": "$__all" },
        "refresh": 2
      }
    ]
  }
}
```
**Note:** PromQL panels use `=~"$playground_id"` to match the regex variable.

### Pattern 3: Unified Alerting File Provisioning
**What:** Single YAML file containing alert rules, contact points, and notification policies.
**When to use:** All alert definitions (ALERT-01 through ALERT-03).
**Example:**
```yaml
# Source: https://grafana.com/docs/grafana/latest/alerting/set-up/provision-alerting-resources/file-provisioning/
apiVersion: 1
contactPoints:
  - orgId: 1
    name: grodt-slack
    receivers:
      - uid: grodt-slack-webhook
        type: slack
        settings:
          url: ${SLACK_OBSERVABILITY_WEBHOOK_URL}
          title: "Grodt Alert: {{ .CommonLabels.alertname }}"

policies:
  - orgId: 1
    receiver: grafana-default-email
    group_by:
      - grafana_folder
      - alertname
    routes:
      - receiver: grodt-slack
        object_matchers:
          - ["severity", "=", "critical"]

groups:
  - orgId: 1
    name: grodt-heartbeat
    folder: grodt-alerts
    interval: 30s
    rules:
      - uid: go-heartbeat-stale
        title: Go Server Heartbeat Stale
        condition: C
        for: 2m
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              expr: "grodt_heartbeat_uptime_seconds"
        labels:
          severity: critical
```

### Pattern 4: Docker Compose Volume Mounts
**What:** Mount local provisioning directories into the otel-lgtm container.
**When to use:** D-09 requirement.
**Example:**
```yaml
services:
  otel-lgtm:
    image: grafana/otel-lgtm
    volumes:
      - otel-lgtm-data:/data
      # Dashboard provisioning
      - ./dashboards/dashboards-provider.yaml:/otel-lgtm/grafana/conf/provisioning/dashboards/custom.yaml:ro
      - ./dashboards/grodt-live-simulation.json:/otel-lgtm/grafana/conf/provisioning/dashboards/custom/grodt-live-simulation.json:ro
      # Alert provisioning
      - ./alerting/alerting.yaml:/otel-lgtm/grafana/conf/provisioning/alerting/grodt-alerting.yaml:ro
    environment:
      - GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/otel-lgtm/grafana/conf/provisioning/dashboards/custom/grodt-live-simulation.json
```

### Anti-Patterns to Avoid
- **Creating dashboards via Grafana UI then exporting:** Creates drift between repo and deployed state. Always author JSON files directly.
- **Separate files per alert rule:** Grafana reads all YAML in the alerting provisioning dir; having too many files makes it harder to reason about the alert tree. One file per concern group is sufficient.
- **Hardcoding webhook URLs in provisioning YAML:** Use `${ENV_VAR}` substitution (Grafana supports this in provisioning files).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Dashboard JSON from scratch | Manual JSON authoring | Export from Grafana UI as starting template, then version-control | Dashboard JSON is 500+ lines; starting from scratch is error-prone |
| Heartbeat staleness detection | Custom Go/Python monitoring | Grafana alert rule with `absent_over_time()` PromQL | Built-in Prometheus function handles exactly this case |
| Slack notification formatting | Custom webhook sender | Grafana contact point with Slack type | Handles retries, templating, resolve messages automatically |
| Log-based error rate alerting | Custom log parser | Grafana alert with LogQL `rate()` | Loki handles aggregation natively |

**Key insight:** This phase is 100% configuration. No application code should be written. Every requirement maps to a Grafana provisioning artifact.

## Common Pitfalls

### Pitfall 1: OTel Metric Name Translation
**What goes wrong:** PromQL queries use OTel metric names (e.g., `grodt.orders.placed`) but Prometheus stores them with underscores and suffixes.
**Why it happens:** OTel-to-Prometheus translation converts dots to underscores and appends `_total` to counters.
**How to avoid:** Use Prometheus-format names in all PromQL queries:
- `grodt.orders.placed` (counter) becomes `grodt_orders_placed_total`
- `grodt.heartbeat.uptime` (gauge) becomes `grodt_heartbeat_uptime_seconds` (unit suffix added)
- `grodt.strategy.heartbeat` (gauge) becomes `grodt_strategy_heartbeat`
**Warning signs:** "No data" in panels when metric names look correct.

### Pitfall 2: Provisioning Path Mismatch in otel-lgtm
**What goes wrong:** Dashboard or alert files are mounted to wrong paths and Grafana ignores them.
**Why it happens:** The otel-lgtm image uses non-standard paths (`/otel-lgtm/grafana/conf/provisioning/`) instead of the typical Grafana paths (`/etc/grafana/provisioning/`).
**How to avoid:** Always mount to `/otel-lgtm/grafana/conf/provisioning/{dashboards,alerting}/`.
**Warning signs:** Grafana starts but shows no provisioned dashboards or alerts.

### Pitfall 3: Provisioned Resources Are Read-Only
**What goes wrong:** Developer tries to edit a provisioned dashboard in the Grafana UI and gets an error.
**Why it happens:** File-provisioned resources are immutable in the UI by design.
**How to avoid:** Document in the dashboard description that edits must go through the JSON file. Use "Save As" to create a copy for experimentation.
**Warning signs:** "Cannot save provisioned dashboard" error in UI.

### Pitfall 4: Template Variable Data Source UID
**What goes wrong:** Template variable query fails because the datasource UID doesn't match.
**Why it happens:** otel-lgtm pre-provisions data sources with specific UIDs that may differ from "prometheus" or "loki".
**How to avoid:** After first boot, check Grafana API (`/api/datasources`) or UI to confirm exact UIDs. The otel-lgtm image typically uses `prometheus` and `loki` as UIDs.
**Warning signs:** Template variable dropdown is empty.

### Pitfall 5: Alert Evaluation Requires Data
**What goes wrong:** Alert rules fire immediately on startup because there's no metric data yet.
**Why it happens:** `absent()` and `absent_over_time()` return 1 when no time series exists.
**How to avoid:** Set `for: 2m` (pending period) so alerts must be firing for 2 minutes before notifying. This prevents spurious alerts during cold start.
**Warning signs:** Slack notifications fire immediately after `docker compose up`.

### Pitfall 6: Slack Webhook URL as Environment Variable
**What goes wrong:** Alert contact point YAML contains `${SLACK_OPTION_ALERTS_WEBHOOK_URL}` but Grafana doesn't resolve it.
**Why it happens:** Grafana only expands `${}` env vars in provisioning files if the variable is available in the container's environment.
**How to avoid:** Pass the env var through docker-compose.yaml `environment:` section, sourced from the host `.env` file.
**Warning signs:** Slack notifications silently fail; check Grafana alerting logs.

## Code Examples

### PromQL Queries for Dashboard Panels

#### Row 1: System Health
```promql
# Go server heartbeat (stat panel -- last value)
grodt_heartbeat_uptime_seconds

# Python strategy heartbeat (stat panel -- last value)
grodt_strategy_heartbeat{strategy_name=~"$playground_id"}

# Active live playgrounds (stat panel)
grodt_heartbeat_active_playgrounds{environment="live"}
```

#### Row 2: Order Activity
```promql
# Orders placed rate (time series panel)
rate(grodt_orders_placed_total{environment=~"live|reconcile"}[5m])

# Orders filled rate (time series panel)
rate(grodt_orders_filled_total{environment=~"live|reconcile"}[5m])

# Orders rejected (stat panel -- total)
increase(grodt_orders_rejected_total{environment=~"live|reconcile"}[1h])
```

#### Row 3: Market Data
```promql
# Candles processed rate (time series panel)
rate(grodt_candles_processed_total[5m])

# Signals generated rate (time series panel)
rate(grodt_signals_generated_total[5m])

# Tick latency histogram (heatmap or histogram panel)
# Note: tick latency metric name depends on exact instrument name in metrics.go
```

#### Row 4: Positions
```promql
# Open orders gauge (stat panel)
grodt_heartbeat_open_orders
```

### LogQL Queries for Log Panels
```logql
# Recent order events (table panel)
{service_name="grodt-server"} | logfmt | event="order_placed" or event="order_filled" or event="order_rejected"

# Error logs (table panel)
{service_name=~"grodt.*"} | logfmt | level="error"

# Strategy signal decisions (table panel)
{service_name="grodt-strategy"} | logfmt | event=~"signal.*"
```

### Alert Rule PromQL

```promql
# ALERT-01: Go heartbeat stale
# Use absent_over_time to detect when the gauge stops being reported
absent_over_time(grodt_heartbeat_uptime_seconds[2m])

# ALERT-02: Python strategy heartbeat stale
absent_over_time(grodt_strategy_heartbeat[2m])

# ALERT-03: Error rate spike
# Count error-level logs over 5 minutes, alert if > 5 errors/min
sum(rate({service_name=~"grodt.*"} | logfmt | level="error" [5m])) > 0.083
# 0.083 = 5 errors per minute (5/60)
```

### Recommended Panel Types per Row

| Row | Panel | Type | Data Source |
|-----|-------|------|-------------|
| Health | Go Uptime | Stat | Prometheus |
| Health | Python Heartbeat | Stat | Prometheus |
| Health | Active Playgrounds | Stat | Prometheus |
| Orders | Orders Placed/Filled Rate | Time series | Prometheus |
| Orders | Orders Rejected | Stat | Prometheus |
| Orders | Recent Order Events | Table (logs) | Loki |
| Market Data | Candles Processed | Time series | Prometheus |
| Market Data | Signals Generated | Time series | Prometheus |
| Positions | Open Orders | Stat | Prometheus |
| Positions | Order Details | Table (logs) | Loki |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Grafana classic alerting | Unified alerting | Grafana 9 (2022), classic removed in 11 | Use unified alerting only |
| Dashboard JSON schema v1 | Dashboard JSON schema v2 (experimental) | Grafana 11.x | v1 is stable and well-documented; v2 is not yet GA. Use v1. |
| Separate alertmanager config | Grafana-managed alert rules | Grafana 9+ | Alert rules, contact points, policies all in one provisioning dir |

**Deprecated/outdated:**
- Classic alerting: Removed in Grafana 11. Do not use `"alert"` field in panel JSON.
- Legacy notification channels: Replaced by contact points in unified alerting.

## Open Questions

1. **Exact Prometheus metric names after OTel translation**
   - What we know: Dots become underscores; counters get `_total` suffix; units may be appended
   - What's unclear: Exact suffixes for each metric (e.g., does `grodt.heartbeat.uptime` with unit "s" become `grodt_heartbeat_uptime_seconds`?)
   - Recommendation: After starting the stack, query `{__name__=~"grodt.*"}` in Prometheus to confirm exact names. Build dashboard JSON after confirming.

2. **otel-lgtm data source UIDs**
   - What we know: The image pre-provisions Prometheus, Loki, Tempo data sources
   - What's unclear: Exact UIDs used (likely `prometheus`, `loki`, `tempo` but unconfirmed)
   - Recommendation: Check via `curl http://localhost:3000/api/datasources` after boot, or inspect the image's `grafana-datasources.yaml`.

3. **Slack webhook: reuse existing or new?**
   - What we know: `SLACK_OPTION_ALERTS_WEBHOOK_URL` exists in .env for trade alerts
   - What's unclear: Whether mixing observability alerts with trade alerts in the same channel is desirable
   - Recommendation: Reuse the existing webhook initially; can split later. Avoids needing a new webhook setup.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | Container runtime | Verified (otel-lgtm already runs) | -- | -- |
| otel-lgtm image | Grafana + backends | Verified (Phase 2 deployed it) | grafana/otel-lgtm:latest | -- |
| Grafana provisioning | Dashboard/alert auto-load | Built into otel-lgtm image | -- | -- |
| Slack webhook URL | ALERT notifications | Via .env (SLACK_OPTION_ALERTS_WEBHOOK_URL) | -- | Grafana UI notifications only |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None -- all dependencies are already deployed from Phase 2.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Manual validation (Grafana UI) + curl smoke tests |
| Config file | N/A -- no code tests for config-only phase |
| Quick run command | `curl -s http://localhost:3000/api/dashboards/uid/grodt-live-sim \| python3 -c "import sys,json; d=json.load(sys.stdin); print(d['dashboard']['title'])"` |
| Full suite command | `curl -s http://localhost:3000/api/ruler/grafana/api/v1/rules \| python3 -c "import sys,json; r=json.load(sys.stdin); print(json.dumps(r, indent=2))"` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DASH-01 | Dashboard shows aggregate live simulation activity | smoke | `curl -s http://localhost:3000/api/dashboards/uid/grodt-live-sim` returns 200 | N/A |
| DASH-02 | Dropdown filter for playground_id | smoke | Dashboard JSON contains `templating.list` with `playground_id` variable | N/A |
| DASH-03 | Position summary panel exists | smoke | Dashboard JSON contains panel with title matching "position" or "open orders" | N/A |
| ALERT-01 | Go heartbeat stale alert | smoke | `curl -s http://localhost:3000/api/ruler/grafana/api/v1/rules` contains "Go Server Heartbeat" | N/A |
| ALERT-02 | Python heartbeat stale alert | smoke | Same endpoint contains "Python Strategy Heartbeat" | N/A |
| ALERT-03 | Error rate alert | smoke | Same endpoint contains "Error Rate" | N/A |

### Sampling Rate
- **Per task commit:** Verify JSON/YAML syntax with `python3 -m json.tool` / `python3 -c "import yaml; yaml.safe_load(open(...))""`
- **Per wave merge:** `docker compose restart` and verify dashboard loads at `http://localhost:3000`
- **Phase gate:** All 6 curl smoke tests pass against running otel-lgtm instance

### Wave 0 Gaps
None -- this phase produces configuration files only. No test infrastructure needed beyond curl.

## Sources

### Primary (HIGH confidence)
- [Grafana otel-lgtm Docker image](https://github.com/grafana/docker-otel-lgtm) - provisioning paths, volume mounts, Dockerfile inspection
- [Grafana file-based alerting provisioning](https://grafana.com/docs/grafana/latest/alerting/set-up/provision-alerting-resources/file-provisioning/) - alert rule, contact point, notification policy YAML formats
- [Grafana dashboard provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/) - dashboard provider YAML format
- [Grafana provisioning-alerting-examples](https://github.com/grafana/provisioning-alerting-examples) - reference implementations

### Secondary (MEDIUM confidence)
- [Grafana Prometheus template variables](https://grafana.com/docs/grafana/latest/datasources/prometheus/template-variables/) - label_values() function for dropdown
- [OTel Prometheus naming](https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/) - dot-to-underscore conversion rules

### Tertiary (LOW confidence)
- Exact Prometheus metric names after OTel translation -- needs runtime verification
- otel-lgtm data source UIDs -- needs runtime verification

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - otel-lgtm is already deployed, Grafana provisioning is well-documented
- Architecture: HIGH - file structure and provisioning patterns are standard Grafana
- Pitfalls: HIGH - metric naming and path issues are well-known in OTel+Grafana stacks
- PromQL/LogQL queries: MEDIUM - metric names need runtime verification after OTel translation

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable -- Grafana provisioning format hasn't changed significantly in years)
