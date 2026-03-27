# Phase 4: Python Telemetry Instrumentation - Research

**Researched:** 2026-03-26
**Domain:** OpenTelemetry Python SDK, strategy signal logging, heartbeat instrumentation
**Confidence:** HIGH

## Summary

Phase 4 adds OpenTelemetry instrumentation to the Python strategy clients. The core work is: (1) install OTel Python SDK in the grodt conda env, (2) create an `engine/otel.py` setup module mirroring the Go `utils.SetupOTelSDK`, (3) instrument `BaseStrategy.on_tick()` with structured signal decision logging, (4) add a background daemon thread heartbeat, and (5) set `trace_id` on Twirp RPC requests from the active OTel span.

The Go server already reads `trace_id` from the protobuf message fields (not HTTP headers), so the Python side just needs to extract the trace ID from the current OTel span context and set it on `NextTickRequest.trace_id` and `PlaceOrderRequest.trace_id` before sending. The `twirp` Python library's `Context` object supports custom headers but those are not needed since trace correlation uses protobuf fields.

**Primary recommendation:** Install `opentelemetry-exporter-otlp-proto-http==1.40.0` (pulls in SDK, API, proto packages automatically). Use OTLP HTTP exporters only -- no grpcio dependency, confirmed safe with numpy 1.26.4.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Signal decisions are logged in `BaseStrategy.on_tick()` -- every strategy gets logging automatically through the base class.
- D-02: Log verbosity is configurable: default to structured decision record (signal_type, direction, decision, reason), but allow full indicator dump via env var or config flag for debugging sessions.
- D-03: Structured record fields: signal_type, direction, decision (place/skip), reason, symbol, playground_id, trace_id.
- D-04: Full dump adds: all indicator values the strategy computed during evaluation (varies per strategy).
- D-05: "No action" decisions (below threshold, position full, spread too wide) are logged with explicit reason -- this is the core observability gap being solved.
- D-06: OTel init code lives in a separate module `engine/otel.py` with a `setup_otel()` function.
- D-07: `setup_otel()` is called inside `trading_engine.run_strategy()` -- automatic for every strategy run.
- D-08: Use OTLP HTTP exporters only (`opentelemetry-exporter-otlp-proto-http`) -- no grpcio to avoid numpy 1.26.4 conflict.
- D-09: trace_id from active OTel span is set on every NextTickRequest and PlaceOrderRequest sent via the Twirp client (`engine/client.py`).
- D-10: OTEL_* env vars configure the Python SDK (same as Go -- OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME="grodt-strategy").
- D-11: Heartbeat runs in a background daemon thread with 30s timer -- reports even when tick loop is blocked waiting for server.
- D-12: Heartbeat stats: strategy name, strategy state (active/idle), tick count, last tick time.
- D-13: Heartbeat is a metric gauge + structured log (same pattern as Go server heartbeat).
- D-14: Signal counts and position data are NOT in heartbeat -- kept simple. Signal counts are separate metrics (from D-01 logging).

### Claude's Discretion
- OTel Python package versions (must be compatible with numpy 1.26.4)
- How to extract trace_id from active span in Python (`trace.get_current_span().get_span_context().trace_id`)
- Config mechanism for full indicator dump vs structured (env var name, default behavior)
- How BaseStrategy exposes indicator values for the full dump mode
- Heartbeat thread implementation details (threading.Timer vs threading.Event)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PYTEL-01 | OTel Python SDK installed in grodt conda env (compatible with numpy 1.26.4) | Verified: `opentelemetry-exporter-otlp-proto-http==1.40.0` has no grpcio dependency, uses `requests` library. Dry-run install confirmed no conflicts. |
| PYTEL-02 | Python client initializes TracerProvider + MeterProvider with OTLP HTTP exporters | `engine/otel.py` setup mirrors Go `utils.SetupOTelSDK`. Code pattern documented below. |
| PYTEL-03 | trading_engine.py tick loop instrumented with parent span per iteration | `run_strategy()` at line 58 is the single entry point. Each iteration can wrap `strategy.on_tick()` in a span. |
| PYTEL-04 | tick() and place_order() methods emit child spans with trace_id on protobuf fields | Go server reads `req.TraceId` from protobuf (not HTTP headers). Python sets it via `request.trace_id = hex(span_context.trace_id)`. |
| STRAT-01 | Python strategy logs indicator evaluation results | BaseStrategy.on_tick() in base class can log after strategy subclass returns decision object. Strategies expose indicators via method. |
| STRAT-02 | Python strategy logs signal generation events | Signal decisions captured in structured log emitted by BaseStrategy wrapper. |
| STRAT-03 | Python strategy logs "no action" decisions with reason | D-05 mandates explicit reason logging. Decision object includes `decision=skip, reason="below threshold"` etc. |
| BEAT-03 | Python strategy client emits a heartbeat metric gauge every 30 seconds | Background daemon thread with `threading.Event` wait pattern. Gauge metric `grodt.strategy.heartbeat`. |
| BEAT-04 | Python strategy client emits a periodic structured heartbeat log with strategy state | Structured log with strategy_name, state, tick_count, last_tick_time, uptime_seconds. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| opentelemetry-exporter-otlp-proto-http | 1.40.0 | OTLP HTTP exporter for traces + metrics | HTTP-only, no grpcio dependency. Uses `requests` (already installed). |
| opentelemetry-sdk | 1.40.0 | TracerProvider, MeterProvider, Resource | Auto-installed as dependency of exporter. |
| opentelemetry-api | 1.40.0 | Span creation, trace context, meter API | Auto-installed as dependency of SDK. |
| opentelemetry-proto | 1.40.0 | Protobuf definitions for OTLP protocol | Auto-installed as dependency of exporter. |
| opentelemetry-semantic-conventions | 0.61b0 | Standard attribute names | Auto-installed as dependency of SDK. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| opentelemetry-exporter-otlp-proto-common | 1.40.0 | Shared serialization for OTLP exporters | Auto-installed. |
| googleapis-common-protos | 1.73.1 | Google protobuf common types | Auto-installed. |
| importlib-metadata | 8.7.1 | Python metadata access | Auto-installed for Python 3.10. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| OTLP HTTP exporter | OTLP gRPC exporter | gRPC exporter requires grpcio which conflicts with numpy 1.26.4. HTTP is the right choice. |
| threading.Event for heartbeat | threading.Timer | Timer creates new threads per tick. Event loop in a single daemon thread is cleaner and matches Go's ticker pattern. |
| Structured logging via OTel | loguru structured | OTel logging keeps Python and Go telemetry in same pipeline. loguru stays for console/file output. |

**Installation:**
```bash
/Users/jamal/miniconda3/envs/grodt/bin/pip install opentelemetry-exporter-otlp-proto-http==1.40.0
```

This single install pulls in all required OTel packages (sdk, api, proto, semantic-conventions, common).

**Version verification:** Confirmed via `pip index versions` and `pip install --dry-run` on 2026-03-26. Latest stable is 1.40.0. No grpcio in dependency tree.

## Architecture Patterns

### Recommended Project Structure
```
src/clients/python/
  engine/
    otel.py             # NEW: setup_otel() function
    heartbeat.py         # NEW: StrategyHeartbeat daemon thread
    client.py            # MODIFY: set trace_id on requests
    trading_engine.py    # MODIFY: call setup_otel(), wrap tick loop in spans
    types.py             # MODIFY: add SignalDecision dataclass
  strategies/
    base_strategy.py     # MODIFY: add decision logging wrapper
    covered_call.py      # MODIFY: return SignalDecision from on_tick()
    wheel.py             # MODIFY: return SignalDecision from on_tick()
    ...                  # Other strategies similarly modified
```

### Pattern 1: OTel SDK Setup (engine/otel.py)
**What:** Initialize TracerProvider + MeterProvider with OTLP HTTP exporters, mirroring Go's `utils.SetupOTelSDK`.
**When to use:** Called once at strategy startup in `run_strategy()`.
**Example:**
```python
# Source: Go utils/otel.go pattern + OTel Python SDK docs
from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.semconv.resource import ResourceAttributes
import os
import logging

logger = logging.getLogger(__name__)

def setup_otel(service_name: str = None, service_version: str = "1.0.0"):
    """Initialize OTel SDK with OTLP HTTP exporters.

    Reads OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME from env vars.
    Returns a shutdown callable for cleanup.
    """
    if service_name is None:
        service_name = os.getenv("OTEL_SERVICE_NAME", "grodt-strategy")

    resource = Resource.create({
        ResourceAttributes.SERVICE_NAME: service_name,
        ResourceAttributes.SERVICE_VERSION: service_version,
    })

    # Traces
    trace_exporter = OTLPSpanExporter()  # reads OTEL_EXPORTER_OTLP_ENDPOINT
    tracer_provider = TracerProvider(resource=resource)
    tracer_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
    trace.set_tracer_provider(tracer_provider)

    # Metrics
    metric_exporter = OTLPMetricExporter()  # reads OTEL_EXPORTER_OTLP_ENDPOINT
    metric_reader = PeriodicExportingMetricReader(metric_exporter)
    meter_provider = MeterProvider(resource=resource, metric_readers=[metric_reader])
    metrics.set_meter_provider(meter_provider)

    def shutdown():
        tracer_provider.force_flush()
        tracer_provider.shutdown()
        meter_provider.force_flush()
        meter_provider.shutdown()

    return shutdown
```

### Pattern 2: Trace ID Extraction and Injection
**What:** Extract trace_id from active OTel span and set on protobuf request fields.
**When to use:** In `client.py` `tick()` and `place_order()` methods.
**Example:**
```python
from opentelemetry import trace

def _get_trace_id() -> str:
    """Extract trace_id from current active span as hex string."""
    span = trace.get_current_span()
    ctx = span.get_span_context()
    if ctx.trace_id == 0:
        return ""
    return format(ctx.trace_id, '032x')

# In tick() method:
request = NextTickRequest(
    playground_id=self.id,
    seconds=seconds,
    is_preview=False,
    request_id=str(uuid.uuid4()),
    trace_id=_get_trace_id(),  # auto-set from active span
)
```

### Pattern 3: Signal Decision Logging
**What:** BaseStrategy wraps on_tick() to log every decision (place or skip) with structured fields.
**When to use:** Every tick iteration.
**Example:**
```python
import logging
from dataclasses import dataclass, asdict
from typing import Optional, Dict, Any

@dataclass
class SignalDecision:
    signal_type: str          # e.g. "covered_call", "mean_reversion_dip"
    direction: str            # "long", "short", "neutral"
    decision: str             # "place" or "skip"
    reason: str               # e.g. "below threshold", "position full"
    symbol: str
    playground_id: str
    trace_id: str = ""
    indicators: Optional[Dict[str, Any]] = None  # full dump when verbose

# In BaseStrategy (modified on_tick wrapper):
class BaseStrategy(ABC):
    def _log_decision(self, decision: SignalDecision):
        verbose = os.getenv("STRATEGY_LOG_VERBOSE", "false").lower() == "true"
        record = asdict(decision)
        if not verbose:
            record.pop("indicators", None)

        otel_logger = logging.getLogger("grodt.strategy.signal")
        otel_logger.info("signal_decision", extra=record)
```

### Pattern 4: Heartbeat Daemon Thread
**What:** Background thread emitting metric gauge + structured log every 30s.
**When to use:** Started in `run_strategy()`, stopped on exit.
**Example:**
```python
import threading
import time
import logging
from opentelemetry import metrics

class StrategyHeartbeat:
    def __init__(self, strategy_name: str):
        self.strategy_name = strategy_name
        self.state = "idle"
        self.tick_count = 0
        self.last_tick_time = None
        self._stop_event = threading.Event()
        self._start_time = time.time()

        meter = metrics.get_meter("grodt-strategy")
        self._heartbeat_gauge = meter.create_gauge(
            "grodt.strategy.heartbeat",
            unit="1",
            description="Strategy heartbeat (1=alive)"
        )
        self._logger = logging.getLogger("grodt.strategy.heartbeat")

    def start(self):
        thread = threading.Thread(target=self._run, daemon=True)
        thread.start()

    def _run(self):
        while not self._stop_event.wait(timeout=30):
            uptime = time.time() - self._start_time
            self._heartbeat_gauge.set(1, attributes={
                "strategy_name": self.strategy_name,
                "state": self.state,
            })
            self._logger.info("strategy heartbeat", extra={
                "event": "heartbeat",
                "strategy_name": self.strategy_name,
                "state": self.state,
                "tick_count": self.tick_count,
                "last_tick_time": self.last_tick_time,
                "uptime_seconds": int(uptime),
            })

    def stop(self):
        self._stop_event.set()
```

### Anti-Patterns to Avoid
- **Do not use `opentelemetry-exporter-otlp` (the meta-package):** It installs BOTH gRPC and HTTP exporters, pulling in grpcio. Always install `opentelemetry-exporter-otlp-proto-http` specifically.
- **Do not pass trace_id via HTTP headers for Twirp:** The Go server reads `req.TraceId` from the protobuf message field, not from HTTP headers. Using Twirp `Context(headers=...)` for trace propagation would be ignored.
- **Do not create spans in simulator/backtest mode:** Per-tick spans in backtesting would create massive trace volume and 35%+ CPU overhead (stated in REQUIREMENTS.md Out of Scope). Use a guard: only create spans when `environment == "live"`.
- **Do not use `threading.Timer` for heartbeat:** Timer creates a new thread each time it fires. Use `threading.Event.wait(timeout=30)` in a single daemon thread for cleaner resource management.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OTLP export | Custom HTTP POST to collector | `OTLPSpanExporter` / `OTLPMetricExporter` | Handles batching, retry, protobuf serialization, endpoint config via env vars |
| Trace context propagation | Manual trace ID generation | `trace.get_current_span().get_span_context()` | OTel manages span context lifecycle, parent-child relationships |
| Metric gauges | Custom metric collection | `meter.create_gauge()` | Automatic periodic export to Prometheus via collector |
| Resource attributes | Hardcoded service name strings | `Resource.create()` with semconv constants | Consistent with OTel conventions, auto-picked up by backends |
| Structured logging | Custom JSON formatter | Python `logging` with OTel-compatible extras | Loki/Grafana can parse structured log fields |

**Key insight:** The OTel Python SDK auto-reads `OTEL_*` env vars for endpoint, service name, and protocol. No need to hardcode any endpoints -- same env var pattern used by the Go side.

## Common Pitfalls

### Pitfall 1: grpcio Sneaking In
**What goes wrong:** Installing `opentelemetry-exporter-otlp` (without `-proto-http` suffix) pulls in grpcio, which conflicts with numpy 1.26.4.
**Why it happens:** The meta-package includes both HTTP and gRPC exporters.
**How to avoid:** Always install `opentelemetry-exporter-otlp-proto-http` explicitly. Pin it in requirements.txt.
**Warning signs:** `pip install` output showing `grpcio` being downloaded.

### Pitfall 2: trace_id Format Mismatch
**What goes wrong:** OTel trace_id is a 128-bit integer internally. Sending the raw int as a string produces unusable trace IDs.
**Why it happens:** `span_context.trace_id` returns an int, not a hex string.
**How to avoid:** Always format as 32-char hex: `format(ctx.trace_id, '032x')`.
**Warning signs:** Go server logs showing numeric trace_id values instead of hex strings.

### Pitfall 3: Span Explosion in Backtest Mode
**What goes wrong:** Creating OTel spans for every tick in backtesting (500K+ iterations) generates massive trace volume and CPU overhead.
**Why it happens:** Backtest runs hit max_iterations=500,000 ticks per run.
**How to avoid:** Only create spans when `playground.environment == "live"`. Use a no-op tracer or skip span creation in simulator mode.
**Warning signs:** Slow backtests, collector OOM, massive trace storage.

### Pitfall 4: Heartbeat Thread Not Stopping
**What goes wrong:** Heartbeat thread continues running after strategy completes, preventing clean shutdown.
**Why it happens:** `threading.Timer` or non-daemon threads persist after main thread exits.
**How to avoid:** Use `daemon=True` on the thread AND call `stop()` explicitly in the shutdown sequence.
**Warning signs:** Python process hanging after strategy completion.

### Pitfall 5: OTel Setup Called Multiple Times
**What goes wrong:** If `run_strategy()` is called multiple times (e.g., optimizer), each call re-initializes OTel providers, leaking resources.
**Why it happens:** `setup_otel()` creates new TracerProvider each time.
**How to avoid:** Add a guard: check if provider is already set before initializing. Use a module-level flag or check `trace.get_tracer_provider()`.
**Warning signs:** Memory leaks during optimizer runs.

### Pitfall 6: Python Logging vs OTel Logging Bridge
**What goes wrong:** Using `loguru` for decision logging bypasses OTel log export pipeline.
**Why it happens:** The project currently uses loguru everywhere. OTel has a separate logging bridge.
**How to avoid:** Use Python's standard `logging` module for structured telemetry logs (OTel can bridge these). Keep loguru for console/file output. The two can coexist.
**Warning signs:** Decision logs visible in local files but not in Loki/Grafana.

## Code Examples

### Extracting trace_id from active span
```python
# Source: OTel Python API docs
from opentelemetry import trace

def get_current_trace_id() -> str:
    span = trace.get_current_span()
    if span is None:
        return ""
    ctx = span.get_span_context()
    if ctx is None or ctx.trace_id == 0:
        return ""
    return format(ctx.trace_id, '032x')
```

### Creating a span for tick loop iteration
```python
# Source: OTel Python SDK docs
tracer = trace.get_tracer("grodt-strategy")

# In run_strategy() tick loop:
with tracer.start_as_current_span(
    "strategy.tick",
    attributes={
        "playground_id": playground.id,
        "tick_number": iteration,
        "symbol": strategy.symbol,
    }
) as span:
    tick_deltas = playground.flush_new_state_buffer()
    strategy.on_tick(tick_deltas)
```

### Twirp Context with headers (NOT needed for trace_id but useful reference)
```python
# Source: twirp Python library context.py
from twirp.context import Context

# The Twirp Context supports headers:
ctx = Context(headers={"X-Custom-Header": "value"})

# For trace_id, we set it on the protobuf message directly (D-09):
request = NextTickRequest(
    playground_id=self.id,
    seconds=seconds,
    trace_id=get_current_trace_id(),  # protobuf field, not HTTP header
)
response = self.client.NextTick(ctx=Context(), request=request)
```

### Go server trace_id handling (for reference)
```go
// Source: src/go/backtester-api/router/grpc.go lines 694-704
// Go reads trace_id from the protobuf message, not from HTTP headers:
if req.TraceId != "" {
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(attribute.String("trace_id", req.TraceId))
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| loguru for all logging | OTel-compatible structured logging for telemetry + loguru for console | Phase 4 | Telemetry logs flow to Loki via OTel Collector |
| No Python instrumentation | OTel SDK with OTLP HTTP exporters | Phase 4 | Python traces and metrics visible alongside Go in Grafana |
| Manual RPC timing via rpc_profiler.py | OTel spans on tick/place_order | Phase 4 | Profiling data in Tempo instead of local profiler |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | pytest (already in grodt conda env) |
| Config file | None specific -- tests run from `src/clients/python/` |
| Quick run command | `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x -q` |
| Full suite command | `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PYTEL-01 | OTel SDK importable | unit | `python -c "from opentelemetry import trace, metrics"` | N/A (import check) |
| PYTEL-02 | setup_otel() creates providers | unit | `pytest tests/test_otel.py -x` | Wave 0 |
| PYTEL-03 | Tick loop creates parent span | unit | `pytest tests/test_otel.py::test_tick_span -x` | Wave 0 |
| PYTEL-04 | trace_id set on NextTickRequest | unit | `pytest tests/test_otel.py::test_trace_id_on_request -x` | Wave 0 |
| STRAT-01 | Indicator values logged | unit | `pytest tests/test_signal_logging.py -x` | Wave 0 |
| STRAT-02 | Signal generation logged | unit | `pytest tests/test_signal_logging.py -x` | Wave 0 |
| STRAT-03 | "No action" logged with reason | unit | `pytest tests/test_signal_logging.py -x` | Wave 0 |
| BEAT-03 | Heartbeat gauge emitted | unit | `pytest tests/test_heartbeat.py -x` | Wave 0 |
| BEAT-04 | Heartbeat structured log emitted | unit | `pytest tests/test_heartbeat.py -x` | Wave 0 |

### Sampling Rate
- **Per task commit:** `pytest tests/test_otel.py tests/test_heartbeat.py tests/test_signal_logging.py -x -q`
- **Per wave merge:** Full test suite
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `tests/test_otel.py` -- covers PYTEL-02, PYTEL-03, PYTEL-04
- [ ] `tests/test_heartbeat.py` -- covers BEAT-03, BEAT-04
- [ ] `tests/test_signal_logging.py` -- covers STRAT-01, STRAT-02, STRAT-03

## Open Questions

1. **SignalDecision return type for existing strategies**
   - What we know: Current `on_tick()` returns `None`. Strategies call `playground.place_order()` internally.
   - What's unclear: How to capture "skip" decisions without changing every strategy's on_tick() signature significantly.
   - Recommendation: Add an optional `_decisions` list attribute to BaseStrategy. Strategies append SignalDecision objects during on_tick(). BaseStrategy logs them after on_tick() returns. No return type change needed.

2. **OTel logging bridge for structured logs**
   - What we know: OTel Python has a logging bridge (`opentelemetry-instrumentation-logging`) that can auto-export Python `logging` records.
   - What's unclear: Whether the bridge is needed for v1 or if direct structured logging to Loki via loguru's existing file output is sufficient.
   - Recommendation: For v1, use Python's standard `logging` module for telemetry-specific logs (signal decisions, heartbeat). These get picked up by OTel Collector if configured. Keep loguru for console/file. Defer the bridge to v2 if needed.

3. **Backtest vs live span creation guard**
   - What we know: Backtests run 500K+ iterations. Creating spans per tick is explicitly out of scope.
   - What's unclear: Whether to use a completely separate code path or a simple if-guard.
   - Recommendation: Simple if-guard in `run_strategy()`: `if playground.environment == "live":` before span creation. Minimal code duplication.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Python 3.10 | OTel SDK | Yes | 3.10.16 | -- |
| grodt conda env | All Python code | Yes | -- | -- |
| pip | Package install | Yes | -- | -- |
| numpy | Must stay at 1.26.4 | Yes | 1.26.4 | -- |
| requests | OTLP HTTP exporter | Yes | 2.32.3 | -- |
| protobuf | OTel proto | Yes | 5.29.3 | -- |
| OTel Collector (Docker) | Telemetry backend | Yes (Phase 2) | grafana/otel-lgtm | -- |

**Missing dependencies with no fallback:**
- `opentelemetry-exporter-otlp-proto-http` -- must be installed (Phase 4 task)

**Missing dependencies with fallback:**
- None

## Sources

### Primary (HIGH confidence)
- `pip install --dry-run opentelemetry-exporter-otlp-proto-http==1.40.0` -- confirmed no grpcio dependency
- `pip index versions opentelemetry-exporter-otlp-proto-http` -- confirmed 1.40.0 is latest stable
- `src/go/utils/otel.go` -- Go OTel setup pattern to mirror
- `src/go/telemetry/heartbeat.go` -- Go heartbeat pattern to mirror
- `src/go/telemetry/metrics.go` -- Go metric naming conventions
- `src/go/backtester-api/router/grpc.go` -- trace_id read from protobuf fields
- `src/clients/python/engine/client.py` -- current Twirp client implementation
- `src/clients/python/engine/trading_engine.py` -- run_strategy() entry point
- `src/clients/python/strategies/base_strategy.py` -- BaseStrategy interface
- `twirp/context.py` (installed package) -- Context supports headers dict

### Secondary (MEDIUM confidence)
- [OTel Python exporters docs](https://opentelemetry.io/docs/languages/python/exporters/) -- OTLP HTTP exporter configuration
- [OTLP Exporter Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/) -- OTEL_* env var reference

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - confirmed via pip dry-run, no grpcio in deps, versions verified from PyPI
- Architecture: HIGH - patterns directly mirror existing Go implementation in the codebase
- Pitfalls: HIGH - grpcio conflict validated, trace_id format verified against Go server code, backtest scope confirmed in REQUIREMENTS.md

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable packages, unlikely to change significantly)
