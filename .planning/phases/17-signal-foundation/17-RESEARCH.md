# Phase 17: Signal Foundation - Research

**Researched:** 2026-03-30
**Domain:** Go domain type definition, protobuf schema extension, GORM migration
**Confidence:** HIGH

## Summary

Phase 17 is a purely additive foundation phase: define the `TradeSignal` type in Go, add `TradeSignalProto` to the proto schema, add `signal_id` to `PlaceOrderRequest`/`OrderRecord`, and define a typed signal name registry. No persistence layer, no repository interface, no strategy changes -- those are Phases 18-22.

The codebase has well-established patterns for every aspect of this work: typed string constants (`PlaygroundEnvironment`, `StockSymbol`), proto `map<string, string>` attributes (already on `PlaceOrderRequest` field 15 and `Order` field 23), GORM auto-migration for new columns, and `SavedEvent` interface for ESDB-ready types. The implementation follows these patterns exactly.

**Primary recommendation:** Create 3-4 new files (trade_signal.go, signal_name.go in eventmodels; proto changes) and modify 3 existing files (order_record.go, create_order_request.go, playground.proto). All changes are additive with zero breaking changes to existing code. Run `task gen:proto` after proto changes and `task test` to verify nothing breaks.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Signal name registry uses `type SignalName string` with a `const` block. Examples: `SignalMACrossover SignalName = "ma_crossover"`, `SignalStartOfWeek SignalName = "start_of_week"`. Python uses matching string literals.
- D-02: Attributes use `map<string, string>` in proto (matches existing PlaceOrderRequest.attributes pattern). Go internal model uses `map[string]interface{}` with JSON marshaling for ESDB storage.
- D-03: signal_id is Go-server-generated UUID. Stored on OrderRecord via GORM auto-migrate. Optional during migration (Phases 21-22), required after Phase 22.
- D-04: RecordSignal RPC is deprecated (comment only), not removed. Keep functional. WriteSignal replacement comes in Phase 19.

### Claude's Discretion
- Exact signal_id generation strategy (Go server UUID recommended)
- TradeSignal struct field naming and package location
- Whether to add convenience methods for attribute parsing in Phase 17 or defer
- Proto field numbers for new messages

### Deferred Ideas (OUT OF SCOPE)
- WriteSignal RPC -- Phase 19
- Signal repositories (in-memory, ESDB) -- Phase 18
- Signal delivery via TickDelta -- Phase 18
- Python datasource scripts -- Phase 19
- Strategy migration -- Phases 21-22
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SIG-01 | TradeSignal struct (Name + Attributes + Timestamp) defined in Go models and proto | New `trade_signal.go` in eventmodels with SavedEvent impl; new `TradeSignalProto` message in proto |
| SIG-02 | Every PlaceOrderRequest includes a signal_id linking back to the originating TradeSignal | Proto field 17 on PlaceOrderRequest; `SignalID *uuid.UUID` on CreateOrderRequest and OrderRecord |
| SIG-03 | Signal attributes use `map[string]interface{}` for flexible, schema-free signal types | Proto uses `map<string, string>` (wire format); Go struct uses `map[string]interface{}` internally with JSON marshaling |
</phase_requirements>

## Standard Stack

### Core

No new dependencies required. All work uses existing libraries already in go.mod.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/google/uuid` | v1.6.0 | UUID generation for signal_id and TradeSignal.ID | Already in go.mod, used for playground IDs |
| `google.golang.org/protobuf` | v1.35.2 | Proto codegen for TradeSignalProto | Already in go.mod |
| `gorm.io/gorm` | v1.25.12 | Auto-migration for signal_id column on OrderRecord | Already in go.mod |
| `github.com/stretchr/testify` | v1.9.0 | Unit test assertions | Already in go.mod |

### Supporting

None needed -- this phase is purely additive type definitions.

### Alternatives Considered

None -- all decisions are locked per CONTEXT.md.

## Architecture Patterns

### Recommended File Structure

```
src/go/eventmodels/
  trade_signal.go          # NEW: TradeSignal struct + SavedEvent impl
  signal_name.go           # NEW: type SignalName string + const block
  stream_names.go          # MODIFIED: add TradeSignalStream constant
  event_names.go           # MODIFIED: add TradeSignalEventName constant
src/go/backtester-api/models/
  order_record.go          # MODIFIED: add SignalID field
  create_order_request.go  # MODIFIED: add SignalID field
src/go/
  playground.proto         # MODIFIED: add TradeSignalProto, signal_id on PlaceOrder
src/go/backtester-api/router/
  grpc.go                  # MODIFIED: deprecation comment on RecordSignal, pass signal_id in PlaceOrder
```

### Pattern 1: Typed String Constant Registry (SignalName)

**What:** `type SignalName string` with a const block, following `PlaygroundEnvironment` in `playground_environment.go`
**When to use:** Signal name must be a known, registered constant -- not freeform.
**Example from codebase:**

```go
// src/go/backtester-api/models/playground_environment.go (existing pattern)
type PlaygroundEnvironment string

const (
    PlaygroundEnvironmentSimulator PlaygroundEnvironment = "simulator"
    PlaygroundEnvironmentLive      PlaygroundEnvironment = "live"
    PlaygroundEnvironmentReconcile PlaygroundEnvironment = "reconcile"
)
```

**Apply as:**

```go
// src/go/eventmodels/signal_name.go
package eventmodels

type SignalName string

const (
    SignalMACrossover   SignalName = "ma_crossover"
    SignalStartOfWeek   SignalName = "start_of_week"
    SignalCoveredCall    SignalName = "covered_call"
    SignalMeanReversion SignalName = "mean_reversion"
)

func (s SignalName) Validate() error {
    switch s {
    case SignalMACrossover, SignalStartOfWeek, SignalCoveredCall, SignalMeanReversion:
        return nil
    default:
        return fmt.Errorf("unknown signal name: %s", s)
    }
}
```

**Package location:** `eventmodels` (not `models`) because SignalName is a shared domain type used across packages, matching `StockSymbol`, `OptionSymbol`, `StreamName`.

### Pattern 2: SavedEvent Implementation for TradeSignal

**What:** TradeSignal embeds `BaseRequestEvent` and implements `SavedEvent` interface.
**When to use:** Any type that will eventually be persisted to EventStoreDB.
**Example from codebase:**

```go
// src/go/eventmodels/tracker_v3.go (existing pattern)
type TrackerV3 struct {
    BaseRequestEvent
    Type       TrackerType `json:"type"`
    streamName StreamName  `json:"-"`
}

func (c *TrackerV3) GetSavedEventParameters() SavedEventParameters {
    return SavedEventParameters{
        StreamName:    c.streamName,
        EventName:     CreateTrackerEvent,
        SchemaVersion: 3,
    }
}
```

**Apply as:**

```go
// src/go/eventmodels/trade_signal.go
type TradeSignal struct {
    BaseRequestEvent
    ID         uuid.UUID              `json:"id"`
    Name       SignalName             `json:"name"`
    Symbol     StockSymbol            `json:"symbol"`
    Timestamp  time.Time              `json:"timestamp"`
    Attributes map[string]interface{} `json:"attributes"`
    streamName StreamName             `json:"-"`
}
```

Note: `Attributes` is `map[string]interface{}` in the Go struct (per SIG-03 and D-02) for internal flexibility. The proto message uses `map<string, string>` for wire format. Conversion happens at the RPC boundary (Phase 19 when WriteSignal is implemented).

### Pattern 3: GORM Column Addition via Auto-Migrate

**What:** Add a nullable UUID column to OrderRecord. GORM auto-migrate adds the column non-destructively on next server startup.
**When to use:** Adding a new field to an existing GORM model.
**Example from codebase:** OrderRecord already has nullable fields like `ExternalOrderID *uint`, `ClientRequestID *string`, `PreviousBalance *float64`.

```go
// Addition to OrderRecord struct
SignalID *uuid.UUID `gorm:"column:signal_id;type:uuid;index:idx_signal_id" copier:"must,nopanic"`
```

### Pattern 4: Proto Field Numbering

**What:** Add fields using next available numbers on existing messages.
**Verified field numbers:**
- `PlaceOrderRequest`: last field is `trace_id = 16` -- use **17** for `signal_id`
- `TickDelta`: last field is `positions = 10` -- **11** reserved for `new_signals` (Phase 18, not this phase)
- `Order`: last field is `pl = 25` -- use **26** for `signal_id` (to return in responses)

### Anti-Patterns to Avoid

- **Do not add signal_id as required in proto**: Use `optional string signal_id` so existing clients without signals keep working during migration.
- **Do not put SignalName type in `models` package**: It is a shared domain type. `eventmodels` is the correct package (matches `StockSymbol`, `OptionSymbol`).
- **Do not implement convenience attribute parsing methods yet**: Defer to Phase 19 when WriteSignal needs them. Keep Phase 17 minimal.
- **Do not modify the RecordSignal handler logic**: Only add a deprecation comment. The handler stays fully functional.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation | Custom ID scheme | `uuid.New()` from `github.com/google/uuid` | Already used for Playground IDs, proven pattern |
| JSON serialization for attributes | Custom marshal/unmarshal | `encoding/json` standard library | Existing `Attributes` type on OrderRecord already uses this |
| Proto code generation | Manual stub files | `task gen:proto` (protoc + twirp plugin) | Existing toolchain generates Go + Python stubs |
| GORM column migration | Manual SQL ALTER TABLE | GORM auto-migrate on startup | Existing pattern -- every OrderRecord field was added this way |

## Common Pitfalls

### Pitfall 1: Attributes Type Mismatch Between Proto and Go
**What goes wrong:** Proto `map<string, string>` generates `map[string]string` in Go, but the internal Go struct uses `map[string]interface{}` per SIG-03.
**Why it happens:** Two different representations needed: wire format (string-only for proto compat) vs internal (flexible for ESDB storage).
**How to avoid:** Keep proto-generated code as-is (`map[string]string`). The `TradeSignal` Go struct uses `map[string]interface{}`. Conversion functions at the RPC boundary (Phase 19). In Phase 17, no conversion is needed because WriteSignal RPC does not exist yet.
**Warning signs:** Compilation errors about type mismatch when trying to assign proto attributes directly to TradeSignal.

### Pitfall 2: Breaking NewOrderRecord Signature
**What goes wrong:** Adding `SignalID` as a parameter to `NewOrderRecord()` breaks every caller (there are many).
**Why it happens:** `NewOrderRecord` already has 20+ positional parameters. Adding another is fragile.
**How to avoid:** Add `SignalID` to the `OrderRecord` struct as a field, but do NOT add it to `NewOrderRecord()` or `PopulateOrderRecord()` parameter lists. Instead, set it after construction: `order.SignalID = signalID`. This is consistent with how `IsAdjustment` and other fields are sometimes set post-construction.
**Warning signs:** Long parameter list diff in NewOrderRecord, many callers needing updates.

### Pitfall 3: Proto Field Number Conflicts
**What goes wrong:** Using a field number that was previously used and removed (reserved) or that conflicts with another proto message.
**Why it happens:** Proto field numbers are forever -- even removed fields leave "holes" that must not be reused.
**How to avoid:** Verified: `PlaceOrderRequest` field 17 is unused. `Order` field 26 is unused. Both are safe for `signal_id`.
**Warning signs:** Proto compilation errors, wire format corruption.

### Pitfall 4: Forgetting to Regenerate Proto Stubs
**What goes wrong:** Go and Python stubs are stale after proto changes; tests pass locally but fail in CI or at runtime.
**Why it happens:** `task gen:proto` must be run manually after proto edits.
**How to avoid:** Run `task gen:proto` immediately after editing `playground.proto`. Verify generated files are updated in `src/go/playground/` and `src/clients/python/rpc/`.
**Warning signs:** Import errors for new proto messages, missing fields at runtime.

### Pitfall 5: OrderRecord SignalID Not Nullable
**What goes wrong:** Existing orders in the database have no signal_id. If the column is NOT NULL, GORM auto-migrate fails or all existing rows need a default.
**Why it happens:** Forgetting that signal_id is optional during migration period.
**How to avoid:** Use `*uuid.UUID` (pointer = nullable). GORM auto-migrate adds the column as nullable by default for pointer types.
**Warning signs:** Migration error on server startup, constraint violation errors.

## Code Examples

### TradeSignal Struct (New File)

```go
// src/go/eventmodels/trade_signal.go
package eventmodels

import (
    "time"

    "github.com/google/uuid"
)

type TradeSignal struct {
    BaseRequestEvent
    ID         uuid.UUID              `json:"id"`
    Name       SignalName             `json:"name"`
    Symbol     StockSymbol            `json:"symbol"`
    Timestamp  time.Time              `json:"timestamp"`
    Attributes map[string]interface{} `json:"attributes"`
    streamName StreamName             `json:"-"`
}

func NewTradeSignal(name SignalName, symbol StockSymbol, timestamp time.Time, attributes map[string]interface{}) *TradeSignal {
    return &TradeSignal{
        ID:         uuid.New(),
        Name:       name,
        Symbol:     symbol,
        Timestamp:  timestamp,
        Attributes: attributes,
    }
}

func (s *TradeSignal) GetSavedEventParameters() SavedEventParameters {
    return SavedEventParameters{
        StreamName:    NewTradeSignalStreamName(string(s.Symbol)),
        EventName:     TradeSignalEventName,
        SchemaVersion: 1,
    }
}
```

### Signal Name Registry (New File)

```go
// src/go/eventmodels/signal_name.go
package eventmodels

import "fmt"

type SignalName string

const (
    SignalMACrossover   SignalName = "ma_crossover"
    SignalStartOfWeek   SignalName = "start_of_week"
    SignalCoveredCall    SignalName = "covered_call"
    SignalMeanReversion SignalName = "mean_reversion"
)

func (s SignalName) Validate() error {
    switch s {
    case SignalMACrossover, SignalStartOfWeek, SignalCoveredCall, SignalMeanReversion:
        return nil
    default:
        return fmt.Errorf("unknown signal name: %s", s)
    }
}
```

### Stream and Event Name Constants (Additions to Existing Files)

```go
// Add to src/go/eventmodels/stream_names.go
const TradeSignalStream StreamName = "trade-signals"

func NewTradeSignalStreamName(symbol string) StreamName {
    return StreamName(fmt.Sprintf("%s-%s", TradeSignalStream, symbol))
}

// Add to src/go/eventmodels/event_names.go
const TradeSignalEventName EventName = "TradeSignalEvent"
```

### Proto Changes

```protobuf
// New message in playground.proto
message TradeSignalProto {
    string id = 1;
    string name = 2;
    string symbol = 3;
    string timestamp = 4;
    map<string, string> attributes = 5;
}

// Add to PlaceOrderRequest (after trace_id = 16)
optional string signal_id = 17;

// Add to Order (after pl = 25)
optional string signal_id = 26;
```

### OrderRecord SignalID Addition

```go
// Add to OrderRecord struct in order_record.go (after Attributes field)
SignalID *uuid.UUID `gorm:"column:signal_id;type:uuid;index:idx_signal_id" copier:"must,nopanic"`
```

### CreateOrderRequest SignalID Addition

```go
// Add to CreateOrderRequest struct in create_order_request.go
SignalID *uuid.UUID `json:"signal_id"`
```

### PlaceOrder Handler signal_id Passthrough

```go
// In grpc.go PlaceOrder handler, when building CreateOrderRequest:
var signalID *uuid.UUID
if req.SignalId != nil {
    parsed, err := uuid.Parse(*req.SignalId)
    if err != nil {
        return nil, fmt.Errorf("PlaceOrder: invalid signal_id: %v", err)
    }
    signalID = &parsed
}

order, webErr := s.dbService.PlaceOrder(playgroundID, &models.CreateOrderRequest{
    // ... existing fields ...
    SignalID: signalID,
})
```

### RecordSignal Deprecation Comment

```go
// Deprecated: RecordSignal is a telemetry-only endpoint that increments Prometheus counters.
// It will be replaced by WriteSignal RPC (Phase 19) which produces domain TradeSignal events.
// Keep functional until all strategies are migrated (Phase 22).
func (s *Server) RecordSignal(ctx context.Context, req *pb.RecordSignalRequest) (*pb.EmptyResponse, error) {
    // ... existing code unchanged ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `SignalV2` (boolean satisfied/not) | `TradeSignal` (named + attributes + timestamp) | Phase 17 | Richer signal representation |
| `RecordSignal` (telemetry counter only) | `TradeSignal` + `WriteSignal` (domain event) | Phase 17 foundation, Phase 19 RPC | Signal-to-order audit trail |
| Inline signal detection in `on_tick()` | Datasource scripts produce signals (future) | Phase 19-22 | Separation of concerns |

## Open Questions

1. **Convenience attribute parsing methods**
   - What we know: D-02 mentions "convenience methods to parse common types" from attributes
   - What's unclear: Whether to add `GetFloat64Attr(key)`, `GetIntAttr(key)` etc. in Phase 17 or defer
   - Recommendation: Defer to Phase 19 when WriteSignal RPC needs them. Phase 17 is just the type definition.

2. **Order.signal_id in proto response**
   - What we know: `PlaceOrderRequest` gets `signal_id` field 17. OrderRecord gets GORM column.
   - What's unclear: Should the `Order` proto message also include `signal_id` so it's visible in responses?
   - Recommendation: Yes, add `optional string signal_id = 26` to the `Order` message. The `convertOrder()` function in grpc.go maps OrderRecord to Order proto -- it should pass through signal_id. This enables Python to verify the link.

3. **Signal name list completeness**
   - What we know: User mentioned `ma_crossover` and `start_of_week` as examples
   - What's unclear: Full list of signal names needed
   - Recommendation: Start with 4-5 signal names from existing strategy logic. New names are trivially added later (just add a constant). The Validate() method ensures only registered names are used.

## Project Constraints (from CLAUDE.md)

- **Go module path**: `github.com/jiaming2012/slack-trading` -- all imports follow this
- **Proto generation**: `task gen:proto` regenerates Go + Python stubs from `src/go/playground.proto`
- **Test command**: `task test` runs `go test -count=1 ./...` in `src/go/backtester-api`
- **GORM auto-migration**: Runs on server startup, adds columns non-destructively
- **Naming conventions**: snake_case.go files, PascalCase exported types, `Err` prefix for new error vars
- **Package organization**: Shared domain types in `eventmodels/`, backtester-specific in `backtester-api/models/`
- **Interface naming**: `I` prefix (`IDatabaseService`, `IBroker`) -- not relevant for Phase 17 but noted
- **Error handling**: `fmt.Errorf("...: %w", err)` wrapping pattern

## Sources

### Primary (HIGH confidence)
- `src/go/playground.proto` -- verified field numbers: PlaceOrderRequest last=16, TickDelta last=10, Order last=25
- `src/go/backtester-api/models/order_record.go` -- OrderRecord struct, NewOrderRecord signature (20+ params), Attributes type
- `src/go/backtester-api/models/create_order_request.go` -- CreateOrderRequest struct fields
- `src/go/backtester-api/models/playground_environment.go` -- typed string constant pattern
- `src/go/eventmodels/stream_names.go` -- stream naming pattern with per-symbol function
- `src/go/eventmodels/event_names.go` -- EventName constant pattern
- `src/go/eventmodels/saved_event.go` -- SavedEvent interface
- `src/go/eventmodels/saved_event_parameters.go` -- SavedEventParameters struct
- `src/go/eventmodels/tracker_v3.go` -- SavedEvent implementation pattern with BaseRequestEvent
- `src/go/eventmodels/base_request_event.go` -- BaseRequestEvent/MetaData embedding
- `src/go/eventmodels/signalv2.go` -- existing SignalV2 type (not reused, confirmed)
- `src/go/backtester-api/router/grpc.go` -- RecordSignal handler (lines 291-321), PlaceOrder handler (lines 1210-1274)
- `.planning/research/ARCHITECTURE.md` -- full architecture patterns and data flow

### Secondary (MEDIUM confidence)
- `.planning/phases/17-signal-foundation/17-CONTEXT.md` -- user decisions and constraints

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all existing libraries verified in go.mod
- Architecture: HIGH -- every pattern directly observed in codebase (SavedEvent, typed constants, GORM model)
- Pitfalls: HIGH -- derived from direct analysis of existing code patterns and proto schema

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable -- no external dependencies, purely internal patterns)
