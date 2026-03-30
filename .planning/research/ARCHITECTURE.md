# Architecture Patterns: TradeSignal Framework Integration

**Domain:** Event-driven trading signal framework for existing Go+Python platform
**Researched:** 2026-03-30
**Confidence:** HIGH (based on direct codebase analysis of all integration points)

## Recommended Architecture

### Overview

The TradeSignal framework introduces a **signal-first event stream** between datasources and strategies. Today, Python strategies both detect signals AND place orders in `on_tick()`. The new architecture separates these concerns:

1. **Datasource scripts** produce `TradeSignal` events via Twirp RPC to Go server, persisted to EventStoreDB
2. **Strategies** consume signals from a `SignalRepository` interface and decide whether to place orders
3. **Replay mode** reads persisted signals with clock synchronization to reproduce simulations deterministically

This is an **additive change** to the existing two-process architecture. Go server gains a new domain type and RPC endpoints. Python gains a signal abstraction layer and standalone datasource scripts.

### Component Boundaries

| Component | Location | Responsibility | New/Modified | Communicates With |
|-----------|----------|---------------|--------------|-------------------|
| `TradeSignal` struct | `src/go/eventmodels/trade_signal.go` | Domain type: Name + Attributes + Timestamp + Symbol | **NEW** | Serialized to/from EventStoreDB, returned via RPC |
| `TradeSignalStream` | `src/go/eventmodels/stream_names.go` | New ESDB stream name constant | **MODIFIED** (add constant) | EventStoreDB |
| Signal ESDB consumer | `src/go/eventconsumers/signal_consumer.go` | Subscribe to trade-signals stream, maintain in-memory cache | **NEW** | EventStoreDB, RPC handlers |
| `WriteSignal` RPC | `src/go/backtester-api/router/grpc.go` | Accept signals from Python datasources, persist to ESDB | **NEW endpoint** | Python datasources, EventStoreDB |
| `GetSignals` RPC | `src/go/backtester-api/router/grpc.go` | Query processed signals per strategy/playground | **NEW endpoint** | Python strategies, signal consumer |
| `PlaceOrderRequest.signal_id` | `src/go/playground.proto` | Link each order to its originating signal | **MODIFIED** (add field) | Python strategies, Go order pipeline |
| `ISignalRepository` (Go) | `src/go/backtester-api/models/signal_repository_interface.go` | Interface: Write, Read, Query signals | **NEW** | DatabaseService, RPC handlers |
| `InMemorySignalRepository` | `src/go/backtester-api/models/signal_repository_memory.go` | Sim/replay: signals stored in memory, delivered by clock | **NEW** | Playground |
| `ESDBSignalRepository` | `src/go/backtester-api/models/signal_repository_esdb.go` | Live: signals persisted to EventStoreDB | **NEW** | EventStoreDB via EsdbProducer |
| `SignalRepository` (Python) | `src/clients/python/engine/signal_repository.py` | Python-side abstraction over signal source | **NEW** | Go server via Twirp RPC |
| Datasource scripts | `src/clients/python/datasources/` | Standalone scripts producing TradeSignal events | **NEW directory** | Go server via WriteSignal RPC |
| `BaseStrategy.on_signal()` | `src/clients/python/strategies/base_strategy.py` | New abstract method: react to a signal | **MODIFIED** | SignalRepository, BacktesterPlaygroundClient |

### Data Flow

#### Current Flow (to be deprecated per strategy)

```
Python on_tick() {
    candles = tick_delta.new_candles
    if detect_signal(candles):       # Signal detection embedded in strategy
        playground.place_order(...)   # Order placement coupled to detection
}
```

#### New Flow: Live Mode

```
                          EventStoreDB
                       "trade-signals-{symbol}"
                              |
     Python Datasource ------+------> Go Signal Consumer (in-memory cache)
     (writes signals via      |              |
      WriteSignal RPC)        |              v
                              |        GetSignals RPC
                              |              |
                              v              v
                      Python Strategy consumes signals
                              |
                              v
                      PlaceOrder RPC (with signal_id)
                              |
                              v
                      Go OrderRecord (signal_id stored in Postgres)
```

#### New Flow: Replay Mode

```
     EventStoreDB (historical signals)
              |
              v
     Go reads all signals for time range at Playground creation
     Loads into InMemorySignalRepository
              |
              v
     Clock.Add() advances time
     Playground delivers signals where Timestamp <= clock.CurrentTime
              |
              v
     Python strategy receives signals via TickDelta.new_signals
              |
              v
     PlaceOrder with signal_id
```

## New Types and Interfaces

### TradeSignal Struct (Go)

Lives in `src/go/eventmodels/trade_signal.go` because it is a shared domain type used across packages (same pattern as `SignalV2`, `TrackerV3`, `Candle`).

```go
package eventmodels

import (
    "time"
    "github.com/google/uuid"
)

type TradeSignal struct {
    BaseRequestEvent
    ID         uuid.UUID         `json:"id"`
    Name       string            `json:"name"`       // e.g. "mean_reversion_dip", "covered_call_entry"
    Symbol     StockSymbol       `json:"symbol"`
    Timestamp  time.Time         `json:"timestamp"`
    Attributes map[string]string `json:"attributes"` // strategy-specific key-value pairs
    streamName StreamName        `json:"-"`
}

func (s *TradeSignal) GetSavedEventParameters() SavedEventParameters {
    return SavedEventParameters{
        StreamName:    s.streamName,
        EventName:     TradeSignalEventName,
        SchemaVersion: 1,
    }
}
```

Key design decisions:
- Implements `SavedEvent` interface -- plugs directly into existing `EsdbProducer.insert()` and `esdbConsumer` generic infrastructure
- `Attributes map[string]string` matches the existing pattern on `PlaceOrderRequest.attributes` (field 15) and `OrderRecord.Attributes`
- `streamName` is set dynamically per symbol: `"trade-signals-AAPL"` -- follows the `NewCandleStreamName()` pattern in `stream_names.go`
- UUID `ID` enables signal-to-order linking

### Relationship to Existing Signal Types

The codebase has several existing signal-related types:

| Existing Type | Purpose | Reuse? |
|---------------|---------|--------|
| `SignalV2` | Boolean "satisfied/not satisfied" state for named signals | **No** -- too simple, no attributes, no ESDB persistence |
| `SignalTriggeredEvent` | Event fired when a signal triggers (Timestamp, Symbol, SignalName) | **No** -- pub/sub event, not domain entity |
| `SignalDecision` (Python) | Telemetry record for strategy decisions (place/skip) | **No** -- telemetry, not domain signal |
| `RecordSignalRequest` | RPC for incrementing Prometheus counter | **No** -- telemetry-only |
| `TrackerV3` / `SignalTrackerV2` | Event-sourced tracker with signal metadata | **Partial** -- similar ESDB pattern, but TrackerV3 wraps multiple tracker types |

`TradeSignal` is a new first-class domain type because none of the existing types combine: named signal + attributes + timestamp + ESDB persistence + signal-to-order linking.

### ISignalRepository Interface (Go)

Lives in `src/go/backtester-api/models/signal_repository_interface.go` (follows `broker_interface.go`, `live_account_interface.go` pattern).

```go
package models

import (
    "time"
    "github.com/google/uuid"
    "github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type SignalQueryOpts struct {
    Name   *string
    Symbol *eventmodels.StockSymbol
    After  *time.Time
    Before *time.Time
}

type ISignalRepository interface {
    // Write stores a signal (called by WriteSignal RPC or replay loader)
    Write(signal *eventmodels.TradeSignal) error

    // ReadPending returns signals not yet consumed, up to the given timestamp
    // Used by NextTick to deliver signals synchronized with clock
    ReadPending(upTo time.Time) []*eventmodels.TradeSignal

    // MarkConsumed marks a signal as consumed by a strategy
    MarkConsumed(signalID uuid.UUID) error

    // Query returns signals matching filters (for GetSignals RPC)
    Query(opts SignalQueryOpts) []*eventmodels.TradeSignal
}
```

Two implementations:
- **InMemorySignalRepository**: For simulator/replay. Holds signals in a sorted slice, delivers based on clock time. `ReadPending` scans forward from a cursor. No external dependencies.
- **ESDBSignalRepository**: For live mode. `Write` calls `EsdbProducer.Save()`. `ReadPending` reads from the ESDB subscription consumer's in-memory cache (same pattern as `esdbConsumer.GetSavedEvents()`). Thread-safe via mutex.

### Signal Delivery in Replay Mode -- Clock Synchronization

The existing `Clock` struct (in `backtester-api/models/clock.go`) advances time via `Clock.Add(duration)`. During `nextTick()` in the Playground, the clock advances and candles are delivered from `CandleMasterRepository` where `candle.Timestamp <= clock.CurrentTime`.

Signals follow the identical pattern:

1. At playground creation for replay, Go reads all historical signals from ESDB via `eventservices.FetchAll[*eventmodels.TradeSignal](ctx, esdbClient, instance)` for the given time range
2. Signals are loaded into `InMemorySignalRepository`, sorted by timestamp
3. Each `NextTick` call: after clock advances, `signalRepo.ReadPending(clock.CurrentTime)` returns signals whose `Timestamp <= CurrentTime` that haven't been delivered yet
4. These signals are enqueued to `newSignalsQueue` and included in the `TickDelta` response as `repeated TradeSignalProto new_signals`
5. Python strategy receives signals alongside candles in the same tick

This mirrors the existing delivery mechanism for `newCandlesQueue` (`*eventmodels.FIFOQueue[*BacktesterCandle]`) and `newTradesQueue` (`*eventmodels.FIFOQueue[*TradeRecord]`) on the Playground struct.

### Proto Changes

```protobuf
// New messages
message TradeSignalProto {
    string id = 1;
    string name = 2;
    string symbol = 3;
    string timestamp = 4;
    map<string, string> attributes = 5;
}

message WriteSignalRequest {
    string name = 1;
    string symbol = 2;
    string timestamp = 3;
    map<string, string> attributes = 4;
    optional string playground_id = 5;  // optional: associate with playground
}

message GetSignalsRequest {
    string playground_id = 1;
    optional string name = 2;
    optional string symbol = 3;
}

message GetSignalsResponse {
    repeated TradeSignalProto signals = 1;
}

// Modified existing messages:

message TickDelta {
    // ... existing fields 1-10 ...
    repeated TradeSignalProto new_signals = 11;  // NEW
}

message PlaceOrderRequest {
    // ... existing fields 1-16 ...
    optional string signal_id = 17;  // NEW: UUID linking order to originating signal
}
```

### Python Signal Repository

```python
# src/clients/python/engine/signal_repository.py
class SignalRepository:
    """Wraps signal access for strategies.

    In live mode: datasources call write() via WriteSignal RPC,
                  strategies poll via GetSignals RPC
    In sim/replay mode: signals arrive in TickDelta.new_signals
    """
    def write(self, client, name, symbol, timestamp, attributes):
        """Used by datasource scripts to produce signals via WriteSignal RPC."""
        ...

    def get_pending(self, tick_delta):
        """Extract signals from tick delta response (sim/replay mode)."""
        return tick_delta.new_signals
```

### Datasource Script Pattern

```python
# src/clients/python/datasources/mean_reversion_signals.py
"""Standalone datasource: produces mean_reversion_dip signals.

Connects to Go server, creates a playground (or uses existing live playground),
runs tick loop, detects signals from candle data, writes TradeSignal
events via WriteSignal RPC. Does NOT place orders.

Usage:
  python -m datasources.mean_reversion_signals --symbol AAPL --playground-id <uuid>
"""
```

## Integration Points with Existing Architecture

### 1. EventStoreDB Integration (Existing Pattern -- Zero New Infrastructure)

The codebase has a complete generic ESDB event pipeline:
- `SavedEvent` interface + `SavedEventParameters` for stream/event metadata (in `eventmodels/saved_event.go`)
- `EsdbProducer.insert()` serializes events with OTel trace context to ESDB (in `eventproducers/esdb_producer.go`)
- `esdbConsumer[T]` generic subscribes to streams with automatic replay from last position (in `eventconsumers/esdb_consumer.go`)
- `eventservices.FetchAll[T]()` reads all events from a stream (in `eventservices/eventstoredb.go`)
- `StreamName` constants in `eventmodels/stream_names.go`

TradeSignal plugs in by:
- Adding `TradeSignalStream StreamName = "trade-signals"` to `stream_names.go`
- Adding `NewTradeSignalStreamName(symbol string) StreamName` helper function
- Adding `TradeSignalEventName EventName = "TradeSignalEvent"` to `event_names.go`
- Implementing `SavedEvent` on `TradeSignal` struct

The generic `esdbConsumer[*eventmodels.TradeSignal]` type alias handles subscription and replay with zero custom code (same as `OptionContractConsumer = esdbConsumer[*eventmodels.OptionContractV1]`).

### 2. Playground Integration

The `Playground` struct (in `backtester-api/models/playground.go`) gains:
- `signalRepo ISignalRepository` field (set at construction based on environment, like `OptionsBroker`)
- `newSignalsQueue *eventmodels.FIFOQueue[*eventmodels.TradeSignal]` for tick delivery (same pattern as `newCandlesQueue` line 48 and `newTradesQueue` line 49)

During `simulateTick()`, after clock advance and candle delivery, pending signals are dequeued and included in the TickDeltaEvent list.

### 3. Order-Signal Linkage

`PlaceOrderRequest` (proto field 17) gains `signal_id`. In Go:
- `CreateOrderRequest` (in `backtester-api/models/create_order_request.go`) gains `SignalID *uuid.UUID`
- `OrderRecord` gains `SignalID *uuid.UUID` GORM column: `gorm:"column:signal_id;type:uuid;index:idx_signal_id"`
- GORM auto-migration adds the column on next server startup

The "single-signal-per-order" rule is enforced in `PlaceOrder` validation in `grpc.go`: if `signal_id` is provided, verify it exists in the signal repository and has not already been used by another order in this playground.

### 4. RPC Server Integration

The `Server` struct in `grpc.go` already has `dbService` and `optionsClient`. For signal support:
- `WriteSignal` handler: validates signal, creates `TradeSignal` struct, calls `EsdbProducer.Save()` via pub/sub or direct reference
- `GetSignals` handler: queries signal repository (ESDB consumer cache or playground's in-memory repo) with filters

Both follow existing RPC handler patterns in `grpc.go` (receive request, validate, delegate to service, return proto response).

### 5. Python Strategy Migration Path

Each strategy (MeanReversion, CoveredCall, etc.) currently has signal detection logic inside `on_tick()`. Migration for each strategy:

1. Extract signal detection logic into a standalone datasource script in `src/clients/python/datasources/`
2. Strategy receives signals via `tick_delta.new_signals` instead of computing them internally
3. Strategy calls `place_order(signal_id=signal.id, ...)` to link the order to the signal
4. Original strategy file moved to `src/clients/python/deprecated/`

The `BaseStrategy` class (in `strategies/base_strategy.py`) gains:
- `on_signal(signal)` -- optional method called when signals arrive in tick delta
- Default implementation: no-op (backward compatible with strategies not yet migrated)

The engine loop (in `engine/trading_engine.py`, `run_strategy()`) checks for `new_signals` in tick delta and routes them to `strategy.on_signal()`.

### 6. RecordSignal RPC vs WriteSignal RPC

These serve distinct purposes and both continue to exist:

| RPC | Purpose | Who Calls | Side Effect |
|-----|---------|-----------|-------------|
| `RecordSignal` | Telemetry: "strategy made a decision" | Strategies via `base_strategy._send_signal_rpc()` | Prometheus counter increment |
| `WriteSignal` | Domain: "datasource produced a tradeable signal" | Datasource scripts | ESDB event persisted |

`RecordSignal` may eventually be deprecated once all signal decisions flow through `WriteSignal` and the signal-to-order chain provides richer telemetry.

## Patterns to Follow

### Pattern 1: SavedEvent for ESDB Types
**What:** All EventStoreDB-persisted types implement `SavedEvent` interface with `GetSavedEventParameters()` returning stream name, event name, and schema version.
**When:** Creating any new event type for ESDB.
**Example:** `TrackerV3`, `OptionContractV1`, `StockTickV1` -- all follow this pattern.
**Why:** Enables generic `esdbConsumer[T]`, `EsdbProducer.insert()`, `eventservices.FetchAll[T]()` with zero per-type plumbing.

### Pattern 2: FIFOQueue for Tick-Synchronized Delivery
**What:** Use `eventmodels.FIFOQueue[T]` for delivering time-ordered items during tick processing.
**When:** Signals need to arrive at the strategy at the correct simulated time.
**Example:** `newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]` on Playground.
**Why:** Thread-safe channel-based queue, already proven for candles and trades.

### Pattern 3: Interface-Based Repository Swapping
**What:** Define `ISignalRepository` interface, swap implementations based on playground environment.
**When:** Simulator uses in-memory, live uses ESDB.
**Example:** `IBroker` (TradierBroker vs MockBroker), `ILiveAccount`, `IOptionsBroker` -- all follow this.
**Why:** Clean testability, environment-based behavior without conditionals.

### Pattern 4: Proto Attributes Map for Extensibility
**What:** `map<string, string> attributes` on TradeSignal.
**When:** Signal-specific metadata varies by strategy (e.g., mean reversion needs `deviation_level`, covered call needs `strike_price`).
**Example:** `PlaceOrderRequest.attributes` (proto field 15), `OrderRecord.Attributes` GORM JSONB.
**Why:** Avoids proto schema changes for each new signal type. Strategy-specific data stays flexible.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Python Writing Directly to EventStoreDB
**What:** Python datasource scripts connecting to ESDB directly (bypassing Go server).
**Why bad:** Bypasses Go server's OTel instrumentation, schema validation, and audit trail. Two processes writing to ESDB creates coordination headaches. The Go ESDB client library is already wired with trace context injection.
**Instead:** Python writes via `WriteSignal` Twirp RPC. Go server handles ESDB persistence. Single writer principle.

### Anti-Pattern 2: Signals in PostgreSQL
**What:** Storing TradeSignal events in GORM/Postgres alongside orders and playgrounds.
**Why bad:** Signals are append-only event data, not relational. ESDB provides stream subscription, temporal ordering, and replay natively via `esdbConsumer`. Postgres would require polling or change notifications.
**Instead:** Use EventStoreDB for signal storage. Query via ESDB stream reads (`eventservices.FetchAll[T]()`).

### Anti-Pattern 3: Embedding Signal Detection in the Go Server
**What:** Having the Go server detect signals during `simulateTick()`.
**Why bad:** Go server manages state and execution, not strategy logic. Signal detection belongs in Python where strategy expertise lives. Mixing concerns makes replay mode impossible (signals would be regenerated rather than replayed).
**Instead:** Datasource scripts (Python) produce signals. Go server stores and delivers them.

### Anti-Pattern 4: Breaking the Existing RecordSignal Flow During Migration
**What:** Removing or replacing `RecordSignal` RPC before all strategies are migrated.
**Why bad:** Existing strategies use `record_decision()` for Prometheus telemetry. Breaking this loses observability during incremental migration.
**Instead:** Keep `RecordSignal` for telemetry alongside `WriteSignal` for domain events. Migrate incrementally, strategy by strategy.

### Anti-Pattern 5: Global Signal Stream Instead of Per-Symbol Streams
**What:** Single ESDB stream `"trade-signals"` for all symbols.
**Why bad:** ESDB streams are optimized for sequential reads. A global stream forces consumers to filter by symbol, wasting read bandwidth. Per-symbol streams enable targeted subscriptions and parallel processing.
**Instead:** `NewTradeSignalStreamName("AAPL")` returns `"trade-signals-AAPL"`. Follow the `NewCandleStreamName` pattern.

## Suggested Build Order (Dependency-Driven)

| Phase | What | Depends On | Unlocks |
|-------|------|-----------|---------|
| 1 | `TradeSignal` struct + ESDB stream constants + `SavedEvent` impl | Nothing | Everything else |
| 2 | `ISignalRepository` interface + `InMemorySignalRepository` | Phase 1 | Sim/test mode, unit tests |
| 3 | `WriteSignal` + `GetSignals` proto messages + RPC endpoints | Phase 1 | Python datasource integration |
| 4 | Signal delivery in `TickDelta` + `newSignalsQueue` on Playground | Phases 1, 2 | Replay mode, strategy consumption |
| 5 | `signal_id` on PlaceOrderRequest + CreateOrderRequest + OrderRecord | Phase 1 | Signal-order audit trail |
| 6 | `ESDBSignalRepository` implementation | Phases 1, 2, 3 | Live mode persistence |
| 7 | Python `SignalRepository` + datasource script skeleton | Phase 3 | Strategy migration |
| 8 | First strategy migration (simplest strategy) | Phases 4, 5, 7 | End-to-end validation |
| 9 | Remaining strategy migrations + move originals to deprecated/ | Phase 8 | Full adoption |
| 10 | Replay mode integration tests | Phases 4, 6 | Deterministic replay validation |
| 11 | Telemetry: signals in OTel/Grafana + missing signal alerts | Phase 6 | Production monitoring |

**Phase ordering rationale:**
- Phases 1-5 are Go-side foundation: purely additive, no breaking changes to existing code
- Phase 3 before Phase 4: proto generation must happen before TickDelta can include signals
- Phase 6 after Phase 2: ESDB repo implements the same interface as in-memory, so interface must be stable
- Phase 8 before Phase 9: validate full pipeline with one strategy before migrating the rest
- Phases 10-11 are production-readiness: tests and monitoring

## Files to Create (New)

| File | Package | Purpose |
|------|---------|---------|
| `src/go/eventmodels/trade_signal.go` | eventmodels | TradeSignal domain type implementing SavedEvent |
| `src/go/backtester-api/models/signal_repository_interface.go` | models | ISignalRepository interface definition |
| `src/go/backtester-api/models/signal_repository_memory.go` | models | InMemorySignalRepository for sim/replay |
| `src/go/backtester-api/models/signal_repository_memory_test.go` | models | Unit tests for in-memory repo |
| `src/go/backtester-api/models/signal_repository_esdb.go` | models | ESDBSignalRepository for live mode |
| `src/clients/python/engine/signal_repository.py` | engine | Python signal abstraction |
| `src/clients/python/datasources/__init__.py` | datasources | New package for standalone datasource scripts |

## Files to Modify

| File | Change | Risk |
|------|--------|------|
| `src/go/eventmodels/stream_names.go` | Add `TradeSignalStream` constant + `NewTradeSignalStreamName()` | Low -- additive |
| `src/go/eventmodels/event_names.go` | Add `TradeSignalEventName` constant | Low -- additive |
| `src/go/playground.proto` | Add `TradeSignalProto`, `WriteSignalRequest`, `GetSignalsRequest/Response`, new fields on `TickDelta` + `PlaceOrderRequest` | Medium -- regenerates stubs |
| `src/go/backtester-api/router/grpc.go` | Add `WriteSignal()` + `GetSignals()` RPC handlers | Medium -- large file (1280+ lines) |
| `src/go/backtester-api/models/playground.go` | Add `signalRepo` + `newSignalsQueue` fields, signal delivery in `simulateTick` | Medium -- large file (2900+ lines), core logic |
| `src/go/backtester-api/models/create_order_request.go` | Add `SignalID *uuid.UUID` field | Low -- additive |
| `src/go/backtester-api/models/order_record.go` | Add `SignalID` GORM field | Low -- additive, auto-migrated |
| `src/clients/python/strategies/base_strategy.py` | Add `on_signal()` optional method | Low -- default no-op, backward compatible |
| `src/clients/python/engine/trading_engine.py` | Signal routing in `run_strategy()` tick loop | Low -- additive check for new_signals |
| `src/clients/python/engine/client.py` | Add `write_signal()` + `get_signals()` RPC wrappers | Low -- additive methods |

## Scalability Considerations

| Concern | Current Scale | At Scale (100+ signals/min) | Mitigation |
|---------|--------------|---------------------------|------------|
| ESDB stream size | Small (trackers stream has hundreds of events) | Grows linearly; ESDB handles millions per stream | Per-symbol stream partitioning |
| In-memory signal cache | N/A | Memory grows with signal volume | Bounded time window (e.g., last 24h) in InMemorySignalRepository |
| Signal-order validation | N/A | O(n) scan for duplicate signal_id per playground | PostgreSQL index on `order_records.signal_id` |
| Replay data loading | N/A | Large signal streams for long replay periods | `eventservices.FetchAll[T]()` already handles 4096-event page chunks |
| Proto message size | TickDelta currently has candles + trades | Signals add ~100-200 bytes each | Negligible at expected signal rates |

## Sources

- Direct codebase analysis (HIGH confidence):
  - `src/go/eventmodels/saved_event.go` -- SavedEvent interface pattern
  - `src/go/eventmodels/stream_names.go` -- stream naming conventions
  - `src/go/eventmodels/signalv2.go` -- existing signal model (informed but not reused)
  - `src/go/eventconsumers/esdb_consumer.go` -- generic ESDB consumer with replay
  - `src/go/eventproducers/esdb_producer.go` -- ESDB write pattern with OTel trace context
  - `src/go/eventservices/eventstoredb.go` -- FetchAll generic stream reader
  - `src/go/backtester-api/models/playground.go` -- tick delivery via FIFOQueue, environment-based interface swapping
  - `src/go/backtester-api/models/clock.go` -- time advancement for replay synchronization
  - `src/go/backtester-api/models/broker_interface.go` -- interface swapping pattern
  - `src/go/backtester-api/router/grpc.go` -- existing RPC handler patterns (PlaceOrder, NextTick, RecordSignal)
  - `src/go/backtester-api/models/create_order_request.go` -- order request structure
  - `src/go/playground.proto` -- existing proto schema (PlaceOrderRequest, TickDelta, RecordSignalRequest)
  - `src/clients/python/strategies/base_strategy.py` -- strategy interface with record_decision/RecordSignal
  - `src/clients/python/engine/trading_engine.py` -- run_strategy tick loop
  - `src/clients/python/engine/types.py` -- SignalDecision, OpenSignal types
  - `.planning/PROJECT.md` -- v3.0 milestone requirements

---

*Architecture research for: TradeSignal framework integration on Go+Python trading platform*
*Researched: 2026-03-30*
