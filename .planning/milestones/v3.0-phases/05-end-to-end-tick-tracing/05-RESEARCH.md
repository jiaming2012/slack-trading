# Phase 5: End-to-End Tick Tracing - Research

**Researched:** 2026-03-26
**Domain:** OpenTelemetry W3C trace context propagation across Python (requests/Twirp) and Go (otelhttp/Twirp)
**Confidence:** HIGH

## Summary

This phase connects two existing OTel instrumentation points -- Python "strategy.tick" spans (Phase 4) and Go RPC handler spans (Phase 2) -- into a single distributed trace via W3C traceparent HTTP header propagation. The Python client already creates parent spans in live mode and sets proto trace_id fields. The Go server already has the W3C propagator configured globally. The gap is: (1) Python does not inject traceparent HTTP headers on outgoing Twirp requests, and (2) Go does not extract traceparent from incoming HTTP headers on the Twirp handler.

Both sides are well-positioned for this. The twirpy Python client's `_make_request()` method accepts headers via `Context(headers={...})`, making injection straightforward. The Go Twirp handler is a standard `http.Handler`, so wrapping with `otelhttp.NewHandler()` (already in go.mod) is a single-line change.

**Primary recommendation:** Inject traceparent via `opentelemetry.propagate.inject()` into the twirp Context headers dict on the Python side; wrap the Twirp handler with `otelhttp.NewHandler()` on the Go side.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Add W3C traceparent HTTP header injection on the Python side (via OTel `inject()` propagator) alongside the existing proto `trace_id` field. Both coexist -- HTTP headers create parent-child span links, proto field provides a searchable attribute.
- D-02: Go side extracts traceparent from HTTP headers. The Go OTel SDK already has the W3C propagator configured (Phase 2). Use `otelhttp` middleware on the Twirp HTTP handler to automatically extract and create child spans.
- D-03: Do NOT remove the proto `trace_id` field -- it remains as a searchable attribute for log correlation.

### Claude's Discretion
- How to inject headers into Twirp HTTP requests from Python (requests library hook, custom transport, or manual injection)
- Whether to use `otelhttp.NewHandler()` wrapper or manual header extraction on Go side
- Test strategy for verifying parent-child span linking

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TICK-01 | Each client tick loop iteration is a single trace span | Already implemented in Phase 4 (trading_engine.py lines 120-128). This phase ensures the server spans appear as children. |
| TICK-02 | Client tick span includes attributes: playground_id, tick_number, symbols, duration_ms | Already implemented in Phase 4 (trading_engine.py lines 122-127). No changes needed. |
| TICK-03 | Server NextTick handler span is linked as child of client tick span via W3C traceparent | Requires: Python inject traceparent into Twirp HTTP headers + Go otelhttp middleware extracts and creates child span |
| TICK-04 | PlaceOrder RPC span is linked as child of client tick span when orders are placed | Same mechanism as TICK-03 -- inject traceparent on PlaceOrder calls too |
</phase_requirements>

## Standard Stack

### Core (already installed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| opentelemetry-api (Python) | 1.40.0 | OTel API including `propagate.inject()` | Already in grodt env |
| opentelemetry-sdk (Python) | 1.40.0 | TracerProvider with W3C propagator (default) | Already in grodt env |
| twirpy (Python) | installed | Twirp client with Context headers support | Already used for all RPC |
| go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp | v0.52.0 | HTTP middleware for auto trace extraction | Already in go.mod, not yet wired |
| go.opentelemetry.io/otel | v1.27.0 | Core OTel Go SDK with W3C propagator set globally | Already configured in utils.SetupOTelSDK |

### No New Dependencies Required

This phase requires zero new library installations on either side. Everything needed is already installed and partially configured.

## Architecture Patterns

### Pattern 1: Python Header Injection via Twirp Context

**What:** Use `opentelemetry.propagate.inject()` to write traceparent headers into a dict, then pass that dict to the twirpy `Context(headers=...)` constructor.

**When to use:** Every RPC call from BacktesterPlaygroundClient in live mode.

**Key insight:** The twirpy `TwirpClient._make_request()` method (line 15-16 of client.py) does `headers = ctx.get_headers()` and merges any additional headers from kwargs. The `Context` class accepts `headers={}` in its constructor. So the injection point is where `Context()` is constructed in `_network_call_with_retry_inner()`.

**Recommended approach:** Modify `_network_call_with_retry_inner()` in `engine/client.py` to inject traceparent headers into the Context before making the RPC call.

```python
from opentelemetry.propagate import inject

def _network_call_with_retry_inner(self, caller, client, request, backoff, max_backoff):
    retries = 0
    while True:
        try:
            # Inject W3C traceparent into headers for distributed tracing
            headers = {}
            inject(headers)  # Writes traceparent + tracestate if span is active
            response = client(
                ctx=Context(headers=headers),
                request=request
            )
            return response
        except TwirpServerException as e:
            # ... existing retry logic unchanged ...
```

**Why this approach (vs alternatives):**
- Requests library hook: Would require subclassing or monkey-patching the session. Over-engineered for this case.
- Manual header construction: Error-prone, duplicates what `inject()` does.
- Context headers: Clean, uses the existing twirpy API exactly as designed. `inject()` is a no-op when no span is active (backtest mode), so no conditional logic needed.

### Pattern 2: Go otelhttp Middleware Wrapping

**What:** Wrap the Twirp HTTP handler with `otelhttp.NewHandler()` in `rpc/twirp.go` so that incoming requests with traceparent headers automatically create child spans.

**Where:** `rpc/twirp.go` line 35, where `panicRecoveryMiddleware(twirpHandler)` is currently set.

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// In SetupTwirpServer:
mux.Handle(twirpHandler.PathPrefix(),
    otelhttp.NewHandler(
        panicRecoveryMiddleware(twirpHandler),
        "twirp",
    ),
)
```

**Why `otelhttp.NewHandler()` (vs manual extraction):**
- Auto-extracts traceparent from request headers using globally-set W3C propagator
- Creates a server span as child of the propagated parent
- Sets standard HTTP span attributes (method, route, status)
- The Twirp Go server internally uses `trace.SpanFromContext(ctx)` in handlers -- otelhttp populates the context correctly so existing `span.SetAttributes(...)` calls in grpc.go work unchanged
- Zero custom code needed

### Pattern 3: Span Nesting Architecture

The resulting trace tree in Grafana/Tempo will look like:

```
[Python] strategy.tick (parent)
  |-- [Python] client HTTP POST (injected traceparent)
  |     |-- [Go] twirp/NextTick (otelhttp child, extracted from traceparent)
  |           |-- [Go] existing handler spans...
  |-- [Python] client HTTP POST (if PlaceOrder called)
        |-- [Go] twirp/PlaceOrder (otelhttp child)
```

The proto `trace_id` field remains as a span attribute on the Go side for log correlation, but is NOT what creates the parent-child relationship. The W3C traceparent header is what links the spans.

### Anti-Patterns to Avoid
- **Do NOT conditionally inject headers based on live/simulator mode:** `inject()` is already a no-op when no span is active. The backtest path in trading_engine.py does not create spans, so inject will write nothing. No `if is_live:` guard needed.
- **Do NOT modify the generated playground_twirp.py:** It is auto-generated (`DO NOT EDIT!`). All changes go in `engine/client.py` where calls are made.
- **Do NOT remove proto trace_id:** Per D-03, it stays as a searchable attribute.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| W3C traceparent header format | Manual "00-{trace_id}-{span_id}-{flags}" string | `opentelemetry.propagate.inject()` | Handles version, sampling flags, tracestate automatically |
| HTTP header extraction + span creation on Go side | Manual `r.Header.Get("traceparent")` parsing | `otelhttp.NewHandler()` | Handles all W3C fields, creates proper server spans, sets HTTP semconv attributes |
| Trace context propagation format | Custom header scheme | W3C TraceContext standard | Industry standard, both OTel SDKs support natively |

## Common Pitfalls

### Pitfall 1: inject() Must Be Called Inside Active Span
**What goes wrong:** If `inject()` is called outside any span context, it writes nothing and the Go side creates an independent trace.
**Why it happens:** The call to `inject()` must happen while the "strategy.tick" span is the current span.
**How to avoid:** The injection happens in `_network_call_with_retry_inner()`, which is called from `tick()` and `place_order()`. In live mode, these are called inside the `with tracer.start_as_current_span("strategy.tick")` block in trading_engine.py (line 120-142). The span context propagates through the call stack automatically.
**Warning signs:** Go spans appear as root spans (no parent) in Grafana/Tempo.

### Pitfall 2: otelhttp Middleware Order Matters
**What goes wrong:** If otelhttp is inside panicRecoveryMiddleware, panics may not be caught properly.
**Why it happens:** Middleware wrapping order determines execution order.
**How to avoid:** otelhttp should be the outermost wrapper: `otelhttp.NewHandler(panicRecoveryMiddleware(twirpHandler), ...)`. This ensures trace context is extracted before any handler code runs, and panics are still caught.
**Warning signs:** Missing spans on panicked requests, or unrecovered panics.

### Pitfall 3: Duplicate Spans from otelhttp + Manual Tracer
**What goes wrong:** The Go handlers in grpc.go already call `trace.SpanFromContext(ctx)` and set attributes. otelhttp creates its own server span. This is expected and correct -- otelhttp creates an HTTP-level span, and the handler code operates on that same span (adding attributes to it).
**Why it's fine:** `trace.SpanFromContext(ctx)` returns the span that otelhttp put on the context. The handler enriches it, not duplicates it.
**Warning signs:** If you see TWO spans per RPC call (one HTTP, one manual), that means someone created a NEW span instead of getting the existing one.

### Pitfall 4: Headers Dict Must Be Mutable
**What goes wrong:** If an immutable mapping is passed to `inject()`, it silently fails.
**How to avoid:** Always pass a plain `dict()` -- the default `{}` works fine. The twirpy Context stores it by reference.

## Code Examples

### Python: Header Injection in client.py

```python
# Source: twirp/client.py line 15-16 shows ctx.get_headers() is merged into request headers
# Source: opentelemetry.propagate.inject() writes traceparent to carrier dict

from opentelemetry.propagate import inject

# In _network_call_with_retry_inner():
headers = {}
inject(headers)  # Adds traceparent, tracestate if span active; no-op otherwise
response = client(
    ctx=Context(headers=headers),
    request=request
)
```

### Go: otelhttp Middleware in rpc/twirp.go

```go
// Source: go.mod already has go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.52.0
// Source: utils/otel.go already sets W3C propagator globally

import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

mux.Handle(twirpHandler.PathPrefix(),
    otelhttp.NewHandler(
        panicRecoveryMiddleware(twirpHandler),
        "twirp",
    ),
)
```

### Verification: Check Trace Linkage in Tempo

After both sides are wired:
1. Start observability stack: `docker compose -f observability/docker-compose.yaml up -d`
2. Start Go server: `task app:dev`
3. Run a live strategy (or a short test with OTel enabled)
4. In Grafana Tempo, search for "strategy.tick" spans
5. Expand to see child spans: the Go "twirp" server span should appear nested under the Python span
6. Both spans share the same trace_id (32-char hex)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Proto trace_id field only (Phase 2) | W3C traceparent HTTP headers + proto trace_id | This phase | Python and Go spans linked as parent-child in Tempo |
| No trace propagation | Standard OTel propagation | This phase | Full distributed trace visibility |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (Python) | unittest (stdlib) |
| Framework (Go) | go test (stdlib) |
| Config file | None needed |
| Quick run command (Python) | `cd src/clients/python && /Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x -q` |
| Quick run command (Go) | `go test ./src/go/backtester-api/rpc/ -run TestOtelhttp -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TICK-01 | Tick loop iteration = single trace span | unit | Already tested in test_trading_engine_otel.py | Yes |
| TICK-02 | Span attributes (playground_id, tick_number, etc.) | unit | Already tested in test_trading_engine_otel.py | Yes |
| TICK-03 | NextTick server span is child of client tick span | unit | `python -m pytest tests/test_trace_propagation.py -x` | No - Wave 0 |
| TICK-04 | PlaceOrder server span is child of client tick span | unit | Same test file as TICK-03 | No - Wave 0 |

### Testing Strategy for Trace Propagation

**Unit test approach (Python side):** Verify that `inject()` is called with a headers dict and that the resulting Context contains traceparent headers. Mock the OTel tracer to produce a known trace_id/span_id, call `_network_call_with_retry_inner()` with a mocked client callable, and assert the Context headers contain a valid traceparent string.

```python
# Test pattern: verify inject() populates headers when span is active
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider

provider = TracerProvider()
trace.set_tracer_provider(provider)
tracer = trace.get_tracer("test")

with tracer.start_as_current_span("test_parent"):
    headers = {}
    inject(headers)
    assert "traceparent" in headers
    # Format: "00-{trace_id}-{span_id}-{flags}"
    parts = headers["traceparent"].split("-")
    assert len(parts) == 4
    assert parts[0] == "00"  # version
```

**Unit test approach (Go side):** Verify that the Twirp handler wrapped with otelhttp correctly extracts traceparent from incoming requests and creates a child span. Use `httptest.NewRequest` with a traceparent header and an in-memory span exporter.

### Sampling Rate
- **Per task commit:** Python: `cd src/clients/python && python -m pytest tests/ -x -q`
- **Per task commit:** Go: `task test`
- **Phase gate:** Both test suites green

### Wave 0 Gaps
- [ ] `tests/test_trace_propagation.py` -- covers TICK-03, TICK-04 (Python inject verification)
- [ ] `src/go/backtester-api/rpc/twirp_test.go` -- covers Go otelhttp extraction (optional, middleware is well-tested upstream)

## Open Questions

1. **otelhttp span naming**
   - What we know: `otelhttp.NewHandler(handler, "twirp")` will name spans "twirp". This is the operation name visible in Tempo.
   - What's unclear: Whether a more specific name like "PlaygroundService" would be better for filtering.
   - Recommendation: Use "twirp" for now. The HTTP route attribute (auto-set by otelhttp) provides the specific RPC method. Can refine naming later.

2. **tracestate header**
   - What we know: `inject()` also writes `tracestate` if baggage is set. Currently no baggage is used.
   - What's unclear: Whether tracestate will cause any issues with the twirpy client or Go extraction.
   - Recommendation: No action needed. Empty tracestate is fine; if present, otelhttp handles it.

## Sources

### Primary (HIGH confidence)
- Twirpy client.py source: `/Users/jamal/miniconda3/envs/grodt/lib/python3.10/site-packages/twirp/client.py` -- verified `_make_request()` merges ctx headers
- Twirpy context.py source: same package -- verified `Context(headers={})` constructor and `get_headers()` method
- `opentelemetry.propagate.inject()` -- verified API signature via Python help()
- Go go.mod -- verified `otelhttp v0.52.0` already present
- Go `utils/otel.go` -- verified W3C TraceContext propagator set globally (lines 48-52)
- `engine/client.py` -- verified `_network_call_with_retry_inner()` creates `Context()` on each call (line 158)
- `rpc/twirp.go` -- verified handler wrapping at line 35

### Secondary (MEDIUM confidence)
- OTel Python SDK 1.40.0 default propagator includes W3C TraceContext -- standard behavior since 1.0

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all packages already installed, versions verified from go.mod and pip
- Architecture: HIGH - both injection and extraction patterns are standard OTel usage, verified against actual source code
- Pitfalls: HIGH - based on direct code reading of twirpy client, otelhttp behavior, and existing handler patterns

**Research date:** 2026-03-26
**Valid until:** 2026-04-25 (stable -- no fast-moving dependencies)
