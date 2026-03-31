---
phase: 26-python-datasource-wiring-observability
plan: 01
subsystem: datasources
tags: [python, datasource, heartbeat, otel, writeSignal, twirp, argparse]

# Dependency graph
requires:
  - phase: 24-signal-rpc-endpoints
    provides: "write_signal() on BacktesterPlaygroundClient and WriteSignal Twirp RPC"
  - phase: 23-replay-telemetry
    provides: "DatasourceHeartbeat class in engine/datasource_heartbeat.py"
  - phase: 25-complete-strategy-migrations
    provides: "All 6 datasource modules with produce_signals()/produce_open_signals()"
provides:
  - "__main__ blocks on all 6 datasource scripts for standalone execution"
  - "DatasourceHeartbeat integration in every datasource for OBS-02 alerting"
  - "WriteSignal RPC wiring pattern for live signal production"
affects: [production-deployment, live-trading-setup]

# Tech tracking
tech-stack:
  added: []
  patterns: [standalone-datasource-main-block, datasource-heartbeat-lifecycle]

key-files:
  created: []
  modified:
    - src/clients/python/datasources/ma_crossover.py
    - src/clients/python/datasources/options_ma_crossover.py
    - src/clients/python/datasources/credit_spread_signals.py
    - src/clients/python/datasources/covered_call_signals.py
    - src/clients/python/datasources/wheel_signals.py
    - src/clients/python/datasources/pdf_wheel_signals.py

key-decisions:
  - "Used PlaygroundServiceClient directly instead of BacktesterPlaygroundClient for standalone __main__ (avoids complex constructor that requires CreatePolygonPlaygroundRequest)"
  - "Callable-based datasources (covered_call, wheel) log placeholder instead of calling produce_open_signals in __main__ since feature_vector_fn requires live candle DataFrame"

patterns-established:
  - "Standalone datasource pattern: argparse CLI + DatasourceHeartbeat.start() + loop(produce_signals + WriteSignal + record_check + sleep) + KeyboardInterrupt handler"
  - "Bar-dict datasources use empty stub data with TODO for Polygon wiring; callable-based datasources log debug message"

requirements-completed: [DS-03, OBS-02]

# Metrics
duration: 3min
completed: 2026-03-31
---

# Phase 26 Plan 01: Python Datasource Wiring & Observability Summary

**All 6 datasource scripts wired with standalone __main__ blocks using DatasourceHeartbeat gauge and WriteSignal RPC for live signal production**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-31T11:56:25Z
- **Completed:** 2026-03-31T11:59:42Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- All 6 datasource scripts runnable standalone via `python -m datasources.<module> --symbol AAPL`
- DatasourceHeartbeat instantiated in every __main__ block, emitting grodt.datasource.heartbeat gauge per OBS-02
- WriteSignal RPC pattern established for live signal production via Twirp
- Sim-mode imports (produce_signals/produce_open_signals) verified unbroken for all 6 modules

## Task Commits

Each task was committed atomically:

1. **Task 1: Add __main__ block to ma_crossover.py** - `39da6fb` (feat)
2. **Task 2: Add __main__ blocks to remaining 5 datasource scripts** - `2509722` (feat)

## Files Created/Modified
- `src/clients/python/datasources/ma_crossover.py` - Reference __main__ with argparse, DatasourceHeartbeat, WriteSignal RPC loop
- `src/clients/python/datasources/options_ma_crossover.py` - __main__ block following ma_crossover pattern
- `src/clients/python/datasources/credit_spread_signals.py` - __main__ block with direction attribute serialization
- `src/clients/python/datasources/covered_call_signals.py` - __main__ block with TODO for live feature_vector_fn wiring
- `src/clients/python/datasources/wheel_signals.py` - __main__ block with TODO for live feature_vector_fn wiring
- `src/clients/python/datasources/pdf_wheel_signals.py` - __main__ block with compound key and daily_signals stub

## Decisions Made
- Used PlaygroundServiceClient directly in __main__ instead of BacktesterPlaygroundClient -- the full client constructor requires CreatePolygonPlaygroundRequest and other sim-specific params; standalone datasources only need WriteSignal RPC
- Callable-based datasources (covered_call, wheel) emit heartbeat only in __main__ with a TODO for live data wiring, since feature_vector_fn requires a candle DataFrame with supertrend indicators that only exist in the backtester tick loop

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

All __main__ blocks contain intentional stubs for live data fetching (marked with TODO comments). These are expected -- the plan explicitly calls for placeholder/stub data since real live data fetching (Polygon streaming/polling) is out of scope for this phase:

1. `src/clients/python/datasources/ma_crossover.py:89` - `bar_dict = {}; pdf = None` (TODO: wire Polygon)
2. `src/clients/python/datasources/options_ma_crossover.py:97` - `bar_dict = {}; pdf = None` (TODO: wire Polygon)
3. `src/clients/python/datasources/credit_spread_signals.py:91` - `bar_dict = {}; pdf = None` (TODO: wire Polygon)
4. `src/clients/python/datasources/covered_call_signals.py:87` - Logs "live data source not yet wired" (TODO: wire Polygon candle DataFrame)
5. `src/clients/python/datasources/wheel_signals.py:87` - Logs "live data source not yet wired" (TODO: wire Polygon candle DataFrame)
6. `src/clients/python/datasources/pdf_wheel_signals.py:110` - `bar_dict = {}; pdf = None; daily_signals = {}` (TODO: wire Polygon)

These stubs are intentional per the plan and do not prevent DS-03/OBS-02 completion -- the heartbeat and RPC wiring pattern is the deliverable.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- DS-03 and OBS-02 requirements satisfied, closing the v3.0 TradeSignal Framework milestone
- Live data fetching (Polygon REST/WebSocket) can be wired into the __main__ blocks in a future phase
- All datasources are independently deployable as standalone processes

---
*Phase: 26-python-datasource-wiring-observability*
*Completed: 2026-03-31*
