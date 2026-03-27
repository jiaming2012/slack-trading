---
phase: 04-python-telemetry-instrumentation
plan: 02
subsystem: python-strategy-telemetry
tags: [signal-logging, base-strategy, structured-logging, otel]
dependency_graph:
  requires: [04-01]
  provides: [SignalDecision-dataclass, decision-logging-methods]
  affects: [strategies/base_strategy.py, engine/types.py]
tech_stack:
  added: []
  patterns: [structured-logging-via-extra, otel-trace-id-extraction, env-var-gated-verbosity]
key_files:
  created:
    - src/clients/python/tests/test_signal_logging.py
  modified:
    - src/clients/python/engine/types.py
    - src/clients/python/strategies/base_strategy.py
decisions:
  - "Used module-level _signal_logger instead of class-level to keep it simple and avoid per-instance loggers"
  - "Auto-fill pattern: record_decision() fills trace_id/playground_id/symbol only when empty, preserving explicit values"
metrics:
  duration: 5min
  completed: 2026-03-27
---

# Phase 04 Plan 02: Signal Decision Logging Summary

SignalDecision dataclass in engine/types.py with structured decision logging wrapper in BaseStrategy, gated verbose indicator dump via STRATEGY_LOG_VERBOSE env var, auto-extracting trace_id from active OTel span.

## What Was Built

### SignalDecision Dataclass (engine/types.py)
Added `SignalDecision` dataclass with fields per D-03: `signal_type`, `direction`, `decision` (place/skip), `reason`, `symbol`, `playground_id`, `trace_id`, and optional `indicators` dict for verbose mode (D-04).

### Decision Logging Methods (strategies/base_strategy.py)
Three new methods on BaseStrategy:
- `record_decision(decision)` -- strategies call this inside on_tick() to record decisions. Auto-fills trace_id from OTel span, playground_id from self.playground.id, symbol from self.symbol.
- `_log_decision(decision)` -- emits structured log via `grodt.strategy.signal` logger. Strips `indicators` field unless `STRATEGY_LOG_VERBOSE=true` (D-02).
- `_flush_decisions()` -- logs all accumulated decisions and clears the list. Called by engine after on_tick() (Plan 03 wires this).

### Test Coverage (tests/test_signal_logging.py)
13 tests covering:
- SignalDecision dataclass construction and field defaults
- record_decision() auto-fill of trace_id, playground_id, symbol
- OTel trace_id extraction (mocked span context)
- _log_decision() verbose vs non-verbose indicator gating
- _flush_decisions() log-all-and-clear behavior
- "skip" and "place" decision structured log validation

## Commits

| Commit | Type | Description |
|--------|------|-------------|
| ba82c40 | test | Add failing tests for signal decision logging (TDD RED) |
| de7e5c1 | feat | Add SignalDecision dataclass and decision logging to BaseStrategy (TDD GREEN) |

## Deviations from Plan

None -- plan executed exactly as written.

## Test Results

- 13 new signal logging tests: all pass
- 318 total unit tests: all pass (excluding 2 pre-existing failures: test_credit_spread_strategy.py has a bug, test_demo_covered_call.py requires running server)

## Known Stubs

None -- all functionality is fully wired. Strategies can call `self.record_decision()` immediately. Engine-side `_flush_decisions()` call is documented as Plan 03 integration point.

## Self-Check: PASSED

All files exist. All commits verified.
