---
phase: 03-go-telemetry-instrumentation
plan: 03
subsystem: telemetry
tags: [otel, heartbeat, gauge-metrics, logrus, goroutine]

# Dependency graph
requires:
  - phase: 03-01
    provides: "telemetry.Init(), ActivePlaygrounds/OpenOrders/UptimeSeconds gauges, ShouldEmitOrderTelemetry"
provides:
  - "StartHeartbeat goroutine emitting OTel gauge metrics every 30s"
  - "HeartbeatStats struct and StatsProvider type for pluggable stats collection"
  - "EmitHeartbeatLog structured log with event=heartbeat field"
  - "DatabaseService.GetHeartbeatStats method for mutex-safe playground iteration"
affects: [grafana-dashboard, alerting, python-heartbeat]

# Tech tracking
tech-stack:
  added: []
  patterns: [StatsProvider callback pattern for testable goroutine, gauge metrics segmented by environment attribute]

key-files:
  created:
    - src/go/telemetry/heartbeat.go
    - src/go/telemetry/heartbeat_test.go
  modified:
    - cmd/main.go
    - src/go/data/database_service.go

key-decisions:
  - "Used StatsProvider callback function (not interface) for heartbeat stats -- simpler, testable, avoids import cycle"
  - "Separated EmitHeartbeatLog as exported function for unit-testable structured log output"
  - "Environment-only segmentation on gauge metrics in v1 (account_type detail in structured log)"

patterns-established:
  - "StatsProvider callback: heartbeat accepts func() HeartbeatStats, testable without DatabaseService dependency"
  - "Nil-guard on OTel instruments: check ActivePlaygrounds != nil before recording (safe when OTel SDK not initialized)"

requirements-completed: [BEAT-01, BEAT-02]

# Metrics
duration: 5min
completed: 2026-03-26
---

# Phase 03 Plan 03: Server Heartbeat Summary

**30-second heartbeat goroutine with OTel gauge metrics (active playgrounds, open orders, uptime) and structured log, wired via DatabaseService stats provider**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-26T21:20:01Z
- **Completed:** 2026-03-26T21:25:00Z
- **Tasks:** 2 (Task 1 was TDD with RED/GREEN commits)
- **Files modified:** 4

## Accomplishments
- Heartbeat goroutine emits OTel gauge metrics every 30 seconds segmented by environment (live, reconcile, simulator)
- Structured log line with event=heartbeat includes playground counts, open orders, uptime, and last tick time
- DatabaseService.GetHeartbeatStats iterates playground map under mutex, counting by environment and open order status
- Goroutine stops cleanly when context is cancelled (SIGTERM-safe)

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing heartbeat tests** - `c35f1a9` (test)
2. **Task 1 GREEN: Heartbeat implementation** - `a33055c` (feat)
3. **Task 2: Wire heartbeat into main with DatabaseService** - `c455cdb` (feat)

## Files Created/Modified
- `src/go/telemetry/heartbeat.go` - HeartbeatStats, StatsProvider, StartHeartbeat, EmitHeartbeatLog, emitHeartbeatMetrics
- `src/go/telemetry/heartbeat_test.go` - Tests for stats, structured log output, goroutine cancellation
- `cmd/main.go` - Launch heartbeat goroutine with dbService.GetHeartbeatStats
- `src/go/data/database_service.go` - GetHeartbeatStats method (mutex-safe playground iteration)

## Decisions Made
- Used `StatsProvider func() HeartbeatStats` callback instead of interface to avoid import cycles between telemetry and data packages
- Separated `EmitHeartbeatLog` as an exported function so structured log output is directly unit-testable
- Segmented ActivePlaygrounds gauge by environment attribute only in v1; account_type detail available in structured log

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing test failures in `src/go/backtester-api/models/` (TestSymbol, TestCalendar, etc.) -- verified these fail identically without any heartbeat changes. Not related to this plan.

## Known Stubs

None - all functionality is fully wired.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Heartbeat metrics ready for Grafana dashboard visualization
- Heartbeat log ready for Loki alerting on stale heartbeat
- Python client heartbeat can follow the same StatsProvider pattern

## Self-Check: PASSED

- All created files exist on disk
- All commit hashes (c35f1a9, a33055c, c455cdb) found in git log
- All builds succeed, all telemetry tests pass

---
*Phase: 03-go-telemetry-instrumentation*
*Completed: 2026-03-26*
