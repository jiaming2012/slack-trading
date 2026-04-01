---
phase: 04-python-telemetry-instrumentation
plan: 03
subsystem: telemetry
tags: [opentelemetry, python, tracing, heartbeat, twirp]

# Dependency graph
requires:
  - phase: 04-01
    provides: setup_otel(), StrategyHeartbeat
  - phase: 04-02
    provides: _flush_decisions() on BaseStrategy
provides:
  - Instrumented run_strategy() with OTel spans, heartbeat, decision flushing
  - trace_id injection on all Twirp RPC requests (NextTick, PlaceOrder)
affects: [05-grafana-dashboards, 06-go-python-trace-linking]

# Tech tracking
tech-stack:
  added: []
  patterns: [live-only span creation, trace_id propagation via protobuf field]

key-files:
  modified:
    - src/clients/python/engine/client.py
    - src/clients/python/engine/trading_engine.py
    - src/clients/python/tests/test_otel.py

key-decisions:
  - "Used hasattr guard for _flush_decisions() since Plan 02 may execute in parallel"
  - "Live-only spans to avoid 500K+ span explosion in backtest mode (Pitfall 3)"

patterns-established:
  - "trace_id extraction via format(ctx.trace_id, '032x') for 32-char hex consistency"
  - "is_live guard pattern for OTel span creation in tick loops"

requirements-completed: [PYTEL-03, PYTEL-04]

# Metrics
duration: 6min
completed: 2026-03-27
---

# Phase 04 Plan 03: Engine OTel Wiring Summary

**OTel spans wired into tick loop (live mode only), trace_id injected on all Twirp RPC requests via _get_trace_id() helper**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-27T01:08:32Z
- **Completed:** 2026-03-27T01:15:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added _get_trace_id() helper that extracts 32-char hex trace ID from active OTel span
- Injected trace_id on NextTickRequest (tick + preview_tick) and PlaceOrderRequest
- Instrumented run_strategy() with setup_otel(), StrategyHeartbeat lifecycle, and per-tick OTel spans (live mode only)
- Added _flush_decisions() call after each on_tick() for signal logging integration
- try/finally ensures clean shutdown of heartbeat and OTel SDK

## Task Commits

Each task was committed atomically:

1. **Task 1: Add _get_trace_id() helper and set trace_id on RPC requests** - `c23920d` (feat)
2. **Task 2: Instrument run_strategy() with OTel spans, heartbeat, and decision flushing** - `28f90e1` (feat)

## Files Created/Modified
- `src/clients/python/engine/client.py` - Added _get_trace_id() helper, trace_id on 3 RPC request constructors
- `src/clients/python/engine/trading_engine.py` - setup_otel(), heartbeat lifecycle, live-only spans, _flush_decisions()
- `src/clients/python/tests/test_otel.py` - 3 new tests for trace ID extraction format and behavior

## Decisions Made
- Used `hasattr(strategy, '_flush_decisions')` guard since Plan 02 (which adds this method) may execute in parallel
- Live-only span creation avoids 500K+ span explosion in backtest/simulator mode (Pitfall 3)
- Code duplication between live and simulator tick paths is intentional for clean span context management

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added hasattr guard for _flush_decisions()**
- **Found during:** Task 2 (Instrument run_strategy)
- **Issue:** Plan 02 (which adds _flush_decisions to BaseStrategy) executes in parallel; method may not exist at merge time
- **Fix:** Used `hasattr(strategy, '_flush_decisions')` conditional call instead of direct call
- **Files modified:** src/clients/python/engine/trading_engine.py
- **Verification:** Tests pass with and without _flush_decisions on strategy
- **Committed in:** 28f90e1 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Guard ensures compatibility regardless of merge order. No scope creep.

## Issues Encountered
- Pre-existing test failures in test_credit_spread_strategy.py and test_demo_covered_call.py (not caused by this plan's changes, confirmed via git stash comparison)

## Known Stubs
None - all functionality is fully wired.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Python strategy execution is now fully instrumented with OTel traces
- trace_id flows from Python spans to Go server via protobuf field
- Ready for Grafana dashboard integration and cross-service trace linking

---
*Phase: 04-python-telemetry-instrumentation*
*Completed: 2026-03-27*
