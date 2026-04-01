# Phase 4: Python Telemetry Instrumentation - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Install OTel Python SDK in the grodt conda env, instrument Python strategy clients with telemetry (signal decision logging, heartbeat, spans), and set trace_id on Twirp RPC requests from the active OTel span. This is where the "why was this trade placed or not placed?" question gets answered.

</domain>

<decisions>
## Implementation Decisions

### Signal Decision Logging
- **D-01:** Signal decisions are logged in `BaseStrategy.on_tick()` — every strategy gets logging automatically through the base class.
- **D-02:** Log verbosity is **configurable**: default to structured decision record (signal_type, direction, decision, reason), but allow full indicator dump via env var or config flag for debugging sessions.
- **D-03:** Structured record fields: signal_type, direction, decision (place/skip), reason, symbol, playground_id, trace_id.
- **D-04:** Full dump adds: all indicator values the strategy computed during evaluation (varies per strategy).
- **D-05:** "No action" decisions (below threshold, position full, spread too wide) are logged with explicit reason — this is the core observability gap being solved.

### OTel Python Setup
- **D-06:** OTel init code lives in a separate module `engine/otel.py` with a `setup_otel()` function.
- **D-07:** `setup_otel()` is called inside `trading_engine.run_strategy()` — automatic for every strategy run. Easy to port if engine expands.
- **D-08:** Use OTLP HTTP exporters only (`opentelemetry-exporter-otlp-proto-http`) — no grpcio to avoid numpy 1.26.4 conflict.
- **D-09:** trace_id from active OTel span is set on every NextTickRequest and PlaceOrderRequest sent via the Twirp client (`engine/client.py`).
- **D-10:** OTEL_* env vars configure the Python SDK (same as Go — OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME="grodt-strategy").

### Python Heartbeat
- **D-11:** Heartbeat runs in a background daemon thread with 30s timer — reports even when tick loop is blocked waiting for server.
- **D-12:** Heartbeat stats: strategy name, strategy state (active/idle), tick count, last tick time.
- **D-13:** Heartbeat is a metric gauge + structured log (same pattern as Go server heartbeat).
- **D-14:** Signal counts and position data are NOT in heartbeat — kept simple. Signal counts are separate metrics (from D-01 logging).

### Claude's Discretion
- OTel Python package versions (must be compatible with numpy 1.26.4)
- How to extract trace_id from active span in Python (`trace.get_current_span().get_span_context().trace_id`)
- Config mechanism for full indicator dump vs structured (env var name, default behavior)
- How BaseStrategy exposes indicator values for the full dump mode
- Heartbeat thread implementation details (threading.Timer vs threading.Event)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 1 Outputs (Python restructure)
- `src/clients/python/strategies/base_strategy.py` — BaseStrategy ABC with on_tick(), get_next_tick_seconds(), on_retrain(), etc.
- `src/clients/python/engine/trading_engine.py` — run_strategy() universal tick loop
- `src/clients/python/engine/client.py` — BacktesterPlaygroundClient (tick(), place_order() — where trace_id needs to be set)
- `src/clients/python/engine/types.py` — Merged type definitions

### Phase 2 Outputs (trace_id in proto)
- `src/clients/python/rpc/playground_pb2.py` — Python proto stubs with trace_id on NextTickRequest and PlaceOrderRequest
- `src/clients/python/rpc/playground_twirp.py` — Python Twirp client

### Strategy Files (where on_tick signal decisions happen)
- `src/clients/python/strategies/covered_call.py` — OptionsStrategyBasic
- `src/clients/python/strategies/wheel.py`
- `src/clients/python/strategies/pdf_wheel.py`
- `src/clients/python/strategies/mean_reversion.py`
- `src/clients/python/strategies/options_mean_reversion.py`
- `src/clients/python/strategies/credit_spread.py`

### Environment
- `grodt.yml` — Conda environment spec (add OTel packages here)
- `src/clients/python/requirements.txt` — Pip requirements (add OTel packages here too)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `engine/rpc_profiler.py` — existing RPC timing profiler, could be replaced/complemented by OTel spans
- `structlog` already in conda env (v25.1.0) but unused — OTel logging preferred for consistency with Go side
- `loguru` used in some scripts — decision logging should use OTel-compatible structured logging

### Integration Points
- `trading_engine.run_strategy()` is THE single entry point — OTel init goes here (D-07)
- `client.tick()` and `client.place_order()` are where trace_id gets set on requests (D-09)
- `BaseStrategy.on_tick()` is where signal decisions are made and logged (D-01)

### Existing Logging
- Current Python logging uses `loguru` in some files and `print()` in others — no consistent pattern
- Phase 4 establishes Python structured logging via OTel

</code_context>

<specifics>
## Specific Ideas

- The configurable verbosity (D-02) should default to structured in production, full dump when env var `STRATEGY_LOG_VERBOSE=true` is set
- trace_id should be automatically extracted from the current OTel span context — no manual passing needed
- Each strategy's on_tick() should return a decision object that BaseStrategy can log — keeps logging DRY

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-python-telemetry-instrumentation*
*Context gathered: 2026-03-26*
