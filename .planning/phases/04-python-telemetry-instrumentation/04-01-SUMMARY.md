---
phase: 04-python-telemetry-instrumentation
plan: 01
subsystem: telemetry
tags: [opentelemetry, python, otel-sdk, heartbeat, metrics, tracing]

# Dependency graph
requires:
  - phase: 01-python-codebase-restructure
    provides: engine/ package structure with __init__.py
provides:
  - engine/otel.py with setup_otel() for TracerProvider + MeterProvider
  - engine/heartbeat.py with StrategyHeartbeat daemon thread
  - opentelemetry-exporter-otlp-proto-http==1.40.0 installed in grodt env
affects: [04-02, 04-03]

# Tech tracking
tech-stack:
  added: [opentelemetry-exporter-otlp-proto-http==1.40.0, opentelemetry-sdk==1.40.0, opentelemetry-api==1.40.0]
  patterns: [idempotent OTel setup with module-level guard, daemon thread heartbeat with Event wait pattern]

key-files:
  created:
    - src/clients/python/engine/otel.py
    - src/clients/python/engine/heartbeat.py
    - src/clients/python/tests/test_otel.py
    - src/clients/python/tests/test_heartbeat.py
  modified:
    - src/clients/python/requirements.txt
    - grodt.yml

key-decisions:
  - "Used _initialized module flag for idempotency guard rather than checking provider type"
  - "Force-reset OTel SDK internal _SET_ONCE locks in test fixtures to enable per-test isolation"
  - "Used threading.Event.wait(timeout=30) pattern for heartbeat loop (matches Go ticker pattern)"

patterns-established:
  - "OTel provider reset pattern: access _TRACER_PROVIDER_SET_ONCE._done for test cleanup"
  - "Heartbeat daemon thread: StrategyHeartbeat with start/stop/record_tick/set_state API"

requirements-completed: [PYTEL-01, PYTEL-02, BEAT-03, BEAT-04]

# Metrics
duration: 6min
completed: 2026-03-27
---

# Phase 04 Plan 01: Python OTel SDK Foundation Summary

**OTel Python SDK with OTLP HTTP exporters (no grpcio), idempotent setup_otel(), and StrategyHeartbeat daemon thread emitting gauge metric + structured log every 30s**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-27T00:55:31Z
- **Completed:** 2026-03-27T01:01:31Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Installed opentelemetry-exporter-otlp-proto-http==1.40.0 in grodt conda env (numpy 1.26.4 confirmed intact)
- Created engine/otel.py with idempotent setup_otel() creating TracerProvider + MeterProvider with OTLP HTTP exporters
- Created engine/heartbeat.py with StrategyHeartbeat daemon thread emitting gauge metric and structured log
- 13 unit tests covering OTel setup (7) and heartbeat lifecycle/state/logging (6)

## Task Commits

Each task was committed atomically:

1. **Task 1: Install OTel SDK and create engine/otel.py** - `c65f72b` (feat)
2. **Task 2: Create engine/heartbeat.py with StrategyHeartbeat** - `f559cce` (feat)

_Note: TDD tasks with RED/GREEN phases committed as single GREEN commits_

## Files Created/Modified
- `src/clients/python/engine/otel.py` - OTel SDK initialization with TracerProvider, MeterProvider, OTLP HTTP exporters
- `src/clients/python/engine/heartbeat.py` - StrategyHeartbeat daemon thread with gauge metric + structured log
- `src/clients/python/tests/test_otel.py` - 7 tests for setup_otel (provider types, idempotency, resource attributes, shutdown)
- `src/clients/python/tests/test_heartbeat.py` - 6 tests for heartbeat (construction, lifecycle, state, logging)
- `src/clients/python/requirements.txt` - Added opentelemetry-exporter-otlp-proto-http==1.40.0
- `grodt.yml` - Added opentelemetry-exporter-otlp-proto-http==1.40.0 to pip section

## Decisions Made
- Used module-level `_initialized` flag for idempotency rather than checking provider type (simpler, explicit)
- Force-reset OTel SDK internal `_SET_ONCE` locks in test fixtures to enable per-test provider isolation
- Used `threading.Event.wait(timeout=30)` pattern for heartbeat loop (matches Go ticker, single thread, clean shutdown)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- OTel SDK prevents overriding global providers once set (`_SET_ONCE` guard). Resolved by directly resetting internal `_TRACER_PROVIDER_SET_ONCE._done` flag in test fixtures for proper per-test isolation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- engine/otel.py ready for integration into trading_engine.run_strategy() (Plan 04-02)
- engine/heartbeat.py ready for integration into tick loop (Plan 04-02)
- OTel SDK importable, tracers and meters available for span/metric creation in Plan 04-02 and 04-03

---
*Phase: 04-python-telemetry-instrumentation*
*Completed: 2026-03-27*
