# Phase 5: End-to-End Tick Tracing - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Add W3C traceparent HTTP header propagation to Twirp RPC calls between Python client and Go server, so spans from both processes appear as parent-child in a single trace in Grafana/Tempo. The existing proto trace_id field is kept as a complementary searchable attribute.

</domain>

<decisions>
## Implementation Decisions

### Propagation Mechanism
- **D-01:** Add W3C traceparent HTTP header injection on the Python side (via OTel `inject()` propagator) alongside the existing proto `trace_id` field. Both coexist — HTTP headers create parent-child span links, proto field provides a searchable attribute.
- **D-02:** Go side extracts traceparent from HTTP headers. The Go OTel SDK already has the W3C propagator configured (Phase 2). Use `otelhttp` middleware on the Twirp HTTP handler to automatically extract and create child spans.
- **D-03:** Do NOT remove the proto `trace_id` field — it remains as a searchable attribute for log correlation.

### What Already Exists (from prior phases)
- Proto `trace_id` on NextTickRequest and PlaceOrderRequest (Phase 2)
- Go handlers extract `req.TraceId` and set as span attribute (Phase 2)
- Python `_get_trace_id()` sets trace_id on proto requests (Phase 4)
- Python `run_strategy()` creates "strategy.tick" parent spans in live mode (Phase 4)
- Go `SetupOTelSDK()` configures W3C TraceContext + Baggage propagator (Phase 2)

### What This Phase Adds
- Python: inject `traceparent` HTTP header on Twirp HTTP requests using OTel propagator
- Go: wrap Twirp HTTP handler with `otelhttp` middleware to extract traceparent and create child spans automatically

### Claude's Discretion
- How to inject headers into Twirp HTTP requests from Python (requests library hook, custom transport, or manual injection)
- Whether to use `otelhttp.NewHandler()` wrapper or manual header extraction on Go side
- Test strategy for verifying parent-child span linking

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Python Client (inject traceparent)
- `src/clients/python/engine/client.py` — BacktesterPlaygroundClient, `_get_trace_id()`, tick/place_order methods
- `src/clients/python/rpc/playground_twirp.py` — Generated Twirp client (HTTP transport layer)
- `src/clients/python/engine/otel.py` — setup_otel() with TracerProvider

### Go Server (extract traceparent)
- `src/go/backtester-api/rpc/twirp.go` — Twirp server setup (where otelhttp middleware goes)
- `src/go/backtester-api/router/grpc.go` — RPC handlers with existing trace_id extraction
- `src/go/utils/otel.go` — SetupOTelSDK with W3C propagator already configured
- `cmd/main.go` — Server startup where Twirp handler is created

### OTel Packages (already in go.mod)
- `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` — HTTP middleware (imported but not yet used)

</canonical_refs>

<code_context>
## Existing Code Insights

### Python Side
- Twirp Python client uses `requests` library for HTTP transport
- `playground_twirp.py` has `_make_request()` method that sets headers
- OTel Python SDK has `opentelemetry.propagate.inject()` to add traceparent to HTTP headers dict

### Go Side
- `otelhttp` package already in go.mod (v0.52.0) — just needs to be wired as middleware
- Twirp server handler is a standard `http.Handler` — wrapping with `otelhttp.NewHandler()` is straightforward
- W3C propagator already set globally in `SetupOTelSDK()`

### Integration Points
- Python: inject headers in the HTTP request before it's sent by the Twirp client
- Go: wrap Twirp handler with otelhttp in `rpc/twirp.go` or `cmd/main.go`

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-end-to-end-tick-tracing*
*Context gathered: 2026-03-26*
