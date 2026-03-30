# Phase 17: Signal Foundation - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Define the canonical TradeSignal type in Go models and proto, add signal_id to PlaceOrderRequest and OrderRecord. This is the foundation — no persistence, no repositories, no strategy changes. Just the type definition, proto messages, and the signal-to-order audit chain.

Requirements: SIG-01, SIG-02, SIG-03

</domain>

<decisions>
## Implementation Decisions

### D-01: Signal name registry
- **Go typed string constants**: `type SignalName string` with a `const` block
- Examples: `SignalMACrossover SignalName = "ma_crossover"`, `SignalStartOfWeek SignalName = "start_of_week"`
- Python uses matching string literals (no generated enum)
- Adding new signal types = add a constant to the registry file
- File location: `src/go/backtester-api/models/` or `src/go/eventmodels/`

### D-02: Attributes type in proto
- **`map<string, string>`** in proto (matches existing PlaceOrderRequest.attributes pattern)
- Numeric values stored as strings ("99.5", "60")
- Complex nested values as JSON strings if needed
- Go-side: `map[string]string` in proto-generated code; convenience methods to parse common types
- Go internal model uses `map[string]interface{}` with JSON marshaling for EventStoreDB storage

### D-03: signal_id generation
- **Claude's Discretion** — recommended: Go server generates UUID when signal is created/written
- Python receives signal_id back and passes it on PlaceOrderRequest
- signal_id stored on OrderRecord (GORM column, additive migration via auto-migrate)
- During migration phases (21-22): signal_id is optional on PlaceOrderRequest; required after Phase 22

### D-04: RecordSignal RPC — deprecate, replace later
- Mark `RecordSignal` as deprecated in proto comments
- Keep it functional — existing strategies still call it
- Remove in Phase 22 when all strategies are migrated to TradeSignal
- New `WriteSignal` RPC (Phase 19) is the replacement

### Claude's Discretion
- Exact signal_id generation strategy (Go server UUID recommended)
- TradeSignal struct field naming and package location
- Whether to add convenience methods for attribute parsing in Phase 17 or defer
- Proto field numbers for new messages

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Proto Definition
- `src/go/playground.proto` — PlaceOrderRequest (line ~325, field 15 is attributes), RecordSignalRequest (line ~105), service definition

### Go Models
- `src/go/eventmodels/` — Shared domain types (StockSymbol, OptionSymbol patterns for typed string constants)
- `src/go/backtester-api/models/order_record.go` — OrderRecord GORM model (where signal_id column goes)

### Existing Signal Types (legacy, being replaced)
- `src/go/backtester-api/router/grpc.go` — RecordSignal handler
- `src/clients/python/engine/trading_engine.py` — record_decision() calls RecordSignal RPC

### Research
- `.planning/research/ARCHITECTURE.md` — Integration points and data flow
- `.planning/research/STACK.md` — No new Go dependencies needed
- `.planning/research/PITFALLS.md` — signal_id on PlaceOrderRequest from day one

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `type StockSymbol string` pattern in eventmodels — exact pattern for `type SignalName string`
- `PlaceOrderRequest.attributes` (field 15) already `map<string, string>` — TradeSignal attributes follow same pattern
- GORM auto-migration adds columns non-destructively — signal_id on OrderRecord is safe
- `task gen:proto` regenerates Go + Python stubs

### Established Patterns
- Typed string constants for enums (StockSymbol, OptionSymbol, PlaygroundEnvironment)
- Proto `map<string, string>` for flexible metadata (attributes on orders)
- GORM struct tags for new columns

### Integration Points
- `playground.proto` — add TradeSignalProto message + signal_id field on PlaceOrderRequest
- `order_record.go` — add SignalID field (uuid, nullable)
- `eventmodels/` — add TradeSignal struct + SignalName type
- `task gen:proto` — regenerate after proto changes

</code_context>

<specifics>
## Specific Ideas

- TradeSignal examples from user's Notion doc: `ma_crossover` (with timeframe, action, symbol, prices), `start_of_week` (with date fields)
- Composite signals are just TradeSignals with more attributes (e.g. `ma_crossover_multi_timeframe`)
- Every PlaceOrderRequest should come from a single TradeSignal — this is the core invariant

</specifics>

<deferred>
## Deferred Ideas

- WriteSignal RPC — Phase 19
- Signal repositories (in-memory, ESDB) — Phase 18
- Signal delivery via TickDelta — Phase 18
- Python datasource scripts — Phase 19
- Strategy migration — Phases 21-22

</deferred>

---

*Phase: 17-signal-foundation*
*Context gathered: 2026-03-30*
