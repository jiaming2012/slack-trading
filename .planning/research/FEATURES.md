# Feature Landscape: TradeSignal Framework

**Domain:** Event-driven trading signal framework for Go+Python backtesting/live platform
**Researched:** 2026-03-30
**Overall confidence:** HIGH (grounded in codebase inspection + established patterns from NautilusTrader, EventStoreDB, and event-sourcing literature)

---

## Table Stakes

Features users (the operator/developer) expect. Missing = the framework feels incomplete or untrustworthy.

| Feature | Why Expected | Complexity | Dependencies | Notes |
|---------|--------------|------------|--------------|-------|
| **TradeSignal struct with Name + Attributes + Timestamp** | Every signal framework needs a uniform event envelope. Currently signals are ad-hoc (`SignalDecision`, `OpenSignalV3`, `SignalV2`, `RsiTradeSignal` -- 5+ incompatible types). | Low | None | Single canonical type replaces the zoo. Must be serializable to both protobuf (for RPC) and JSON (for EventStoreDB). Attributes is `map[string]string` to stay schema-flexible. |
| **Single-signal-per-order rule** | Audit trail. Every `PlaceOrderRequest` must reference exactly one TradeSignal. Without this, you cannot answer "why was this order placed?" after the fact. | Low | TradeSignal struct exists | Add `signal_id` field to `PlaceOrderRequest` proto message. Go server validates presence before accepting the order. |
| **Signal persistence to EventStoreDB (live mode)** | Signals are the "why" behind every trade. If they are ephemeral, you lose the audit trail on process restart. EventStoreDB is already deployed and used for event sourcing. | Med | TradeSignal struct, ESDB client (already in `eventconsumers/esdb_consumer_stream.go`) | Stream naming convention: `signals-{playground_id}`. Use category projection `$ce-signals` for cross-playground queries. |
| **In-memory signal repository (sim mode)** | Simulator playgrounds run millions of ticks. Writing every signal to ESDB would be absurdly slow for backtests. Need a fast in-memory store with the same interface. | Med | TradeSignal struct | Go-side `SignalRepository` interface with `InMemorySignalRepository` and `ESDBSignalRepository` implementations. Strategy code sees the same API regardless of environment. |
| **Signal emission from Python strategies** | Strategies currently call `PlaceOrder` directly. The new flow is: strategy emits TradeSignal, then passes signal reference to `PlaceOrder`. Python must have a clean way to create and emit signals. | Med | TradeSignal proto message, `RecordSignal` RPC (already exists -- needs expansion) | Expand existing `RecordSignalRequest` proto to carry full TradeSignal payload instead of just telemetry metadata. Python emits via RPC; Go server persists to the appropriate repository. |
| **Live/sim parity: same strategy code, different repository backend** | The #1 architectural goal per PROJECT.md. NautilusTrader calls this "research-to-live parity" and it is the gold standard for trading frameworks. If sim and live diverge, backtests are meaningless. | Med | Signal repository interface, env-based injection | Strategy code calls `emit_signal()` and `place_order(signal_id)`. The engine selects `InMemorySignalRepository` for sim or `ESDBSignalRepository` for live based on `PlaygroundEnvironment`. No `if live:` branches in strategy code. |
| **Strategy migration: all existing strategies use TradeSignal** | If only new strategies use the framework while old ones bypass it, you have two systems running in parallel indefinitely. Clean break needed. | High | All above features stable | 7 strategies to migrate: MeanReversion, CoveredCall, CreditSpread, OptionsMeanReversion, Wheel, PDFWheel. Move originals to `deprecated/`. This is the bulk of the work. |

---

## Differentiators

Features that set the framework apart from the current ad-hoc approach. Not strictly required for v3.0 to ship, but high-value.

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| **Replay mode: simulate from persisted signal streams** | Record a live session's signals, then replay them in simulation to validate that the strategy would have made the same decisions. Critical for regression testing and "what if" analysis. | High | ESDB signal persistence, in-memory repository | Read signals from ESDB stream, feed them into a simulator playground as if they were freshly generated. Requires a `ReplaySignalRepository` that reads from ESDB but writes to in-memory. Integration test validates round-trip fidelity. |
| **Standalone datasource scripts producing signals** | Decouple "where data comes from" from "how signals are generated." A datasource script (e.g., Polygon candle fetcher) writes raw market data signals to a stream. Multiple strategies can consume the same stream independently. | High | ESDB signal persistence, stream naming conventions | New process type: a datasource producer that runs alongside (or instead of) the strategy. Writes `MarketData` signals to `datasource-{symbol}-{timeframe}` stream. Strategies subscribe to that stream rather than directly calling Polygon. |
| **Composite signals (AND/OR combinations)** | The existing codebase already has multi-timeframe composite logic (e.g., `SuperTrend4h1hStochRsi15mUp`). Formalizing this as a first-class concept -- "Signal A AND Signal B within N seconds" -- makes compound conditions declarative rather than buried in strategy code. | Med | TradeSignal struct with `parent_signal_ids` field | A CompositeSignal references child signal IDs and a combination rule. Strategies define composites declaratively; the framework evaluates them. Reduces duplicated multi-indicator logic across strategies. |
| **Signal queryability via EventStoreDB projections** | Query signals by name, symbol, or timeframe without loading entire streams. ESDB's `$by_category` and `$by_event_type` system projections enable this natively. | Low | ESDB signal persistence | Enable `$by_event_type` system projection. Use event type naming convention: `TradeSignal-{signal_name}`. Then `$et-TradeSignal-mean_reversion_dip` gives all mean reversion dip signals across all playgrounds. |
| **New RPC endpoint: GetProcessedSignals** | View what signals a strategy has emitted for a playground, with filtering by name/symbol/timeframe. Essential for debugging "why did the strategy not trade today?" | Low | Signal repository (either impl) | New `GetProcessedSignals` RPC in `playground.proto`. Reads from the appropriate signal repository. Returns paginated list of TradeSignals with metadata. |
| **Telemetry integration: signals in OTel/Grafana** | The existing `RecordSignal` RPC already increments `grodt_signals_generated_total` in Prometheus. Extending this to include signal attributes as OTel span events creates full observability. | Low | TradeSignal struct, existing OTel infrastructure | Add signal name, symbol, direction as attributes to the existing OTel metric. Add Grafana dashboard panel for signal rate by type. Alert on "zero signals in N minutes" for live playgrounds. |

---

## Anti-Features

Features to explicitly NOT build. Tempting rabbit holes that would derail v3.0.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Signal backtesting engine (separate from playground)** | The playground + tick loop IS the backtesting engine. Building a separate signal-only backtester duplicates the entire simulation stack without the order-filling logic that validates signals actually produce profitable trades. | Use existing playground backtester. Replay mode feeds signals into it. |
| **Real-time signal streaming via WebSocket/gRPC streaming** | Adds significant complexity (connection management, backpressure, reconnection) for a single-operator platform. The tick-based polling model works for both sim and live. | Keep the existing tick-based polling. Python calls `NextTick` which returns new signals. For live mode, the tick interval determines latency -- already configurable via `get_next_tick_seconds()`. |
| **Signal marketplace / multi-user signal sharing** | Single-operator platform. Multi-tenancy adds auth, isolation, rate limiting -- all irrelevant. | Signals are scoped to a playground. One operator, one instance. |
| **ML-based signal scoring / confidence weighting** | Interesting but orthogonal to the framework. Signal scoring is strategy logic, not infrastructure. Adding it to the framework forces all strategies to use a scoring model. | Let individual strategies implement their own confidence logic in Python. The framework just carries the signal. |
| **Signal deduplication / conflict resolution at framework level** | Strategies already manage their own position limits and signal cooldowns (e.g., MeanReversion's `TradeGroup` with status tracking). Framework-level dedup would need to understand strategy semantics it cannot know. | Each strategy manages its own signal dedup logic. The framework stores all signals, even duplicates, for audit completeness. |
| **Custom ESDB projections in JavaScript** | ESDB supports user-defined projections in JS, but they are fragile, hard to debug, and a maintenance burden. System projections (`$by_category`, `$by_event_type`) cover 95% of query needs. | Use system projections + server-side filtering in Go. If complex queries are needed, read the stream and filter in Go code. |
| **Signal versioning / schema evolution** | Premature. The `Attributes` map provides schema flexibility without formal versioning. If signal schemas need to evolve, add new attribute keys -- old consumers ignore unknown keys. | Use `map[string]string` Attributes for extensibility. Add typed helper methods in Go/Python for common attribute access patterns. |

---

## Feature Dependencies

```
TradeSignal struct (proto + Go + Python)
  |
  +---> Signal repository interface (Go)
  |       |
  |       +---> InMemorySignalRepository (sim)
  |       |
  |       +---> ESDBSignalRepository (live)
  |       |       |
  |       |       +---> Signal queryability (ESDB projections)
  |       |       |
  |       |       +---> Replay mode (reads from ESDB, writes to in-memory)
  |       |
  |       +---> GetProcessedSignals RPC endpoint
  |
  +---> Expand RecordSignal RPC (or new EmitSignal RPC)
  |       |
  |       +---> Python signal emission from strategies
  |               |
  |               +---> Strategy migration (all 7 strategies)
  |
  +---> Single-signal-per-order rule
  |       |
  |       +---> PlaceOrderRequest proto change (signal_id field)
  |
  +---> Telemetry integration (OTel attributes + Grafana panel)
  |
  +---> Composite signals (optional, after base signals work)

Standalone datasource scripts
  |
  +---> ESDB signal persistence (must exist first)
  +---> Stream naming conventions (must be defined first)
```

---

## MVP Recommendation

**Phase 1: Foundation (ship first)**
1. TradeSignal struct in proto + Go + Python
2. Signal repository interface + InMemorySignalRepository
3. Expand RecordSignal RPC to handle full signal emission
4. Single-signal-per-order rule (PlaceOrderRequest proto change)

**Phase 2: Persistence + Parity**
5. ESDBSignalRepository implementation
6. Environment-based repository injection (sim = in-memory, live = ESDB)
7. GetProcessedSignals RPC endpoint
8. Telemetry integration (OTel + Grafana)

**Phase 3: Migration**
9. Migrate all 7 strategies to use TradeSignal emission
10. Move originals to `deprecated/`
11. Integration tests proving sim/live parity

**Phase 4: Advanced (defer to v3.1 or later)**
12. Replay mode from ESDB streams
13. Standalone datasource scripts
14. Composite signals
15. Signal queryability via ESDB projections

**Rationale for ordering:**
- Foundation must exist before anything else can be built on it.
- Persistence + parity must be proven before migrating strategies (don't migrate to a broken foundation).
- Migration is the highest-effort phase but is straightforward once the foundation is solid.
- Advanced features are valuable but not blocking -- strategies work without them.

**Defer explicitly:**
- Replay mode: Requires both ESDB persistence AND a test harness. High value but high effort. Ship v3.0 without it, add in v3.1.
- Standalone datasource scripts: Architectural shift (new process type). Needs careful design of stream ownership and backpressure. Better as a separate milestone.
- Composite signals: Nice formalization but strategies already implement composite logic manually. Not blocking.

---

## Complexity Estimates

| Feature | Complexity | Effort Estimate | Risk |
|---------|------------|-----------------|------|
| TradeSignal struct (proto + Go + Python) | Low | 1-2 days | Low -- straightforward protobuf change |
| Signal repository interface + in-memory impl | Med | 2-3 days | Low -- standard Go interface pattern |
| ESDB signal repository | Med | 3-4 days | Med -- ESDB stream naming, serialization, error handling |
| Expand RecordSignal RPC | Low | 1 day | Low -- proto change + handler update |
| Single-signal-per-order rule | Low | 1-2 days | Low -- validation in PlaceOrder handler |
| GetProcessedSignals RPC | Low | 1-2 days | Low -- read from repository |
| Strategy migration (7 strategies) | High | 7-10 days | Med -- each strategy has unique signal patterns |
| Live/sim parity testing | Med | 2-3 days | Med -- proving behavioral equivalence |
| Replay mode | High | 5-7 days | High -- deterministic replay is subtle |
| Standalone datasource scripts | High | 5-7 days | High -- new process type, new deployment concern |
| Composite signals | Med | 3-4 days | Med -- design of combination rules |
| Telemetry integration | Low | 1-2 days | Low -- extends existing OTel infrastructure |

---

## Current State vs Target State

### Current (pre-v3.0)
- **5+ signal types** in Go (`SignalV2`, `RsiTradeSignal`, `SignalTriggeredEvent`, `ExitSignal`, `OpenSignalV3`, `SignalDecision`)
- **No signal persistence** -- signals are ephemeral, lost on process restart
- **Signal emission is telemetry-only** -- `RecordSignal` RPC increments a counter, does not store the signal
- **Strategies couple signal detection and order placement** -- `on_tick()` both detects signals AND calls `PlaceOrder`
- **No replay capability** -- cannot reconstruct what signals a past session produced
- **No signal-to-order linkage** -- `PlaceOrderRequest` has no `signal_id` field

### Target (post-v3.0)
- **1 canonical TradeSignal type** across Go and Python
- **Signals persisted in EventStoreDB** for live playgrounds
- **Signal emission is a first-class operation** -- `EmitSignal` stores the signal, returns a signal_id
- **Strategies decouple signal detection from order placement** -- emit signal first, then place order referencing it
- **Replay possible** from any persisted signal stream
- **Every order traces back to exactly one signal** via `signal_id`

---

## Sources

- Codebase inspection: `src/go/eventmodels/signalv2.go`, `signal_name.go`, `rsitradesignal.go`, `signaltype.go`, `exitsignal.go`, `signal_triggered_event.go` -- existing signal type zoo (HIGH confidence)
- Codebase inspection: `src/clients/python/strategies/base_strategy.py` -- current `record_decision()` and `_flush_decisions()` pattern (HIGH confidence)
- Codebase inspection: `src/clients/python/engine/types.py` -- `SignalDecision`, `OpenSignalV3` types (HIGH confidence)
- Codebase inspection: `src/go/playground.proto` -- existing `RecordSignalRequest` RPC (HIGH confidence)
- Codebase inspection: `src/go/eventconsumers/esdb_consumer_stream.go` -- existing ESDB consumer pattern with generics (HIGH confidence)
- [NautilusTrader Strategies Documentation](https://nautilustrader.io/docs/latest/concepts/strategies/) -- signal/data publishing pattern, research-to-live parity (HIGH confidence)
- [NautilusTrader Message Bus](https://nautilustrader.io/docs/latest/concepts/message_bus/) -- signal as lightweight notification pattern (HIGH confidence)
- [EventStoreDB Projections](https://developers.eventstore.com/server/v5/projections) -- `$by_category`, `$by_event_type` system projections (HIGH confidence)
- [Event Sourcing & Audit Trail for Trading Systems](https://durgaanalytics.com/event_sourcing_audit_trading) -- signal lifecycle, replay harness (MEDIUM confidence)
- [QuantStart: Backtesting Considerations](https://www.quantstart.com/articles/backtesting-systematic-trading-strategies-in-python-considerations-and-open-source-frameworks/) -- live/sim parity patterns (MEDIUM confidence)
- [MQL5: Composite Signals](https://www.mql5.com/en/articles/7759) -- composite signal aggregation with logical operators (MEDIUM confidence)
- [Enterprise Integration Patterns: Aggregator](https://www.enterpriseintegrationpatterns.com/patterns/messaging/Aggregator.html) -- correlation, completeness, aggregation algorithm (HIGH confidence)

---

*Feature landscape research for: TradeSignal Framework (v3.0) on Go + Python event-driven trading platform*
*Researched: 2026-03-30*
