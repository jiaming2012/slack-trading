---
phase: 09-playground-filtering-detail-panels
plan: 01
subsystem: infra
tags: [grafana, dashboard, prometheus, loki, client_id, filtering]

requires:
  - phase: 08-metric-labels-bug-fix
    provides: client_id label on all Go OTel metrics via PlaygroundAttrs
provides:
  - client_id template variable dropdown for dashboard filtering
  - Active Playgrounds table panel showing client_id + playground_id pairs
  - Open Orders detail table panel from Loki order_placed logs
  - traceID in Recent Order Events and Position Details log line formats
  - client_id filter on Orders Placed/Filled and Orders Rejected panels
affects: [10-strategy-dashboard-signals, per-strategy dashboards]

tech-stack:
  added: []
  patterns:
    - "Grafana table panels with fieldConfig overrides to hide unwanted columns"
    - "Loki line_format with traceID for trace correlation in log panels"
    - "Multi-variable dashboard filtering (client_id + playground_id)"

key-files:
  created: []
  modified:
    - observability/dashboards/grodt-live-simulation.json

key-decisions:
  - "client_id variable placed before playground_id in template list for primary filtering"
  - "Active Playgrounds uses grodt_candles_processed_total with max-by aggregation to enumerate live pairs"
  - "Open Orders panel switched from Prometheus heartbeat metric to Loki order_placed logs for per-order detail"

patterns-established:
  - "Dashboard variable cascading: client_id filters first, playground_id filters second"
  - "Column hiding via fieldConfig.overrides with byName matcher and custom.hidden property"

requirements-completed: [LABEL-02, PANEL-01, PANEL-03, PANEL-05, PANEL-06]

duration: 2min
completed: 2026-03-29
---

# Phase 09 Plan 01: Playground Filtering and Detail Panels Summary

**Grafana dashboard with client_id dropdown filter, Active Playgrounds table, Open Orders detail from Loki, and traceID in order/position log panels**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-29T18:59:00Z
- **Completed:** 2026-03-29T19:01:02Z
- **Tasks:** 2 (1 auto + 1 informational checkpoint)
- **Files modified:** 1

## Accomplishments
- Added client_id template variable enabling dashboard-wide filtering by client
- Converted Active Playgrounds from simple count stat to table showing all client_id/playground_id pairs
- Converted Open Orders from Prometheus heartbeat stat to Loki-powered table with per-order symbol, side, quantity detail
- Added traceID to Recent Order Events and Position Details log line formats for trace correlation
- Added client_id filter to Orders Placed/Filled and Orders Rejected panel queries

## Task Commits

Each task was committed atomically:

1. **Task 1: Update dashboard JSON with client_id filter and enriched panels** - `a075374` (feat)

**Plan metadata:** (pending docs commit)

## Files Created/Modified
- `observability/dashboards/grodt-live-simulation.json` - Updated Grafana dashboard with 5 panel/filter changes

## Decisions Made
- Placed client_id variable before playground_id in template list so it appears first in the dashboard dropdown row
- Used `max by (client_id, playground_id)` on grodt_candles_processed_total as the source for Active Playgrounds table -- shows any playground that has ever processed candles
- Switched Open Orders panel from Prometheus grodt_heartbeat_open_orders metric to Loki order_placed log query for richer per-order detail

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Checkpoint Note

Task 2 (checkpoint:human-verify) was treated as informational per execution context. Visual verification in Grafana UI is recommended after deployment:
1. Restart otel-lgtm container: `cd observability && docker compose up -d`
2. Open Grafana at http://localhost:3000
3. Navigate to "Grodt Live Simulation" dashboard
4. Verify: Client dropdown, Active Playgrounds table, Open Orders table, traceID in log panels

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Dashboard filtering and detail panels complete
- Ready for Phase 10 (strategy dashboard signals) which is independent of this plan
- Visual verification pending -- structural correctness validated via automated JSON checks

---
*Phase: 09-playground-filtering-detail-panels*
*Completed: 2026-03-29*
