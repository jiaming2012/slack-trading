# Technology Stack

**Project:** TradeSignal Framework (v3.0 milestone)
**Researched:** 2026-03-30
**Scope:** NEW additions only. Existing validated stack (Go, Python, OTel, Grafana, PostgreSQL, EventStoreDB, Metabase) is not re-researched.

---

## What This Milestone Adds

One new Python dependency and zero new Go dependencies. The TradeSignal framework is built almost entirely on existing infrastructure:

1. **`esdbclient` (Python)** -- Python EventStoreDB gRPC client for standalone datasource scripts
2. **New Go event types** -- `TradeSignal` struct following existing `SavedEvent` pattern
3. **New ESDB stream naming** -- `trade-signals-{name}` leveraging category projections
4. **Signal repository interface** -- in-memory (sim) and ESDB-backed (live) implementations
5. **Proto additions** -- `GetProcessedSignals` RPC endpoint

---

## Already Present -- DO NOT Add

These are confirmed in the codebase and cover the TradeSignal framework's needs.

| Technology | Version | Location | Role in TradeSignal |
|------------|---------|----------|---------------------|
| EventStore-Client-Go/v4 | v4.1.0 | `go.mod` | ESDB append, read, subscribe for signal persistence |
| EventStoreDB | 24.2.0-jammy | `docker-compose` | Signal event store. Category projections already enabled. |
| google/uuid | v1.6.0 | `go.mod` | Event stream IDs for signals |
| encoding/json | stdlib | all ESDB code | JSON serialization of signal events |
| OpenTelemetry | v1.27.0 (Go) / 1.40.0 (Python) | `go.mod` / `requirements.txt` | Trace context propagation in signal events |
| protobuf | v1.35.2 (Go) / 5.29.3 (Python) | `go.mod` / `requirements.txt` | Twirp RPC for signal query endpoint |
| Twirp | v8.1.3 (Go) / 0.0.7 (Python) | `go.mod` / `requirements.txt` | Already has `RecordSignal` RPC; extend for `GetProcessedSignals` |
| logrus | v1.9.3 | `go.mod` | Structured logging for signal lifecycle |
| BaseRequestEvent | -- | `eventmodels/base_request_event.go` | Metadata embedding for signal events |
| esdbConsumerStream[T] | -- | `eventconsumers/esdb_consumer_stream.go` | Generic typed consumer with replay support |
| EsdbProducer | -- | `eventproducers/esdb_producer.go` | Appending events with OTel trace context |

---

## New Stack Component

### Python EventStoreDB Client: `esdbclient`

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `esdbclient` | >=1.0 (latest) | Python datasource scripts appending TradeSignal events directly to EventStoreDB | Official Python gRPC client maintained by Event Store Ltd. Tested with ESDB 24.10 LTS and Python 3.10 -- both match this project exactly. |

**Confidence:** HIGH -- official client, tested against the project's Python version (3.10) and ESDB version (24.x).

**Why `esdbclient`:**
- Standalone datasource scripts must write to ESDB independently of the Go server. A datasource script running a cron job or streaming from a market data feed should not require the Go server to be running.
- gRPC protocol matches the Go client (consistency), unlike the deprecated AtomPub HTTP API.
- Supports `append_to_stream()`, `get_stream()`, `subscribe_to_all()` -- everything needed for signal production and diagnostic replay.

**Why NOT route signals through Go Twirp RPC:**
- Datasource scripts are intended to be standalone processes, potentially running on different machines or schedules.
- The Go server is the signal *consumer*, not the signal gateway. Decoupling producer from consumer is the core architectural goal of this milestone.
- ESDB is the shared bus; both Go and Python speak to it directly.

**Installation:**

```bash
# Conda env (preferred)
pip install esdbclient

# Or add to requirements.txt
esdbclient>=1.0

# Or add to grodt.yml under pip section
- esdbclient>=1.0
```

**Compatibility note:** `esdbclient` depends on `grpcio` and `protobuf`. The project already has `protobuf==5.29.3` pinned. Verify no `grpcio` version conflict with existing Python OTel packages (both use gRPC, should be compatible).

---

## No Other New Dependencies Required

| Capability Needed | Already Available Via |
|---|---|
| ESDB append/read/subscribe (Go) | `EventStore-Client-Go/v4` -- `esdb_producer.go`, `esdb_consumer.go` |
| JSON serialization of signals | `encoding/json` (stdlib) |
| Signal stream naming | `eventmodels.StreamName` type + constructor pattern in `stream_names.go` |
| Generic typed consumers | `esdbConsumer[T]` and `esdbConsumerStream[T]` generics |
| Event replay for simulation | `esdbConsumerStream.Replay()` method (already implemented) |
| Trace context in ESDB events | `EsdbMetadata.SpanContext` + `utils.SerializeTraceContext` |
| UUID event IDs | `google/uuid` |
| In-memory mock repo (sim mode) | Pure Go slice + `sync.RWMutex`, no dependencies |
| Proto definitions | `playground.proto` already has `RecordSignalRequest`; extend with new RPC |
| Pub/sub event routing | `eventpubsub` package + `EventBus` |

---

## Existing Patterns to Reuse

### Go Event Model Pattern

The codebase has a well-established pattern for ESDB events. `TradeSignal` must follow it exactly:

1. **Define struct** in `eventmodels/` implementing `SavedEvent` interface
2. **Embed `BaseRequestEvent`** for metadata support (`GetMetaData()`, `SetMetaData()`)
3. **Return `SavedEventParameters`** with stream name, event name, schema version
4. **Register stream name** as constant in `eventmodels/stream_names.go`
5. **Register event name** as constant in `eventmodels/event_names.go`
6. **Subscribe** in `esdb_producer.go` via `pubsub.Subscribe()` for the write path

Existing reference: `CreateSignalRequestEventV1DTO` in `new_signal_request_event_v1.go` already does this for the legacy signal system (writes to `AccountsStream`). The new `TradeSignal` gets its own dedicated stream.

### Consumer Variant Selection

| Variant | File | Pattern | Use For TradeSignal |
|---------|------|---------|---------------------|
| `esdbConsumer[T]` | `esdb_consumer.go` | Batch read into memory slice, mutex-guarded | Signal repository: load all signals at startup, serve reads |
| `esdbConsumerStream[T]` | `esdb_consumer_stream.go` | Channel-based streaming + replay | Live signal subscription + replay mode for simulations |

**Recommendation:** Use `esdbConsumerStream[T]` because:
- Has `Replay(ctx, startAtEventNumber)` method -- directly enables signal replay mode
- Has `Start(ctx)` for live subscription mode
- Carries `IsReplay` flag per event -- sim/live distinction built in
- Propagates OTel trace context via `EsdbMetadata`
- Channel-based (`GetEventCh()`) integrates cleanly with Go's concurrency model

### ESDB Producer Write Path

The existing `EsdbProducer.insert()` method handles:
- Setting event stream ID (UUID)
- Setting schema version from `SavedEventParameters`
- Serializing OTel trace context into `EsdbMetadata`
- Appending to the correct stream via `AppendToStream`

TradeSignal events from the Go server (e.g., when a strategy emits a signal) use this path. TradeSignal events from Python datasource scripts bypass this entirely and write to ESDB via `esdbclient`.

---

## EventStoreDB Stream Design

### Stream Naming Convention

```
trade-signals-{signal_name}
```

Examples:
- `trade-signals-stochastic_rsi_buy`
- `trade-signals-covered_call_entry`
- `trade-signals-mean_reversion_long`

**Why per-signal-name streams:**
- EventStoreDB's `$by_category` system projection (already enabled via `EVENTSTORE_RUN_PROJECTIONS=All` + `EVENTSTORE_START_STANDARD_PROJECTIONS=true`) splits on the first `-` by default
- All trade signal streams group under the `trade-signals` category
- Enables `$ce-trade-signals` category stream for querying ALL signals across types
- Matches existing naming pattern: `stock-ticks-{symbol}`, `option-chain-ticks-{symbol}`, `candles-{symbol}`, `fx-ticks-{symbol}`

**Stream name constructor (Go):**
```go
// In eventmodels/stream_names.go
const TradeSignalStream StreamName = "trade-signals"

func NewTradeSignalStreamName(signalName string) StreamName {
    return StreamName(fmt.Sprintf("%s-%s", TradeSignalStream, signalName))
}
```

### Signal Event Type

Use a single event type `TradeSignalCreated` for all signal events in the stream. The signal `Name` field inside the JSON payload provides the signal type -- the ESDB event type is the structural envelope, not the business domain discriminator.

---

## Signal Serialization

### TradeSignal Struct (Go)

```go
type TradeSignal struct {
    BaseRequestEvent
    Name       string                 `json:"name"`       // e.g. "stochastic_rsi_buy"
    Attributes map[string]interface{} `json:"attributes"` // flexible key-value pairs
    Timestamp  time.Time              `json:"timestamp"`  // when the signal was produced
    Symbol     StockSymbol            `json:"symbol"`     // which instrument
    Timeframe  uint                   `json:"timeframe"`  // candle period in seconds
    Source     SignalSource           `json:"source"`     // reuse existing SignalSource type
}

func (s *TradeSignal) GetSavedEventParameters() SavedEventParameters {
    return SavedEventParameters{
        StreamName:    NewTradeSignalStreamName(s.Name),
        EventName:     TradeSignalCreatedEventName,
        SchemaVersion: 1,
    }
}
```

**Why `map[string]interface{}` for Attributes:**
- Different signal types carry different data (RSI values, price levels, volume thresholds, moving average crossover points)
- A typed struct per signal type would require Go code changes and recompilation for every new signal -- defeating the purpose of decoupled datasource scripts
- Matches the project requirement: "Name + Attributes + Timestamp"
- JSON serialization to ESDB handles `map[string]interface{}` natively

**Tradeoff:** Loses compile-time type safety on attribute values. Mitigate with:
- `ValidateAttributes(name string, attrs map[string]interface{}) error` function per signal name
- Schema validation at write time in both Go producer and Python datasource scripts

### TradeSignal JSON (Python datasource)

```python
from esdbclient import EventStoreDBClient, NewEvent, StreamState
import json

client = EventStoreDBClient(uri="esdb://localhost:2113?tls=false")

signal_data = {
    "name": "stochastic_rsi_buy",
    "attributes": {"k_value": 15.2, "d_value": 18.7},
    "timestamp": "2026-03-30T14:30:00Z",
    "symbol": "AAPL",
    "timeframe": 300,
    "source": "PythonDatasource"
}

event = NewEvent(
    type="TradeSignalCreated",
    data=json.dumps(signal_data).encode("utf-8"),
    content_type="application/json",
)

client.append_to_stream(
    stream_name="trade-signals-stochastic_rsi_buy",
    current_version=StreamState.ANY,
    events=[event],
)
```

**Existing `SignalSource` values to extend:**
```go
// In eventmodels/signal_request_source.go -- add:
SignalSourcePythonDatasource SignalSource = "PythonDatasource"
```

---

## Signal Repository Interface

Two implementations, same interface, zero new dependencies:

```go
type ISignalRepository interface {
    Append(ctx context.Context, signal *TradeSignal) error
    GetByName(ctx context.Context, name string) ([]*TradeSignal, error)
    GetBySymbol(ctx context.Context, symbol StockSymbol) ([]*TradeSignal, error)
    Subscribe(ctx context.Context, name string) (<-chan *TradeSignal, error)
}
```

| Implementation | Backing | Use Case | Dependencies |
|----------------|---------|----------|-------------|
| `InMemorySignalRepository` | `[]TradeSignal` + `sync.RWMutex` | Simulation mode, unit tests | None (pure Go) |
| `ESDBSignalRepository` | Wraps `esdbConsumerStream[*TradeSignal]` | Live mode, replay mode | Existing ESDB client |

The `ESDBSignalRepository` delegates to `esdbConsumerStream` for subscription and replay, and to `EsdbProducer` for writes. No new ESDB client code needed.

---

## Proto Changes

The existing `RecordSignalRequest` is for the observability layer (recording that a signal was evaluated). The TradeSignal framework adds a new query endpoint:

```protobuf
// Add to playground.proto

message GetProcessedSignalsRequest {
    string playground_id = 1;
    optional string signal_name = 2;
    optional string symbol = 3;
}

message ProcessedSignal {
    string name = 1;
    string symbol = 2;
    string timestamp = 3;
    string attributes_json = 4;  // JSON-encoded map[string]interface{}
    string source = 5;
    uint32 timeframe = 6;
}

message GetProcessedSignalsResponse {
    repeated ProcessedSignal signals = 1;
}

// Add to PlaygroundService:
rpc GetProcessedSignals(GetProcessedSignalsRequest) returns (GetProcessedSignalsResponse);
```

**Why `attributes_json` as string:** Protobuf `map<string, string>` loses type information (numeric values become strings). Protobuf `google.protobuf.Struct` is awkward to work with in Go. A JSON string is pragmatic, matches ESDB storage format, and the Python client already has `json.loads()`.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Python ESDB client | `esdbclient` | `esdb-py` (andriykohut) | `esdbclient` is the official client maintained by Event Store Ltd, better test coverage (100% line+branch), supports Python 3.10 |
| Python ESDB client | `esdbclient` | HTTP REST via `requests` | AtomPub API is deprecated; gRPC is the supported protocol path |
| Signal write path (Python) | Direct to ESDB via `esdbclient` | Route through Go Twirp RPC | Adds coupling; datasource scripts should run independently of Go server lifecycle |
| Signal attributes | `map[string]interface{}` | Typed structs per signal | Too rigid; every new signal type requires Go code changes and recompilation |
| Signal attributes | `map[string]interface{}` | Protobuf `Any` or `Struct` | Over-engineered for this use case; JSON map is simpler and matches existing ESDB serialization |
| In-memory mock | Custom slice + mutex | `go-cache` or Redis | Overkill for simulation; signal repos are append-only within a session |
| Stream naming | `trade-signals-{name}` | Single `trade-signals` stream | Per-name streams enable ESDB category projections for cross-signal queries and targeted replay |
| Stream naming | `trade-signals-{name}` | `trade-signals-{symbol}-{name}` | Over-segmentation; filter by symbol within the stream. Category projection gives cross-signal view for free. |
| Event type naming | Single `TradeSignalCreated` | Per-signal event types | Signal name is a data field, not a structural concern. One event type simplifies consumer generics. |

---

## Confidence Assessment

| Area | Confidence | Basis |
|------|------------|-------|
| `esdbclient` compatibility | HIGH | Official docs state Python 3.10 + ESDB 24.x support; matches project versions |
| No new Go dependencies | HIGH | Direct codebase inspection of `go.mod`, ESDB consumer/producer patterns |
| Stream naming with category projections | HIGH | `EVENTSTORE_RUN_PROJECTIONS=All` + `START_STANDARD_PROJECTIONS=true` confirmed in docker-compose |
| `esdbConsumerStream[T]` for replay | HIGH | `Replay()` method exists and is tested in `esdb_consumer_stream.go` |
| `map[string]interface{}` for attributes | HIGH | Standard Go JSON pattern; already used in `eventservices/eventstoredb.go` `FetchAllData` |
| `esdbclient` + existing `grpcio` compatibility | MEDIUM | Both use gRPC; version alignment not yet tested in the grodt env |
| Proto `attributes_json` approach | MEDIUM | Pragmatic but unconventional; alternative is `google.protobuf.Struct` |

---

## Sources

- [esdbclient on PyPI](https://pypi.org/project/esdbclient/) -- Official Python gRPC client for EventStoreDB (HIGH confidence)
- [EventStore-Client-Go on GitHub](https://github.com/EventStore/EventStore-Client-Go) -- Go client v4 already in project (HIGH confidence)
- [EventStoreDB Category Projections](https://docs.kurrent.io/clients/tcp/dotnet/21.2/projections) -- `$by_category` splits on first dash (HIGH confidence)
- [EventStoreDB Go: Appending Events](https://docs-next.eventstore.com/clients/go/appending-events/) -- Go append patterns (HIGH confidence)
- [EventStoreDB Python Client Docs](https://docs-next.eventstore.com/clients/python/) -- Official Python client introduction (HIGH confidence)
- Codebase analysis: `esdb_consumer.go`, `esdb_consumer_stream.go`, `esdb_producer.go`, `eventservices/eventstoredb.go`, `eventmodels/stream_names.go`, `eventmodels/new_signal_request_event_v1.go`, `eventmodels/signal_request_source.go`, `eventmodels/signal_request_header.go`, `docker-compose` files

---

*Stack research for: TradeSignal Framework (v3.0) -- additions to existing Go + Python + OTel + Grafana + PostgreSQL + EventStoreDB + Metabase stack*
*Researched: 2026-03-30*
