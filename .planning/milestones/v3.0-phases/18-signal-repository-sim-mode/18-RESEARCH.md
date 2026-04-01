# Phase 18: Signal Repository & Sim Mode - Research

**Researched:** 2026-03-30
**Domain:** Go interface-based repository pattern, FIFOQueue clock-gated delivery, proto extension, ESDB batch persistence
**Confidence:** HIGH

## Summary

Phase 18 builds ISignalRepository with an InMemorySignalRepository for simulation mode, delivers signals to Python strategies via a new `NewSignals` field on TickDelta, and adds opt-in ESDB batch persistence on SavePlayground. All building blocks already exist from Phase 17 (TradeSignal struct, TradeSignalProto, signal_id on OrderRecord, ESDB stream names).

The implementation follows three established codebase patterns: (1) interface-based repository swapping (IBroker pattern), (2) clock-gated delivery during simulateTick, and (3) proto TickDelta extension with Go-to-proto conversion in grpc.go's NextTick handler. The signal repository is simpler than most existing subsystems because sim mode requires no external dependencies -- signals are written and read from an in-memory sorted slice.

**Primary recommendation:** Implement as a slice-based InMemorySignalRepository with a cursor index, drain pending signals in simulateTick after clock advance, convert to TradeSignalProto in NextTick handler, and batch-write to ESDB in SavePlayground when --save-to-db is used.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Repository interface -- single global stream, not per-symbol. Symbol is an attribute ON the TradeSignal, not a stream partition key. API: `Write(signal)` / `ReadPending(upToTime, filterSymbols?, filterNames?)`. OVERRIDES research recommendation of per-symbol streams.
- D-02: Signal delivery -- NewSignals on TickDelta. Add `repeated TradeSignalProto new_signals` field to TickDelta proto message. Playground drains signal queue during simulateTick, same pattern as candles.
- D-03: Clock gating -- FIFOQueue pattern with clock cutoff. Signals sorted by timestamp, drain all signals where timestamp <= clock.GetCurrentTime() during simulateTick.
- D-04: Persistence -- live write-through to ESDB (automatic), sim batch-on-save only (via existing --save-to-db flag on SavePlayground).

### Claude's Discretion
- FIFOQueue implementation details for signal delivery
- Exact ISignalRepository method signatures
- Stream naming convention for persisted sim signals
- Whether ReadPending returns TradeSignal or a wrapper with delivery metadata

### Deferred Ideas (OUT OF SCOPE)
- ESDBSignalRepository (live mode implementation) -- Phase 20
- WriteSignal RPC -- Phase 19
- Signal replay from persisted streams -- Phase 23
- Per-consumer cursor tracking -- not needed with FIFOQueue approach
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REPO-01 | ISignalRepository interface with in-memory implementation for simulations | Interface pattern from IBroker; InMemorySignalRepository with sorted slice + cursor; clock-gated ReadPending |
| REPO-03 | Opt-in persistence of sim signals to EventStoreDB via CLI flag | Batch write in SavePlayground using existing EsdbProducer.Save(); TradeSignal already implements SavedEvent |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `sort` | 1.22.4 | Sorting signals by timestamp in InMemorySignalRepository | No external dependency needed for sorted slice |
| Go stdlib `sync` | 1.22.4 | Mutex on InMemorySignalRepository for thread safety | Same pattern as ExerciseOptionRequestQueue |
| `google.golang.org/protobuf` | 1.35.2 | Proto timestamp conversion for TradeSignalProto | Already in go.mod, used for all proto types |
| `github.com/google/uuid` | 1.6.0 | Signal ID generation and parsing | Already used throughout for playground/order IDs |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | 1.9.0 | Unit test assertions | Testing clock-gated delivery, repository operations |
| EventStoreDB client | 4.1.0 | Batch signal persistence in SavePlayground | Only when --save-to-db flag triggers ESDB write |

No new dependencies required. Everything needed is already in go.mod.

## Architecture Patterns

### Recommended Project Structure

New files:
```
src/go/backtester-api/models/
  signal_repository_interface.go    # ISignalRepository interface
  signal_repository_memory.go       # InMemorySignalRepository
  signal_repository_memory_test.go  # Unit tests
```

Modified files:
```
src/go/backtester-api/models/
  tick_delta.go                     # Add NewSignals field
  playground.go                     # Add signalRepo field, drain in simulateTick
src/go/playground.proto             # Add new_signals to TickDelta
src/go/backtester-api/router/
  grpc.go                           # Convert NewSignals to proto in NextTick, batch persist in SavePlayground
```

### Pattern 1: Interface-Based Repository (IBroker Pattern)

**What:** Define `ISignalRepository` interface in a dedicated `*_interface.go` file. InMemorySignalRepository implements it. Future ESDBSignalRepository (Phase 20) implements the same interface.
**When to use:** Always -- Playground holds `ISignalRepository`, never a concrete type.
**Source:** `src/go/backtester-api/models/broker_interface.go`

```go
// signal_repository_interface.go
package models

import (
    "time"
    "github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type ISignalRepository interface {
    // Write stores a signal in the repository
    Write(signal *eventmodels.TradeSignal) error

    // ReadPending returns signals with Timestamp <= upTo that haven't been delivered yet.
    // Returns them in timestamp order. Advances the internal cursor past returned signals.
    ReadPending(upTo time.Time) []*eventmodels.TradeSignal

    // GetAll returns all signals in the repository (used for batch persistence on save)
    GetAll() []*eventmodels.TradeSignal
}
```

**Design note on D-01 (single global stream):** The interface has NO symbol parameter on ReadPending. All signals go into one sorted list. If a strategy wants only certain symbols, it filters client-side after receiving signals in TickDelta. This matches the user's explicit decision that symbol is a filter attribute, not a partition key.

### Pattern 2: Clock-Gated Delivery in simulateTick

**What:** During simulateTick, after `clock.Add(d)`, call `signalRepo.ReadPending(clock.CurrentTime)` to get signals whose timestamp <= current clock time. Include them in TickDelta.
**When to use:** Every tick in simulator mode.

The existing simulateTick flow is:
1. Process pending orders and fills
2. Check liquidations
3. `clock.Add(d)` -- advance time
4. `repo.Update(clock.CurrentTime)` -- get new candles
5. Check option expirations/assignments
6. Build TickDelta and return

Signals should be drained at step 4.5 (after clock advance, alongside candle delivery):

```go
// In simulateTick, after candle updates (line ~1627):
var newSignals []*eventmodels.TradeSignal
if p.signalRepo != nil {
    newSignals = p.signalRepo.ReadPending(p.clock.CurrentTime)
}

// Then include in TickDelta return:
return &TickDelta{
    NewTrades:     newTrades,
    NewCandles:    newCandles,
    NewSignals:    newSignals,  // NEW
    // ... existing fields
}, nil
```

**Important:** This is NOT the channel-based FIFOQueue drain pattern used in liveTick. In simulateTick, candles come from `repo.Update()` (direct return), not from dequeuing a channel. Signals follow the same approach: direct read from the repository, gated by clock time. The "FIFOQueue" concept from CONTEXT.md D-03 means "first-in-first-out ordered by timestamp with a cursor" -- implemented as a sorted slice with an index pointer, not as `eventmodels.FIFOQueue[T]` (which is a channel-based queue for async inter-goroutine communication).

### Pattern 3: InMemorySignalRepository Implementation

**What:** Sorted slice of signals + cursor index. Thread-safe via mutex.
**Source pattern:** `ExerciseOptionRequestQueue` (slice + mutex + Drain) in `exercise_option_request_queue.go`

```go
// signal_repository_memory.go
package models

type InMemorySignalRepository struct {
    signals []*eventmodels.TradeSignal  // sorted by Timestamp
    cursor  int                         // index of next undelivered signal
    mutex   sync.Mutex
}

func (r *InMemorySignalRepository) Write(signal *eventmodels.TradeSignal) error {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    // Insert in sorted position by Timestamp
    // (binary search insertion to maintain sort order)
    return nil
}

func (r *InMemorySignalRepository) ReadPending(upTo time.Time) []*eventmodels.TradeSignal {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    var result []*eventmodels.TradeSignal
    for r.cursor < len(r.signals) && !r.signals[r.cursor].Timestamp.After(upTo) {
        result = append(result, r.signals[r.cursor])
        r.cursor++
    }
    return result
}

func (r *InMemorySignalRepository) GetAll() []*eventmodels.TradeSignal {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    out := make([]*eventmodels.TradeSignal, len(r.signals))
    copy(out, r.signals)
    return out
}
```

### Pattern 4: Proto Conversion in NextTick Handler

**What:** Convert `[]*eventmodels.TradeSignal` to `[]*pb.TradeSignalProto` in grpc.go's NextTick handler, same location where candles and trades are converted.
**Source:** Lines 851-946 of `grpc.go` -- iterates over tick.NewCandles, tick.NewTrades, builds proto equivalents.

```go
// In NextTick handler, after candle/trade conversion:
newSignals := make([]*pb.TradeSignalProto, 0, len(tick.NewSignals))
for _, sig := range tick.NewSignals {
    attrs := make(map[string]string, len(sig.Attributes))
    for k, v := range sig.Attributes {
        attrs[k] = fmt.Sprintf("%v", v)  // map[string]interface{} -> map[string]string for proto
    }
    newSignals = append(newSignals, &pb.TradeSignalProto{
        Id:         sig.ID.String(),
        Name:       string(sig.Name),
        Symbol:     string(sig.Symbol),
        Timestamp:  timestamppb.New(sig.Timestamp),
        Attributes: attrs,
    })
}

tickDelta = &pb.TickDelta{
    // ... existing fields
    NewSignals: newSignals,
}
```

**Note on Attributes type mismatch:** Go's `TradeSignal.Attributes` is `map[string]interface{}`, proto's `TradeSignalProto.attributes` is `map<string, string>`. The conversion uses `fmt.Sprintf("%v", v)` to stringify interface values. This was a deliberate Phase 17 decision (D-02 in STATE.md).

### Pattern 5: Batch ESDB Persistence in SavePlayground

**What:** When SavePlayground is called (triggered by Python's --save-to-db flag), batch-write all signals from the InMemorySignalRepository to ESDB.
**Source:** `DatabaseService.SavePlayground()` in `data/database_service.go` (line 1605)

The current SavePlayground flow saves: playground metadata, order records, equity plots. Signal persistence should be added as an additional step:

```go
// In SavePlayground or a new helper called from grpc.go's SavePlayground handler:
if playground.GetSignalRepo() != nil {
    signals := playground.GetSignalRepo().GetAll()
    for _, signal := range signals {
        if err := esdbProducer.Save(ctx, signal); err != nil {
            return fmt.Errorf("failed to persist signal: %w", err)
        }
    }
}
```

**Stream naming for persisted sim signals:** Per D-01, signals use a single global stream. However, `TradeSignal.GetSavedEventParameters()` currently returns a per-symbol stream name (`trade-signals-{symbol}`). For ESDB persistence, the existing per-symbol stream naming is fine because ESDB naturally handles this -- each signal writes to its own symbol stream. The "single global stream" decision applies to the in-memory repository (one sorted list), not to ESDB storage.

### Anti-Patterns to Avoid

- **Using channel-based FIFOQueue for sim signals:** The `eventmodels.FIFOQueue[T]` is designed for async cross-goroutine communication (live mode). In sim mode, signals are written and read synchronously within the same tick loop. A sorted slice with cursor is simpler and more appropriate.

- **Adding signalRepo to NewPlayground constructor:** The constructor already has 12+ positional params. Follow the `SetNewTradesQueue` / `SetNewCandlesQueue` setter pattern instead. Set signalRepo via a setter after construction.

- **Modifying PopulatePlayground signature:** Same concern. Use a setter method `SetSignalRepo(repo ISignalRepository)` on Playground, called from the RPC handler that creates the playground.

- **Per-symbol ReadPending:** D-01 explicitly says single global stream. Do NOT add symbol filtering to ReadPending. Strategy-side filtering in Python is the correct approach.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ESDB event persistence | Custom ESDB write logic | `EsdbProducer.Save(ctx, signal)` | TradeSignal already implements SavedEvent; zero custom code needed |
| Sorted insertion | Manual array shifting | `sort.Search` + `slices.Insert` | Binary search insertion is O(log n) for finding position |
| Proto timestamp | Manual string parsing | `timestamppb.New(time.Time)` | Already used in proto conversion; handles timezone correctly |
| Thread-safe queue | Lock-free or channel-based | `sync.Mutex` + slice | Matches ExerciseOptionRequestQueue; no async needed in sim mode |

## Common Pitfalls

### Pitfall 1: Lookahead Bias from Incorrect Clock Gating
**What goes wrong:** Signals with future timestamps leak into the current tick, giving the strategy information it shouldn't have.
**Why it happens:** Using `<` instead of `<=` in the comparison, or draining signals before clock.Add() instead of after.
**How to avoid:** ReadPending must use `signal.Timestamp <= upTo` (not `<`). Drain signals AFTER `clock.Add(d)` in simulateTick. Unit test: write signal at T+5min, tick with 1min duration, verify signal NOT delivered until clock reaches T+5min.
**Warning signs:** Strategy placing orders based on signals that haven't "happened" yet in simulated time.

### Pitfall 2: Attributes Type Mismatch Between Go and Proto
**What goes wrong:** Complex attribute values (numbers, booleans) lose type fidelity when converted to `map<string, string>`.
**Why it happens:** Go's `map[string]interface{}` can hold any type, proto's `map<string, string>` cannot.
**How to avoid:** Document that strategy-side Python code should parse string attributes back to appropriate types. The conversion is `fmt.Sprintf("%v", v)` -- test with float64, int, bool values to ensure round-trip works.
**Warning signs:** Python strategy fails to parse numeric attribute values.

### Pitfall 3: Missing nil Check on signalRepo
**What goes wrong:** Existing tests and live playgrounds that don't use signals crash with nil pointer dereference.
**Why it happens:** signalRepo is nil on playgrounds that predate this feature or don't opt in.
**How to avoid:** Always guard with `if p.signalRepo != nil` before calling ReadPending. The nil check pattern is already used for `dbService` in `Tick()` (line 1693).
**Warning signs:** Test failures in existing tests that don't set up a signal repository.

### Pitfall 4: Cursor Reset on Multiple ReadPending Calls
**What goes wrong:** Calling ReadPending twice with the same timestamp returns empty on the second call because cursor already advanced.
**Why it happens:** Cursor is monotonically advancing -- signals are "consumed" once returned.
**How to avoid:** This is the correct behavior (signals delivered once). Document it clearly. GetAll() exists for batch persistence which needs all signals regardless of cursor.
**Warning signs:** None -- this is by design.

### Pitfall 5: SavePlayground ESDB Write Without EsdbProducer Access
**What goes wrong:** The SavePlayground RPC handler in grpc.go needs access to EsdbProducer to batch-persist signals, but the Server struct may not have it.
**Why it happens:** The Server struct currently has `dbService`, `optionsClient`, but may not have `esdbProducer`.
**How to avoid:** Check if Server struct already has EsdbProducer access. If not, it may need to be injected, OR the batch write can go through the pub/sub event bus (same pattern as `handleSaveRequest` in esdb_producer.go).
**Warning signs:** Cannot call `esdbProducer.Save()` from grpc.go.

## Code Examples

### TickDelta Proto Change (verified against current proto)

Current TickDelta message ends at field 10 (positions). Add field 11:

```protobuf
message TickDelta {
    repeated Trade new_trades = 1;
    repeated Candle new_candles = 2;
    repeated Order invalid_orders = 3;
    repeated TickDeltaEvent events = 4;
    string current_time = 5;
    bool is_backtest_complete = 6;
    double balance = 7;
    double equity = 8;
    double free_margin = 9;
    map<string, Position> positions = 10;
    repeated TradeSignalProto new_signals = 11;  // NEW: Phase 18
}
```

Source: `src/go/playground.proto` lines 353-365 (verified TradeSignalProto already exists at line 478)

### TickDelta Go Struct Change

```go
type TickDelta struct {
    NewTrades          []*TradeRecord          `json:"new_trades,omitempty"`
    NewCandles         []*BacktesterCandle     `json:"new_candles,omitempty"`
    NewSignals         []*eventmodels.TradeSignal `json:"new_signals,omitempty"`  // NEW
    InvalidOrders      []*OrderRecord          `json:"invalid_orders,omitempty"`
    Events             []*TickDeltaEvent       `json:"events,omitempty"`
    // ... existing fields unchanged
}
```

Source: `src/go/backtester-api/models/tick_delta.go`

### Unit Test: Clock-Gated Signal Delivery

```go
func TestInMemorySignalRepository_ReadPending_ClockGating(t *testing.T) {
    repo := NewInMemorySignalRepository()

    // Write signals at different times
    t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
    t2 := time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC)
    t3 := time.Date(2025, 1, 1, 10, 10, 0, 0, time.UTC)

    repo.Write(eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, "AAPL", t1, nil))
    repo.Write(eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, "AAPL", t2, nil))
    repo.Write(eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, "SPY", t3, nil))

    // At t1: should get only first signal
    signals := repo.ReadPending(t1)
    assert.Len(t, signals, 1)
    assert.Equal(t, eventmodels.SignalMeanReversion, signals[0].Name)

    // At t2: should get second signal only (first already consumed)
    signals = repo.ReadPending(t2)
    assert.Len(t, signals, 1)
    assert.Equal(t, eventmodels.SignalCoveredCall, signals[0].Name)

    // At t3: should get third signal
    signals = repo.ReadPending(t3)
    assert.Len(t, signals, 1)
    assert.Equal(t, eventmodels.SignalMACrossover, signals[0].Name)

    // No more signals
    signals = repo.ReadPending(t3.Add(time.Hour))
    assert.Len(t, signals, 0)
}
```

### Playground Integration Point

```go
// On Playground struct (around line 48-55):
signalRepo ISignalRepository `json:"-" gorm:"-"`

// Setter (follows SetNewCandlesQueue pattern at line 2373):
func (p *Playground) SetSignalRepo(repo ISignalRepository) {
    p.signalRepo = repo
}

func (p *Playground) GetSignalRepo() ISignalRepository {
    return p.signalRepo
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Signals embedded in strategy on_tick() | TradeSignal as first-class domain type | Phase 17 (current milestone) | Enables decoupled datasource/strategy |
| No signal persistence | ESDB-backed signal streams | Phase 17 foundation, Phase 18 sim batch | Enables replay and audit |
| N/A (no repo pattern for signals) | ISignalRepository interface | Phase 18 (this phase) | Clean testability, environment-based behavior |

## Open Questions

1. **EsdbProducer access from SavePlayground handler**
   - What we know: SavePlayground in grpc.go delegates to `s.dbService.SavePlayground()`. EsdbProducer is separate.
   - What's unclear: Whether Server struct has EsdbProducer reference, or if signals should persist via pub/sub event.
   - Recommendation: Check Server struct fields. If no EsdbProducer, either add it or use `eventpubsub.Publish()` to trigger ESDB write. The pub/sub approach is simpler (no constructor change) but adds indirection.

2. **Signal stream name for batch persistence**
   - What we know: `TradeSignal.GetSavedEventParameters()` returns per-symbol stream name (`trade-signals-AAPL`).
   - What's unclear: D-01 says "single global stream" but that's for the in-memory repo. For ESDB, per-symbol streams may still be correct.
   - Recommendation: Use the existing per-symbol ESDB stream naming (it's already implemented). D-01's "single stream" applies to the in-memory repository only. This is consistent because ESDB storage is an implementation detail; the "single stream" concept is about the API surface (one ReadPending call returns all symbols).

3. **Signal Write entry point for sim mode**
   - What we know: In sim mode, Python strategies will eventually call WriteSignal RPC (Phase 19). But for Phase 18, the in-memory repo needs signals loaded somehow.
   - What's unclear: How do signals get into the repository before Phase 19's WriteSignal RPC exists?
   - Recommendation: Phase 18 provides the `Write()` method on the interface. For now, signals can be loaded programmatically in Go tests and via a future RPC. The in-memory repo is infrastructure that Phase 19 will build on. Phase 18 tests load signals directly via `repo.Write()`.

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis of all integration points:
  - `src/go/eventmodels/trade_signal.go` -- TradeSignal struct (Phase 17 output)
  - `src/go/eventmodels/fifo_queue.go` -- FIFOQueue generic (channel-based, for live mode)
  - `src/go/backtester-api/models/playground.go` -- simulateTick flow (lines 1499-1683), queue fields (lines 48-50), setter pattern (lines 2373-2393)
  - `src/go/backtester-api/models/tick_delta.go` -- current TickDelta struct
  - `src/go/backtester-api/models/broker_interface.go` -- IBroker interface pattern
  - `src/go/backtester-api/models/exercise_option_request_queue.go` -- slice+mutex+Drain pattern
  - `src/go/backtester-api/router/grpc.go` -- NextTick handler (lines 791-950), SavePlayground handler (lines 684-700)
  - `src/go/playground.proto` -- TickDelta (lines 353-365), TradeSignalProto (line 478)
  - `src/go/data/database_service.go` -- SavePlayground flow (lines 1605-1639)
  - `src/go/eventmodels/stream_names.go` -- NewTradeSignalStreamName already exists
  - `src/go/eventproducers/esdb_producer.go` -- EsdbProducer.Save pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies, all exist in go.mod
- Architecture: HIGH - directly verified all integration points in source code
- Pitfalls: HIGH - identified from actual code patterns and type mismatches
- ESDB batch persistence: MEDIUM - need to verify EsdbProducer access from Server struct

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable -- no external library changes expected)
