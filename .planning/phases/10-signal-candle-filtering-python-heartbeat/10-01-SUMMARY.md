---
phase: 10-signal-candle-filtering-python-heartbeat
plan: 01
subsystem: dashboard, telemetry, heartbeat
tags: [grafana, otel, metrics, filtering]
dependency_graph:
  requires: [08-01]
  provides: [LABEL-03, LABEL-04, PANEL-02]
  affects: [observability, dashboard]
tech_stack:
  added: []
  patterns: [per-symbol metric attributes, per-playground heartbeat dimensions, Grafana template variables]
key_files:
  created: []
  modified:
    - src/go/backtester-api/models/playground.go
    - src/clients/python/engine/heartbeat.py
    - src/clients/python/engine/client.py
    - src/clients/python/engine/trading_engine.py
    - observability/dashboards/grodt-live-simulation.json
decisions:
  - Heartbeat gauge emits per-playground rows (not summed) so table panel shows individual playground status
  - Symbol attribute added at CandlesProcessed emission site using candle.Symbol.GetTicker()
  - client_id stored on BacktesterPlaygroundClient via getattr for safe access
metrics:
  duration: 2m24s
  completed: 2026-03-29
---

# Phase 10 Plan 01: Signal/Candle Filtering and Python Heartbeat Summary

Signal_type and symbol dropdown filters on dashboard panels, plus per-playground heartbeat table showing client_id, strategy_name, and state.

## What Was Done

### Task 1: Add symbol attribute to CandlesProcessed metric and store client_id on Python client
- Added `attribute.String("symbol", candle.Symbol.GetTicker())` to CandlesProcessed metric emission in playground.go liveTick method
- Added `metric` and `attribute` OTel imports to playground.go
- Stored `self.client_id` on BacktesterPlaygroundClient from request object

**Commit:** 1858453

### Task 2: Update Python heartbeat with per-playground dimensions
- Extended StrategyHeartbeat constructor to accept `playground_id` and `client_id` parameters
- Added playground_id and client_id to gauge attributes and structured log output
- Updated run_strategy in trading_engine.py to pass playground.id and playground.client_id to heartbeat

**Commit:** 2b1b195

### Task 3: Add template variables and update dashboard panels
- Added `signal_type` template variable sourced from `grodt_signals_generated_total`
- Added `symbol` template variable sourced from `grodt_candles_processed_total`
- Updated Signals Generated panel: query filters by `signal_type=~"$signal_type"` with per-type legend (`{{signal_type}}`)
- Updated Candles Processed panel: query filters by `symbol=~"$symbol"` with per-symbol legend (`{{symbol}}`)
- Converted Python Strategy Heartbeat panel from `stat` type to `table` type with per-playground rows, hidden metadata columns (Time, __name__, Value, job, instance), sortable by client_id

**Commit:** bfc64e2

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

1. `go build ./src/go/backtester-api/models/` -- PASSED (compiles without errors)
2. Dashboard JSON valid with all 4 template variables (client_id, playground_id, signal_type, symbol) -- PASSED
3. Python heartbeat constructor accepts playground_id and client_id -- PASSED
4. Heartbeat panel type is "table" (not "stat") -- PASSED
5. Signals Generated query includes `signal_type=~"$signal_type"` filter -- PASSED
6. Candles Processed query includes `symbol=~"$symbol"` filter -- PASSED

## Known Stubs

None -- all data sources are wired to live metrics.

## Self-Check: PASSED
