# Phase 18: Signal Repository & Sim Mode - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Create ISignalRepository interface with InMemorySignalRepository for simulations. Signals are delivered to strategies via TickDelta (new NewSignals field), clock-gated to prevent lookahead bias. Live persistence is write-through; sim persistence is batch-on-save only.

Requirements: REPO-01, REPO-03

</domain>

<decisions>
## Implementation Decisions

### D-01: Repository interface — single global stream, not per-symbol
- **Single signal stream** for all signals (matches Notion doc: "all signals go to the same event stream so they are ordered")
- Symbol is an attribute ON the TradeSignal, not a stream partition key
- API: `Write(signal)` / `ReadPending(upToTime, filterSymbols?, filterNames?)`
- Filtering by symbol and name happens at read time, not at the storage level
- **IMPORTANT: Overrides research recommendation** of per-symbol streams — user explicitly wants one stream

### D-02: Signal delivery — NewSignals on TickDelta
- Add `repeated TradeSignalProto new_signals` field to TickDelta proto message
- Playground drains signal queue during `simulateTick`, same pattern as candles
- Strategy receives signals alongside candle data each tick
- Requires proto change + regen

### D-03: Clock gating — FIFOQueue pattern (Claude's discretion)
- **Recommended: FIFOQueue with clock cutoff** — same proven pattern as candle/trade delivery
- Signals sorted by timestamp in a FIFOQueue on Playground
- During simulateTick, drain all signals where timestamp <= clock.GetCurrentTime()
- Zero new abstractions needed

### D-04: Persistence — live write-through, sim batch-on-save
- **Live mode**: Every signal write-through persists to ESDB immediately (automatic, no flag needed)
- **Sim mode**: Signals stay in-memory only; persisted to ESDB as a batch when `SavePlayground` is called (which uses the existing `--save-to-db` flag from Phase 15)
- Stream naming for persisted signals: single stream (e.g. `trade-signals` or `trade-signals-{playground_id}`)

### Claude's Discretion
- FIFOQueue implementation details for signal delivery
- Exact ISignalRepository method signatures
- Stream naming convention for persisted sim signals
- Whether ReadPending returns TradeSignal or a wrapper with delivery metadata

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Signal Foundation (Phase 17)
- `src/go/eventmodels/trade_signal.go` — TradeSignal struct with SavedEvent implementation
- `src/go/eventmodels/signal_name.go` — SignalName typed constants
- `src/go/eventmodels/stream_names.go` — TradeSignalStream, NewTradeSignalStreamName
- `src/go/playground.proto` — TradeSignalProto message, signal_id on PlaceOrderRequest/Order

### Existing Patterns (FIFOQueue, TickDelta)
- `src/go/backtester-api/models/playground.go` — newCandlesQueue, newTradesQueue, invalidOrdersQueue (FIFOQueue usage)
- `src/go/backtester-api/models/tick_delta.go` — TickDelta struct (add NewSignals field)
- `src/go/eventmodels/fifo_queue.go` — FIFOQueue generic implementation
- `src/go/backtester-api/models/playground.go:simulateTick()` — where candles/trades are drained from queues

### Broker Interface Pattern
- `src/go/backtester-api/models/broker_interface.go` — IBroker interface pattern (model for ISignalRepository)

### ESDB Integration
- `src/go/eventconsumers/` — Existing ESDB consumer patterns
- `src/go/eventproducers/` — Existing ESDB producer patterns

### Research
- `.planning/research/ARCHITECTURE.md` — Signal delivery architecture, FIFOQueue pattern recommendation

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `FIFOQueue[T]` generic — already used for candles, trades, invalid orders
- `simulateTick()` on Playground — drains all queues, builds TickDelta
- `SavedEvent` interface on TradeSignal — ready for ESDB serialization
- `EsdbProducer` — existing ESDB write infrastructure

### Established Patterns
- Interface files: `*_interface.go` in models/ (IBroker, IDatabaseService, ILiveAccount)
- Mock implementations alongside production code
- FIFOQueue drain in simulateTick → included in TickDelta response

### Integration Points
- Playground struct — add `newSignalsQueue *eventmodels.FIFOQueue[*eventmodels.TradeSignal]`
- `simulateTick()` — drain signals queue, filter by clock time, add to TickDelta
- `tick_delta.go` — add `NewSignals []*eventmodels.TradeSignal` field
- Proto TickDelta — add `repeated TradeSignalProto new_signals` field
- `SavePlayground` flow — batch-persist signals to ESDB when saving sim playground

</code_context>

<specifics>
## Specific Ideas

- Single global signal stream per user's explicit request — overrides research recommendation
- Live write-through is automatic (no flag), sim batch-on-save uses existing --save-to-db flag
- Clock gating prevents lookahead: signal at 10:30:15 delivered during 10:31 tick, not 10:30 tick

</specifics>

<deferred>
## Deferred Ideas

- ESDBSignalRepository (live mode implementation) — Phase 20
- WriteSignal RPC — Phase 19
- Signal replay from persisted streams — Phase 23
- Per-consumer cursor tracking — not needed with FIFOQueue approach

</deferred>

---

*Phase: 18-signal-repository-sim-mode*
*Context gathered: 2026-03-31*
