# Phase 3: Go Telemetry Instrumentation - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Instrument the Go server with structured logs and metrics for order lifecycle (live playgrounds only), market data flow, and server heartbeat. This phase is Go server-side only — signal decision logging (why a trade was/wasn't placed) is Python-side and deferred to Phase 4.

</domain>

<decisions>
## Implementation Decisions

### Order Lifecycle (Live Only)
- **D-01:** Order placed/filled/rejected events emit structured logs with playground_id, order_id, symbol, side, quantity, order_type, environment="live", and trace_id.
- **D-02:** Only live playgrounds (Meta.Environment == "live") emit order telemetry. Simulator playgrounds are excluded. Reconcile playgrounds ARE included (they place real trades to the broker).
- **D-03:** Order rejection logs include the rejection reason as a structured field.

### Heartbeat
- **D-04:** Go server heartbeat emits every 30 seconds via background goroutine with ticker.
- **D-05:** Heartbeat is a metric gauge (for Grafana) + structured log (for forensics).
- **D-06:** Heartbeat stats: active playground count, last tick time, open order count, uptime.
- **D-07:** Heartbeat reports live playground stats only (consistent with order telemetry filter).
- **D-08:** Heartbeat metrics must be segmented by playground type and account type. Claude should design the optimal metric label/tag system using these dimensions:
  - **Environment:** live, reconcile, simulator
  - **Account type:** paper, margin
  - Live and reconcile playgrounds get granular per-playground metrics
  - Simulator playgrounds get only an aggregate count (nice-to-have, not granular)
  - Every live playground has exactly one reconcile playground. The reconcile playground places trades to the actual broker. This allows multiple live strategies to be segmented internally while trading through the same external broker account.

### Market Data Flow
- **D-09:** Keep a counter of candles successfully processed per strategy/playground (not individual candle logs — too noisy).
- **D-10:** Keep a counter of signals generated and processed.
- **D-11:** Signal decision logging (why trade was/wasn't placed after signal) is Phase 4 (Python client) — NOT this phase.
- **D-12:** Data gaps (missing candles, stale data) emit warning-level logs.

### Claude's Discretion
- Metric label/tag taxonomy for playground environment + account type segmentation
- Where exactly to hook into order lifecycle (grpc.go PlaceOrder handler vs models vs services)
- Heartbeat implementation details (background goroutine, ticker interval, gauge registration)
- Which candle/tick processing points to instrument for counters
- Whether to use OTel metrics API or Prometheus client directly

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 2 Outputs (foundation this builds on)
- `src/go/utils/otel.go` — SetupOTelSDK with TracerProvider + MeterProvider
- `cmd/main.go` — OTel init, logfmt formatter, graceful shutdown
- `observability/docker-compose.yaml` — Local otel-lgtm stack

### Order Lifecycle
- `src/go/backtester-api/router/grpc.go` — PlaceOrder handler (already has trace_id extraction from Phase 2)
- `src/go/backtester-api/models/playground.go` — Playground with Meta.Environment field
- `src/go/backtester-api/models/playground_environment.go` — PlaygroundEnvironment enum (simulator, live, reconcile)
- `src/go/backtester-api/models/live_account_type.go` — LiveAccountType enum (paper, margin, mock, simulator, reconcilation)
- `src/go/backtester-api/models/order_record.go` — OrderRecord with status lifecycle
- `src/go/backtester-api/models/trade_record.go` — TradeRecord (fills)
- `src/go/backtester-api/services/order_queue.go` — Order queue processing

### Market Data
- `src/go/eventservices/polygon.go` — Polygon data fetching (has existing OTel spans)
- `src/go/eventservices/fetch_option_chain_with_params.go` — Option chain fetching (has existing OTel spans)
- `src/go/backtester-api/models/candle_repository.go` — Candle storage and processing
- `src/go/eventconsumers/process_signals.go` — Signal processing (has existing OTel spans)

### Heartbeat
- `src/go/data/database_service.go` — DatabaseService with playground/order caches (source of heartbeat stats)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 2 established logfmt + snake_case conventions — follow same pattern
- trace_id already on PlaceOrder handler — extend to order fills/rejections
- OTel MeterProvider is initialized — use `otel.GetMeterProvider()` for metrics

### Playground Hierarchy
```
LiveAccount (paper or margin)
  └── Live Playground (Meta.Environment == "live")
        └── Reconcile Playground (Meta.Environment == "reconcile")
              └── Places trades to broker
Simulator Playground (Meta.Environment == "simulator")
  └── Short-lived, throwaway — aggregate count only
```

### Integration Points
- DatabaseService.GetPlaygrounds() returns all playgrounds — filter by Meta.Environment
- OrderRecord has status field for lifecycle tracking
- cmd/main.go is where the heartbeat goroutine should be started (alongside other background workers)

</code_context>

<specifics>
## Specific Ideas

- Candle counter per playground, not individual candle log entries (too noisy)
- Signal counter per playground — track generated vs processed
- Reconcile playgrounds should be included in order telemetry (they place real trades)
- Simulator playground metrics are nice-to-have aggregate count only

</specifics>

<deferred>
## Deferred Ideas

- **Signal decision logging** — why a trade was/wasn't placed after a signal. This happens in the Python strategy client, not the Go server. Deferred to Phase 4.

</deferred>

---

*Phase: 03-go-telemetry-instrumentation*
*Context gathered: 2026-03-26*
