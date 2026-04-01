---
phase: 23-replay-telemetry
plan: 02
subsystem: observability
tags: [otel, grafana, prometheus, alerting, python, heartbeat]

requires:
  - phase: 23-01
    provides: OTel metrics pipeline and Grafana dashboard foundation
provides:
  - DatasourceHeartbeat class for datasource liveness monitoring
  - Signal Consumption Rate and Datasource Heartbeat Grafana panels
  - Datasource heartbeat stale alert rule (5m threshold)
affects: [23-03, datasource-scripts]

tech-stack:
  added: []
  patterns: [datasource heartbeat daemon thread mirroring StrategyHeartbeat]

key-files:
  created:
    - src/clients/python/engine/datasource_heartbeat.py
  modified:
    - observability/dashboards/grodt-live-simulation.json
    - observability/alerting/alerting.yaml

key-decisions:
  - "5m staleness threshold for datasource heartbeat (10 missed 30s emissions balances responsiveness vs false positives)"
  - "Panel IDs 15-17 for new Signals & Datasources row and panels"

patterns-established:
  - "DatasourceHeartbeat: same daemon-thread pattern as StrategyHeartbeat but with datasource_name/symbol attributes and record_check() method"

requirements-completed: [OBS-01, OBS-02]

duration: 2min
completed: 2026-03-31
---

# Phase 23 Plan 02: Datasource Heartbeat & Signal Panels Summary

**DatasourceHeartbeat class emitting grodt.datasource.heartbeat gauge every 30s, Grafana signal panels, and datasource staleness alerting at 5m threshold**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-31T03:22:48Z
- **Completed:** 2026-03-31T03:24:27Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- DatasourceHeartbeat class with OTel gauge, daemon thread, and structured logging
- Signal Consumption Rate timeseries panel and Datasource Heartbeat stat panel in new dashboard row
- Datasource heartbeat stale alert rule firing after 5 minutes of absent metrics

## Task Commits

Each task was committed atomically:

1. **Task 1: DatasourceHeartbeat class** - `bf2006f` (feat)
2. **Task 2: Grafana signal panels and datasource heartbeat alert** - `3be2628` (feat)

## Files Created/Modified
- `src/clients/python/engine/datasource_heartbeat.py` - DatasourceHeartbeat class with OTel gauge grodt.datasource.heartbeat, daemon thread at 30s interval, record_check() method
- `observability/dashboards/grodt-live-simulation.json` - Added Signals & Datasources row with Signal Consumption Rate (timeseries) and Datasource Heartbeat (stat) panels; shifted Positions row down
- `observability/alerting/alerting.yaml` - Added datasource-heartbeat-stale alert rule (absent_over_time 5m, severity critical)

## Decisions Made
- 5m staleness threshold for datasource heartbeat alert (10 missed 30s heartbeats) -- balances responsiveness vs false positives for scripts that may have brief interruptions
- Used panel IDs 15-17 (row, signal consumption, datasource heartbeat) continuing from existing max ID 14

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- DatasourceHeartbeat class ready for import by datasource scripts in Phase 23-03
- Grafana dashboard and alerting rules ready for deployment
- No blockers for next plan

---
*Phase: 23-replay-telemetry*
*Completed: 2026-03-31*
