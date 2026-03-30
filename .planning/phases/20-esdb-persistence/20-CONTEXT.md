# Phase 20: ESDB Persistence - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement ESDBSignalRepository for live mode signal persistence. Single global ESDB stream. Environment-based repository injection at startup. Queryability via Go-side filtering (read stream + filter). Integration test for write-read round-trip.

Requirements: REPO-02, QUERY-01

</domain>

<decisions>
## Implementation Decisions

### D-01: Single global ESDB stream — CONFIRMED
- **One stream: `trade-signals`** for all signals regardless of symbol
- This overrides ROADMAP wording of "per-symbol EventStoreDB streams"
- Matches user's Notion doc: "All signals should go to the same event stream so that they are ordered"
- Performance is negligible at expected signal volume (<1000/day)
- Global ordering preserved — every signal has a position in one sequence

### D-02: Queryability — read stream + filter in Go
- Read `trade-signals` stream from ESDB, deserialize events, filter by name/symbol/time in Go
- No ESDB projections ($by_category) needed
- GetSignals RPC already supports these filters via ISignalRepository interface
- ESDBSignalRepository.ReadPending() reads from ESDB and applies same filter logic

### D-03: Environment-based repository injection
- **Live mode**: ESDBSignalRepository (write-through to ESDB)
- **Sim mode**: InMemorySignalRepository (already built in Phase 18)
- Selection based on playground environment at startup
- Same ISignalRepository interface — strategy code unchanged

### Claude's Discretion
- ESDBSignalRepository internal design (batch reads, cursor management)
- How environment detection works at Server startup
- Integration test structure (TestContainers pattern from existing tests)
- ESDB stream retention policy

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Signal Infrastructure (Phases 17-19)
- `src/go/backtester-api/models/signal_repository_interface.go` — ISignalRepository interface
- `src/go/backtester-api/models/signal_repository_memory.go` — InMemorySignalRepository (reference implementation)
- `src/go/eventmodels/trade_signal.go` — TradeSignal with SavedEvent implementation
- `src/go/eventmodels/stream_names.go` — TradeSignalStream constant
- `src/go/backtester-api/router/grpc.go` — WriteSignal/GetSignals/GetProcessedSignals handlers + globalSignalRepo

### Existing ESDB Patterns
- `src/go/eventconsumers/` — ESDB consumer patterns (esdbConsumerStream generic)
- `src/go/eventproducers/esdb_producer.go` — ESDB producer with Save(), TradeSignalEventName subscription
- `src/go/data/in_memory.go` — SavedEvent persistence patterns

### Integration Tests
- `integration_testing/` — Existing TestContainers E2E tests

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `esdbConsumerStream[T]` generic — read events from ESDB stream with type-safe deserialization
- `EsdbProducer.Save()` — write SavedEvent to ESDB (TradeSignal already implements SavedEvent)
- `TradeSignalEventName` — already registered in EsdbProducer subscriptions (Phase 18)
- TestContainers setup for Postgres + ESDB in integration tests

### Established Patterns
- `SavedEvent` interface → `GetSavedEventParameters()` → stream name + event name + schema version
- Environment detection via `GO_ENV` or playground environment field
- Interface-based repository swapping (IBroker, IDatabaseService patterns)

### Integration Points
- Server struct `globalSignalRepo` field — swap InMemory for ESDB based on environment
- `esdb_producer.go` already subscribes to TradeSignalEventName (from Phase 18 batch-persist)
- `cmd/main.go` — server initialization where environment detection happens

</code_context>

<specifics>
## Specific Ideas

- Stream name: `trade-signals` (constant already exists as TradeSignalStream in stream_names.go)
- Write-through: ESDBSignalRepository.Write() persists immediately to ESDB
- ReadPending: reads from ESDB stream, deserializes, filters by time/name/symbol
- Integration test: write 3 signals, read back with filters, verify correctness

</specifics>

<deferred>
## Deferred Ideas

- ESDB stream retention policy — configure later based on volume
- ESDB projection-based queryability — not needed at current scale
- Signal replay from ESDB stream — Phase 23

</deferred>

---

*Phase: 20-esdb-persistence*
*Context gathered: 2026-03-31*
