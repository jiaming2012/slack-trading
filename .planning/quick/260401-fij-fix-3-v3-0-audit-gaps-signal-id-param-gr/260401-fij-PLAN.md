---
phase: quick
plan: 260401-fij
type: execute
wave: 1
depends_on: []
files_modified:
  - src/clients/python/engine/client.py
  - src/go/backtester-api/router/grpc.go
  - src/clients/python/tests/test_mean_reversion_diff.py
autonomous: true
requirements: [SIG-02, OBS-01, MIG-01]
must_haves:
  truths:
    - "Python place_order() accepts signal_id and forwards it to Go via RPC"
    - "Grafana signal panels show data when WriteSignal emits signals"
    - "All mean_reversion behavioral diff tests pass"
  artifacts:
    - path: "src/clients/python/engine/client.py"
      provides: "signal_id parameter on place_order()"
      contains: "signal_id"
    - path: "src/go/backtester-api/router/grpc.go"
      provides: "signal_type OTel attribute on WriteSignal counter"
      contains: "signal_type"
    - path: "src/clients/python/tests/test_mean_reversion_diff.py"
      provides: "Fixed mock playground with zero existing position"
      contains: "get_quantity"
  key_links:
    - from: "src/clients/python/engine/client.py"
      to: "grpc.go PlaceOrder handler"
      via: "Twirp RPC PlaceOrderRequest.signal_id"
      pattern: "request\\.signal_id"
    - from: "src/go/backtester-api/router/grpc.go"
      to: "observability/dashboards/grodt-live-simulation.json"
      via: "OTel attribute name matching Grafana query label"
      pattern: "signal_type"
---

<objective>
Close 3 gaps identified in the v3.0 milestone audit: SIG-02 (Python signal_id param), OBS-01 (Grafana label mismatch), and MIG-01 (mean_reversion diff test failure).

Purpose: Complete the v3.0 TradeSignal Framework milestone by resolving all partial requirement gaps.
Output: 3 files modified, all 3 requirements promoted from partial to satisfied.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/v3.0-MILESTONE-AUDIT.md
@src/clients/python/engine/client.py
@src/go/backtester-api/router/grpc.go
@src/clients/python/tests/test_mean_reversion_diff.py
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add signal_id parameter to Python place_order()</name>
  <files>src/clients/python/engine/client.py</files>
  <action>
At line 639 of client.py, add `signal_id: str = None` parameter to `place_order()` method signature, after the existing `attributes=None` parameter.

Then, after the existing block that sets `request.attributes` (around line 675), add:

```python
if signal_id is not None:
    request.signal_id = signal_id
```

This mirrors the pattern used for `close_order_id` (line 670-671) and `sl` (line 667-668). The Go side already parses `signal_id` from `PlaceOrderRequest` at grpc.go:1300-1306.

Do NOT modify any callers of place_order() -- signal_id defaults to None so all existing call sites remain valid.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && grep -n "signal_id" src/clients/python/engine/client.py | head -5</automated>
  </verify>
  <done>place_order() accepts signal_id parameter and sets it on the PlaceOrderRequest before RPC call. Existing callers unaffected.</done>
</task>

<task type="auto">
  <name>Task 2: Fix Grafana label mismatch in WriteSignal OTel attribute</name>
  <files>src/go/backtester-api/router/grpc.go</files>
  <action>
At grpc.go line 1586, change the OTel attribute name from `signal_name` to `signal_type`:

Before:
```go
attribute.String("signal_name", req.Name),
```

After:
```go
attribute.String("signal_type", req.Name),
```

This aligns the emitted OTel metric attribute with what Grafana dashboard queries expect (`signal_type`). The dashboards (grodt-live-simulation.json, grodt-mean-reversion.json, grodt-covered-call.json) all filter on `signal_type`. Renaming the Go attribute is a 1-line fix vs updating multiple dashboard JSON files.

Do NOT change the `symbol` attribute on line 1587 or the log message on line 1591.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && grep -n "signal_type\|signal_name" src/go/backtester-api/router/grpc.go | head -5</automated>
  </verify>
  <done>WriteSignal handler emits `signal_type` attribute on `grodt.signals.generated` counter, matching Grafana dashboard query labels.</done>
</task>

<task type="auto">
  <name>Task 3: Fix mean_reversion diff test mock playground</name>
  <files>src/clients/python/tests/test_mean_reversion_diff.py</files>
  <action>
The 2 failing diff tests in test_mean_reversion_diff.py fail because V2's `_max_shares_for_margin()` computes available shares as 0 when the mock playground has `get_quantity` returning 10000 (simulating a huge existing position). V1 has no such exposure check, so V1 places orders but V2 blocks them.

Fix: At line 38 of test_mean_reversion_diff.py, change the mock's `get_quantity` return value from 10000 to 0:

Before:
```python
pg.account.get_quantity = MagicMock(return_value=10000)
```

After:
```python
pg.account.get_quantity = MagicMock(return_value=0)
```

This creates an equal starting state (no existing position) for both V1 and V2, which is the correct baseline for behavioral diff testing. The intent of diff tests is to verify V1 and V2 produce identical orders given the same inputs -- NOT to test margin logic.

NOTE: The audit originally described this as "stale @patch() decorators" but investigation confirmed the @patch paths are already correct (they reference `deprecated.*` for V1 and `strategies.*` for V2). The actual root cause is the mock playground's unrealistic existing position size triggering V2's exposure limit guard.
  </action>
  <verify>
    <automated>cd /Users/jamal/projects/slack-trading && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest src/clients/python/tests/test_mean_reversion_diff.py -v 2>&1 | tail -10</automated>
  </verify>
  <done>All 3 mean_reversion behavioral diff tests pass. V1 and V2 produce identical orders from zero-position starting state.</done>
</task>

</tasks>

<verification>
All 3 gaps resolved:
1. `grep "signal_id" src/clients/python/engine/client.py` shows parameter in place_order signature and request assignment
2. `grep "signal_type" src/go/backtester-api/router/grpc.go` shows corrected OTel attribute
3. `python -m pytest src/clients/python/tests/test_mean_reversion_diff.py -v` shows 3/3 pass
4. `go build ./src/go/...` compiles without errors
</verification>

<success_criteria>
- Python place_order() has signal_id parameter that forwards to RPC request
- grpc.go WriteSignal emits signal_type (not signal_name) on counter metric
- All 3 mean_reversion diff tests pass
- Go builds cleanly
- No other tests broken
</success_criteria>

<output>
After completion, create `.planning/quick/260401-fij-fix-3-v3-0-audit-gaps-signal-id-param-gr/260401-fij-SUMMARY.md`
</output>
