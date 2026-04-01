---
phase: quick
plan: 260401-fij
subsystem: trade-signals
tags: [signal-id, otel, grafana, diff-tests, audit-gaps]
dependency_graph:
  requires: []
  provides: [SIG-02, OBS-01, MIG-01]
  affects: [observability-dashboards, python-engine, backtester-rpc]
tech_stack:
  added: []
  patterns: [signal-id-forwarding, otel-attribute-alignment]
key_files:
  created: []
  modified:
    - src/clients/python/engine/client.py
    - src/go/backtester-api/router/grpc.go
    - src/clients/python/tests/test_mean_reversion_diff.py
decisions: []
metrics:
  duration: 1m10s
  completed: 2026-04-01
---

# Quick Task 260401-fij: Fix 3 v3.0 Audit Gaps Summary

Python place_order() gains signal_id parameter, WriteSignal OTel attribute renamed from signal_name to signal_type for Grafana dashboard alignment, and mean_reversion diff tests fixed by zeroing mock initial position.

## Completed Tasks

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add signal_id parameter to Python place_order() | 57cbed8 | src/clients/python/engine/client.py |
| 2 | Fix Grafana label mismatch in WriteSignal OTel attribute | 674b795 | src/go/backtester-api/router/grpc.go |
| 3 | Fix mean_reversion diff test mock playground | 28981cb | src/clients/python/tests/test_mean_reversion_diff.py |

## Task Details

### Task 1: Add signal_id parameter to Python place_order()

Added `signal_id: str = None` parameter to `place_order()` method after the existing `attributes` parameter. When provided, sets `request.signal_id` on the PlaceOrderRequest before RPC call. All existing callers unaffected since the parameter defaults to None.

### Task 2: Fix Grafana label mismatch in WriteSignal OTel attribute

Changed OTel attribute name from `signal_name` to `signal_type` on the `grodt.signals.generated` counter metric in WriteSignal handler. All three Grafana dashboards (grodt-live-simulation, grodt-mean-reversion, grodt-covered-call) filter on `signal_type`, so this 1-line fix aligns the emitted metric with dashboard queries.

### Task 3: Fix mean_reversion diff test mock playground

Changed mock `get_quantity` return value from 10000 to 0 in `_make_mock_playground()`. The value of 10000 simulated a massive existing position that triggered V2's `_max_shares_for_margin()` guard, blocking orders and causing false behavioral diffs. With zero initial position, V1 and V2 start from equal state and all 3 diff tests pass.

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

- `grep signal_id client.py`: Shows parameter in signature (line 639) and request assignment (lines 677-678)
- `grep signal_type grpc.go`: Shows corrected attribute at line 1586
- `pytest test_mean_reversion_diff.py`: 3/3 passed in 1.40s
- `go build ./src/go/...`: Clean build, no errors

## Known Stubs

None.
