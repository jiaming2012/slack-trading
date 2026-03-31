# Phase 23: Replay & Telemetry - Research

**Researched:** 2026-03-31
**Domain:** ESDB signal replay, OTel telemetry, Grafana dashboards, alerting
**Confidence:** HIGH

## Summary

Phase 23 is the final phase of v3.0. It adds four capabilities: (1) replay signals from ESDB into simulator playgrounds via a `--replay-signals` CLI flag, (2) signal production/consumption OTel telemetry visible in Grafana, (3) a DatasourceHeartbeat class for datasource liveness monitoring, and (4) Grafana alerting on datasource heartbeat staleness.

The existing codebase has all the infrastructure needed. The `InMemorySignalRepository` already supports bulk-loading signals via repeated `Write()` calls and clock-gated delivery via `ReadPending(upTo)`. The `ESDBSignalRepository` can read the full `trade-signals` stream via `FetchAll`. The `StrategyHeartbeat` class provides a copy-paste template for `DatasourceHeartbeat`. The `grodt-live-simulation.json` dashboard already has 14 panels across 4 rows (System Health, Order Activity, Market Data, Positions) -- signal panels add as a new "Signals & Datasources" row. The `alerting.yaml` already has heartbeat stale rules that serve as a template.

**Primary recommendation:** Implement in order: (1) replay preload logic + CLI flag, (2) DatasourceHeartbeat class, (3) Grafana signal panels + heartbeat alert, (4) dual-run integration test.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Replay mode via `--replay-signals trade-signals` CLI flag on demo scripts. Preloads ESDB signals into InMemorySignalRepository. Strategy code unchanged.
- D-02: Alerting is about datasource script liveness (heartbeat), NOT signal production frequency.
- D-03: Replay integration test uses dual-run comparison (in-memory datasource vs ESDB replay). Requires ESDB TestContainer. Reuses Phase 21 behavioral diff pattern.
- D-04: Add 2-3 panels to existing `grodt-live-simulation.json` dashboard (no new dashboard).
- D-05: New `DatasourceHeartbeat` class (separate from `StrategyHeartbeat`) with datasource-specific OTel metrics.

### Claude's Discretion
- DatasourceHeartbeat class internals and OTel metric names
- Exact Grafana panel queries and layout
- How ESDB replay preloads signals (batch read on startup vs streaming)
- Alert threshold timing (how many minutes of silence before alerting)

### Deferred Ideas (OUT OF SCOPE)
- Composite signal framework -- v3.1
- Signal backtesting optimizer -- future
- Per-signal-type alerting thresholds -- future enhancement
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REPLAY-01 | Sim strategies can replay persisted signal streams, gated by playground clock time | ESDBSignalRepository.GetAll() reads full stream; InMemorySignalRepository.Write() bulk-loads; ReadPending() gates by clock |
| REPLAY-02 | Integration tests verify replay results match in-memory datasource results | Existing ESDB TestContainer setup in integration_testing/; Phase 21 diff test pattern reusable |
| OBS-01 | Signals visible in OTel/Grafana telemetry framework | Existing `grodt.signals.generated` counter + dashboard panels; add consumption counter and datasource heartbeat |
| OBS-02 | Alerting when expected signal not produced within expected interval | DatasourceHeartbeat gauge + absent_over_time() alert rule follows existing pattern |
</phase_requirements>

## Architecture Patterns

### Replay Preload Flow

The replay mechanism is straightforward because the signal infrastructure already supports it:

1. **Demo script** receives `--replay-signals trade-signals` flag
2. **On startup**, script reads all signals from ESDB via the Go server's existing `FetchAll` mechanism
3. Signals are bulk-loaded into a fresh `InMemorySignalRepository` via repeated `Write()` calls
4. Playground uses this preloaded InMemorySignalRepository instead of an empty one
5. `ReadPending(clock.CurrentTime)` delivers signals at the right time -- no strategy changes needed

**Key insight:** The replay does NOT happen in Python. The Go server's `CreatePlayground` RPC already assigns the signal repository. The approach requires either:
- **Option A (Recommended):** New RPC or flag on `CreatePlayground` that tells the server "preload signals from ESDB stream X into InMemorySignalRepository." This keeps the ESDB read server-side.
- **Option B:** Python reads signals from ESDB via a new `GetAllSignals` RPC, then writes them back via `WriteSignal` RPC. More round-trips but no server-side change to CreatePlayground.

**Recommendation (Claude's discretion):** Option A -- add an optional `replay_signal_stream` field to the CreatePlayground proto request. When set, the server reads all signals from that ESDB stream and bulk-loads them into the InMemorySignalRepository. This is simpler, faster, and keeps ESDB access in Go.

### Signal Consumption Telemetry

Currently, `RecordSignal` RPC increments `grodt.signals.generated` counter with attributes `signal_type`, `decision`, `symbol`, `playground_id`, `client_id`. This tracks signal **production**.

For **consumption** visibility (OBS-01), the signal delivery path in `playground.go:1633` (`p.signalRepo.ReadPending(p.clock.CurrentTime)`) is where signals become visible to the strategy. Adding a counter increment here tracks consumption.

**New metric:** `grodt.signals.consumed` counter with attributes `signal_name`, `symbol`, `playground_id`, `client_id`.

### DatasourceHeartbeat Pattern

Direct copy of `StrategyHeartbeat` with datasource-specific fields:

```python
# src/clients/python/engine/datasource_heartbeat.py
class DatasourceHeartbeat:
    def __init__(self, datasource_name: str, symbol: str = ""):
        # OTel gauge: grodt.datasource.heartbeat (1=alive)
        # Attributes: datasource_name, symbol
        # Structured log: heartbeat | datasource={name} checks={count} ...

    def record_check(self):
        """Called each time the datasource checks for new data."""
        self.check_count += 1
        self.last_check_time = now()
```

**Metric name:** `grodt.datasource.heartbeat` (gauge, value=1 when alive)
**Attributes:** `datasource_name`, `symbol`

### Grafana Dashboard Additions

The dashboard currently has rows: System Health (y=0), Order Activity (y=9), Market Data (y=30), Positions (y=41).

**Add new row:** "Signals & Datasources" after the existing "Signals Generated" panel (id=11, currently in Market Data row at y=31).

New panels (ids 15-17):
1. **Signal Consumption Rate** (timeseries) -- `rate(grodt_signals_consumed_total[5m])` by signal_name
2. **Datasource Heartbeat Status** (stat) -- `grodt_datasource_heartbeat` by datasource_name. Color: green=1, red=0/absent.
3. **Datasource Checks** (timeseries) -- `rate(grodt_datasource_checks_total[5m])` by datasource_name (optional, if adding a counter for checks)

### Alert Rule

```yaml
# In alerting.yaml, under grodt-heartbeat group
- uid: datasource-heartbeat-stale
  title: Datasource Heartbeat Stale
  condition: C
  for: 5m  # 5 minutes of staleness (Claude's discretion)
  annotations:
    summary: Datasource heartbeat has not been received for over 5 minutes
    description: A datasource script has stopped emitting heartbeat. Check if the process is running.
  labels:
    severity: critical
  data:
    - refId: A
      datasourceUid: prometheus
      model:
        expr: absent_over_time(grodt_datasource_heartbeat[5m])
        instant: true
    - refId: C
      datasourceUid: __expr__
      model:
        type: threshold
        expression: A
        conditions:
          - evaluator: { type: gt, params: [0] }
```

**Threshold reasoning (Claude's discretion):** 5 minutes is appropriate because the heartbeat emits every 30 seconds. A 5-minute window means 10 missed heartbeats before alerting, avoiding false positives from brief network hiccups while still catching a dead process quickly.

### Dual-Run Integration Test

The test follows the Phase 21 behavioral diff pattern but at the integration level with a real ESDB container:

1. Start ESDB TestContainer (reuse `startESDBContainer` from `integration_testing/esdb_signal_repository_test.go`)
2. **Run 1 (baseline):** Create playground with InMemorySignalRepository. Run strategy with inline datasource. Signals written to InMemory during tick loop. Record final metrics.
3. **Persist signals:** Write all signals from Run 1 to ESDB via `ESDBSignalRepository.Write()`
4. **Run 2 (replay):** Create new playground with `replay_signal_stream` flag. Signals preloaded from ESDB into InMemorySignalRepository. Run same strategy. Record final metrics.
5. **Compare:** Assert P&L, trade count, win rate match exactly.

**Location:** `integration_testing/replay_integration_test.go` (Go build tag `integration`)

## Existing Code Assets

### Ready to Reuse (No Modification)
| Asset | Location | How It's Used |
|-------|----------|---------------|
| InMemorySignalRepository | `src/go/backtester-api/models/signal_repository_memory.go` | Preload target for replay signals |
| ESDBSignalRepository.GetAll() | `src/go/backtester-api/models/signal_repository_esdb.go` | Read all signals from ESDB for preloading |
| StrategyHeartbeat | `src/clients/python/engine/heartbeat.py` | Template for DatasourceHeartbeat |
| ESDB TestContainer setup | `integration_testing/esdb_signal_repository_test.go` | Reuse `startESDBContainer` and `createESDBProducer` |
| FetchAll generic | `src/go/eventservices/eventstoredb.go:121` | Server-side ESDB stream read |
| OTel setup (Python) | `src/clients/python/engine/otel.py` | Already provides MeterProvider for new gauges |
| Alerting YAML | `observability/alerting/alerting.yaml` | Heartbeat stale pattern to copy |

### Requires Modification
| Asset | Location | Change Needed |
|-------|----------|---------------|
| CreatePlayground RPC / proto | `src/go/playground.proto`, `router/grpc.go` | Add optional `replay_signal_stream` field |
| Playground.simulateTick | `src/go/backtester-api/models/playground.go:1633` | Add signal consumption counter |
| grodt-live-simulation.json | `observability/dashboards/grodt-live-simulation.json` | Add signal panels row (ids 15-17) |
| alerting.yaml | `observability/alerting/alerting.yaml` | Add datasource heartbeat stale rule |
| Demo script (mean_reversion_v2) | `src/clients/python/demos/demo_mean_reversion_v2.py` | Add `--replay-signals` argparse flag |
| telemetry/metrics.go | `src/go/telemetry/metrics.go` | Add `SignalsConsumed` counter |

### New Files
| File | Purpose |
|------|---------|
| `src/clients/python/engine/datasource_heartbeat.py` | DatasourceHeartbeat class |
| `integration_testing/replay_integration_test.go` | Dual-run replay comparison test |

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ESDB stream reading | Custom event iteration | `eventservices.FetchAll[*eventmodels.TradeSignal]()` | Already handles pagination, error handling, type deserialization |
| Clock-gated signal delivery | Custom time filtering | `InMemorySignalRepository.ReadPending(upTo)` | Already has cursor advancement, sorted insertion, thread safety |
| OTel metric emission | Raw HTTP to collector | `opentelemetry.metrics.get_meter().create_gauge()` | Python OTel SDK already configured in `engine/otel.py` |
| Heartbeat daemon thread | Custom threading | Copy `StrategyHeartbeat` pattern | Proven daemon thread with clean shutdown |
| ESDB TestContainer | Manual Docker setup | `testcontainers.GenericContainer` with existing helper | `startESDBContainer` already handles image, env vars, health wait |

## Common Pitfalls

### Pitfall 1: ESDB Cursor State in Replay
**What goes wrong:** The InMemorySignalRepository cursor starts at 0 after bulk-loading. If CreatePlayground sets the playground clock start time AFTER some signals, those signals are never delivered.
**Why it happens:** Signals loaded with timestamps before the playground's start date get skipped on the first `ReadPending(startTime)` call, which is correct behavior. But if the playground clock starts at `startDate` and signals exist before that, they appear consumed but never delivered.
**How to avoid:** Only load signals whose timestamps fall within the playground's `[startDate, stopDate]` range during preload.
**Warning signs:** Replay run has fewer signals than expected.

### Pitfall 2: Signal Ordering After ESDB Round-Trip
**What goes wrong:** ESDB stores events in append order, not timestamp order. If signals were written out of order, `FetchAll` returns them in ESDB append order, not chronological order.
**Why it happens:** ESDB event ordering is by stream position, not by event data fields.
**How to avoid:** After reading from ESDB, sort signals by `Timestamp` before bulk-loading into `InMemorySignalRepository`. (Alternatively, `InMemorySignalRepository.Write()` already maintains sorted insertion, so writing in any order is safe.)
**Warning signs:** Signals delivered in unexpected order during replay.

### Pitfall 3: Proto Field Backward Compatibility
**What goes wrong:** Adding `replay_signal_stream` to CreatePlaygroundRequest proto can break existing callers if not optional.
**Why it happens:** proto3 fields are optional by default, but some serializers may fail on unknown fields.
**How to avoid:** Use `optional string replay_signal_stream = N;` in proto3. Ensure field number doesn't conflict. Regenerate stubs for both Go and Python.
**Warning signs:** Existing demo scripts or tests fail after proto update.

### Pitfall 4: Metric Name Convention Mismatch
**What goes wrong:** OTel SDK uses dot-separated names (`grodt.datasource.heartbeat`) but Prometheus converts to underscores (`grodt_datasource_heartbeat`). Grafana queries must use the Prometheus convention.
**Why it happens:** OTel-to-Prometheus naming convention automatically converts dots to underscores.
**How to avoid:** Use dot notation in code (`grodt.datasource.heartbeat`) and underscore notation in Grafana/alerting queries (`grodt_datasource_heartbeat`).
**Warning signs:** Grafana panels show "no data" despite metrics being emitted.

### Pitfall 5: Integration Test Timing
**What goes wrong:** ESDB writes are eventually consistent. Reading immediately after writing may return incomplete results.
**Why it happens:** ESDB has internal commit/flush timing.
**How to avoid:** In integration tests, add a brief polling loop after ESDB writes before reading back, or use the existing `FetchAll` which reads the full stream (waits for consistency).
**Warning signs:** Flaky tests that pass sometimes and fail sometimes.

## Code Examples

### Replay Preload in CreatePlayground (Go)

```go
// In router/grpc.go, within CreatePlayground handler:
// Source: existing pattern in grpc.go:1467-1474

if req.ReplaySignalStream != "" {
    // Read all signals from ESDB
    signals, err := eventservices.FetchAll[*eventmodels.TradeSignal](
        ctx, s.esdbProducer.GetClient(), &eventmodels.TradeSignal{},
    )
    if err != nil {
        return nil, twirp.InternalErrorWith(err)
    }

    // Bulk-load into InMemorySignalRepository (already sorted on Write)
    repo := models.NewInMemorySignalRepository()
    for _, sig := range signals {
        if err := repo.Write(sig); err != nil {
            return nil, twirp.InternalErrorWith(err)
        }
    }
    playground.SetSignalRepo(repo)
} else {
    // Existing logic: live/reconcile gets ESDB, simulator gets InMemory
    switch playgroundEnvironment {
    case models.PlaygroundEnvironmentLive, models.PlaygroundEnvironmentReconcile:
        if s.esdbProducer != nil {
            playground.SetSignalRepo(models.NewESDBSignalRepository(s.esdbProducer))
        }
    default:
        playground.SetSignalRepo(models.NewInMemorySignalRepository())
    }
}
```

### DatasourceHeartbeat Class (Python)

```python
# src/clients/python/engine/datasource_heartbeat.py
# Source: adapted from engine/heartbeat.py (StrategyHeartbeat)

import threading
import time
from datetime import datetime, timezone
from loguru import logger
from opentelemetry import metrics

HEARTBEAT_INTERVAL_SECONDS = 30

class DatasourceHeartbeat:
    def __init__(self, datasource_name: str, symbol: str = ""):
        self.datasource_name = datasource_name
        self.symbol = symbol
        self.check_count = 0
        self.last_check_time = None
        self._stop_event = threading.Event()
        self._start_time = time.time()
        self._thread = None

        meter = metrics.get_meter("grodt-datasource")
        self._heartbeat_gauge = meter.create_gauge(
            "grodt.datasource.heartbeat",
            description="Datasource heartbeat (1=alive)",
        )

    def start(self):
        self._thread = threading.Thread(
            target=self._run,
            name=f"heartbeat-ds-{self.datasource_name}",
            daemon=True,
        )
        self._thread.start()

    def stop(self):
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=2)

    def record_check(self):
        self.check_count += 1
        self.last_check_time = datetime.now(timezone.utc).isoformat()

    def _run(self):
        while not self._stop_event.wait(timeout=HEARTBEAT_INTERVAL_SECONDS):
            self._emit_heartbeat()

    def _emit_heartbeat(self):
        uptime_seconds = round(time.time() - self._start_time, 1)
        self._heartbeat_gauge.set(
            1,
            attributes={
                "datasource_name": self.datasource_name,
                "symbol": self.symbol,
            },
        )
        logger.info(
            "heartbeat | datasource={} checks={} uptime={}s symbol={}",
            self.datasource_name, self.check_count, uptime_seconds, self.symbol,
        )
```

### Signal Consumption Counter (Go)

```go
// In src/go/telemetry/metrics.go, add:
SignalsConsumed metric.Int64Counter

// In Init():
SignalsConsumed, err = meter.Int64Counter("grodt.signals.consumed",
    metric.WithUnit("{signal}"),
    metric.WithDescription("Number of signals consumed by strategies"))

// In src/go/backtester-api/models/playground.go, after ReadPending:
newSignals = p.signalRepo.ReadPending(p.clock.CurrentTime)
for _, sig := range newSignals {
    if telemetry.SignalsConsumed != nil {
        telemetry.SignalsConsumed.Add(context.Background(), 1,
            metric.WithAttributes(
                attribute.String("signal_name", string(sig.Name)),
                attribute.String("symbol", string(sig.Symbol)),
            ))
    }
}
```

### Grafana Panel: Signal Consumption Rate

```json
{
  "datasource": { "type": "prometheus", "uid": "prometheus" },
  "fieldConfig": { "defaults": { "custom": { "drawStyle": "line", "lineWidth": 1, "fillOpacity": 10 } } },
  "gridPos": { "h": 8, "w": 12, "x": 0, "y": 42 },
  "id": 15,
  "targets": [{
    "expr": "rate(grodt_signals_consumed_total{client_id=~\"$client_id\"}[5m])",
    "legendFormat": "{{signal_name}}"
  }],
  "title": "Signal Consumption Rate",
  "type": "timeseries"
}
```

### Replay CLI Flag (Python)

```python
# In demo_mean_reversion_v2.py, add to argparse:
parser.add_argument(
    "--replay-signals", type=str, default=None,
    help="Replay signals from ESDB stream name (e.g., 'trade-signals')",
)

# In playground creation:
req = CreatePolygonPlaygroundRequest(
    balance=args.balance,
    start_date=args.start,
    stop_date=args.end,
    repositories=repos,
    environment=PlaygroundEnvironment.SIMULATOR.value,
    replay_signal_stream=args.replay_signals,  # new optional field
)
```

## Dashboard Panel Layout

Current layout (last panel id=14, last y=60):

| Row | Y | Panels |
|-----|---|--------|
| System Health | 0 | Go Server Uptime (2), Active Playgrounds (3), Strategy Heartbeat (4) |
| Order Activity | 9 | Orders Placed (5), Order Fill Rate (6), Order Rejection (7), Order Log (8) |
| Market Data | 30 | Candles Processed (9), Candle Rate (10), Signals Generated (11) |
| Positions | 41 | Position Summary (13), Position Details (14) |

**Updated layout** -- insert "Signals & Datasources" row between Market Data and Positions:

| Row | Y (adjusted) | Panels |
|-----|---|--------|
| ... (unchanged) | ... | ... |
| Market Data | 30 | Candles Processed (9), Candle Rate (10), Signals Generated (11) |
| **Signals & Datasources** | **41** | **Signal Consumption Rate (15), Datasource Heartbeat (16)** |
| Positions | 52 (shifted) | Position Summary (13), Position Details (14) |

## Open Questions

1. **Proto field number for replay_signal_stream**
   - What we know: Need to add field to CreatePolygonPlaygroundRequest message
   - What's unclear: Current highest field number in that message
   - Recommendation: Check proto file during implementation, use next available number

2. **Signal time filtering during preload**
   - What we know: Playground has start/stop dates; ESDB signals span all time
   - What's unclear: Whether to filter server-side (in CreatePlayground) or load all and let ReadPending handle it
   - Recommendation: Filter to playground date range server-side for efficiency. Signals outside the range would never be delivered by ReadPending anyway, but loading thousands of irrelevant signals wastes memory.

## Sources

### Primary (HIGH confidence)
- `src/go/backtester-api/models/signal_repository_memory.go` -- InMemorySignalRepository implementation (Write, ReadPending, cursor logic)
- `src/go/backtester-api/models/signal_repository_esdb.go` -- ESDBSignalRepository (FetchAll, ReadPending)
- `src/go/backtester-api/models/signal_repository_interface.go` -- ISignalRepository interface
- `src/go/backtester-api/models/playground.go:1628-1634` -- Signal delivery in simulateTick
- `src/go/backtester-api/router/grpc.go:1467-1474` -- Signal repo assignment in CreatePlayground
- `src/clients/python/engine/heartbeat.py` -- StrategyHeartbeat (template for DatasourceHeartbeat)
- `src/clients/python/engine/otel.py` -- OTel SDK initialization
- `src/go/telemetry/metrics.go` -- Existing OTel metrics (SignalsGenerated counter pattern)
- `observability/alerting/alerting.yaml` -- Existing heartbeat stale alert rules
- `observability/dashboards/grodt-live-simulation.json` -- Current dashboard structure (14 panels, 4 rows)
- `integration_testing/esdb_signal_repository_test.go` -- ESDB TestContainer setup pattern
- `src/clients/python/tests/test_mean_reversion_diff.py` -- Behavioral diff test pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use (OTel, ESDB, Grafana, TestContainers)
- Architecture: HIGH -- replay mechanism uses only existing InMemorySignalRepository + ESDBSignalRepository
- Pitfalls: HIGH -- identified from direct code analysis of cursor behavior and ESDB ordering
- Dashboard: HIGH -- verified current panel IDs, row structure, and PromQL conventions from actual JSON

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable -- all dependencies already in project)
