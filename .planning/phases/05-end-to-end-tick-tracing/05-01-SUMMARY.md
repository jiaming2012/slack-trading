---
phase: 05-end-to-end-tick-tracing
plan: 01
subsystem: observability
tags: [otel, tracing, twirp, python, go]
dependency_graph:
  requires: [04-03]
  provides: [w3c-traceparent-propagation, otelhttp-twirp-middleware]
  affects: [src/clients/python/engine/client.py, src/go/backtester-api/rpc/twirp.go]
tech_stack:
  added: [otelhttp-middleware]
  patterns: [w3c-trace-context, opentelemetry-propagation]
key_files:
  created:
    - src/clients/python/tests/test_trace_propagation.py
  modified:
    - src/clients/python/engine/client.py
    - src/go/backtester-api/rpc/twirp.go
decisions:
  - "inject() called once before retry loop (not per retry) since trace context is fixed for the request"
  - "otelhttp is outermost middleware wrapper (outside panicRecoveryMiddleware) per research Pitfall 2"
metrics:
  duration: 4min
  completed: 2026-03-27
---

# Phase 05 Plan 01: End-to-End Tick Tracing Summary

W3C traceparent header propagation between Python Twirp client and Go Twirp server via opentelemetry.propagate.inject() and otelhttp.NewHandler middleware

## What Was Done

### Task 1: Python traceparent injection (TDD)

**RED:** Created `src/clients/python/tests/test_trace_propagation.py` with 3 unit tests:
- `test_traceparent_injected_when_span_active` -- verifies traceparent header present in Context when OTel span active
- `test_no_traceparent_when_no_span_active` -- verifies no traceparent when no span (backtest mode)
- `test_traceparent_format_valid_w3c` -- validates 00-{32hex}-{16hex}-{2hex} format

**GREEN:** Modified `src/clients/python/engine/client.py`:
- Added `from opentelemetry.propagate import inject` import
- In `_network_call_with_retry_inner()`, create `headers = {}`, call `inject(headers)`, pass `Context(headers=headers)`
- Total change: 4 lines added, 1 line modified
- Existing `trace_id=_get_trace_id()` in `tick()` and `place_order()` preserved (D-03 coexistence)

### Task 2: Go otelhttp middleware on Twirp handler

Modified `src/go/backtester-api/rpc/twirp.go`:
- Added `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` import (already in go.mod v0.52.0)
- Wrapped Twirp handler: `otelhttp.NewHandler(panicRecoveryMiddleware(twirpHandler), "twirp")`
- otelhttp is outermost (extracts trace context from HTTP headers before handler code runs)
- panicRecoveryMiddleware preserved as inner middleware (panic recovery still works)

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- Python tests: 3/3 passed
- Go build (`go build ./src/go/...`): success
- Go router tests: pass
- Existing Python OTel tests (23 tests): all pass
- Acceptance criteria: all 5 grep checks pass

## Known Stubs

None.

## Self-Check: PASSED
