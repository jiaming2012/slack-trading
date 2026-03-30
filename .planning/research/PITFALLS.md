# Domain Pitfalls: TradeSignal Framework for Event-Driven Trading Platform

**Domain:** Event-sourced signal framework added to existing Go+Python trading system
**Researched:** 2026-03-30
**Overall confidence:** MEDIUM-HIGH

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or silent correctness bugs in production trading.

---

### C1: Stream-Per-Signal-Name Explosion in EventStoreDB (CRITICAL)

**What goes wrong:** The natural instinct is to create one ESDB stream per signal name (e.g., `signals-rsi-oversold`, `signals-pdf-bullish`, `signals-covered-call-entry`). As strategies proliferate, you end up with hundreds of fine-grained streams. EventStoreDB handles many streams fine mechanically, but querying across them becomes painful -- you need projections or `$ce-` category streams to answer "what signals fired for AAPL today?" and those projections add latency and operational complexity.

**Why it happens:** The existing codebase already follows a per-entity stream pattern (e.g., `candles-AAPL`, `stock-ticks-SPY` via `NewCandleStreamName`/`NewStockTickStreamName`). It feels natural to extend this to `signals-{name}`. But signals are not independent aggregates -- they are events within a trading context.

**Consequences:**
- Cannot efficiently replay "all signals for symbol X in time range Y" without projections
- Category projections (`$ce-signals`) add a second system to maintain
- Stream naming becomes a hidden schema that couples producers and consumers

**Prevention:**
- Use `signals-{symbol}` as the primary stream (e.g., `signals-AAPL`). Each event carries its signal name as a field, not encoded in the stream name.
- This matches the query pattern: "replay all signals for AAPL" is the primary use case, not "replay all RSI signals across all symbols"
- For cross-symbol queries (rare), use ESDB's `$ce-signals` category projection which is built-in and zero-config
- Add a `signalName` field to the event body for filtering during replay

**Detection:** If you find yourself writing `NewSignalStreamName(signalName, symbol)` with two components, you are over-partitioning.

**Phase:** Address in Phase 1 (TradeSignal struct + ESDB stream design). Get this wrong and every subsequent phase builds on the wrong foundation.

**Confidence:** HIGH -- validated against existing codebase patterns in `stream_names.go` and EventStoreDB community guidance.

---

### C2: Replay Clock vs. Signal Timestamp Mismatch (CRITICAL)

**What goes wrong:** During replay mode, persisted signals carry their original wall-clock timestamps. The backtester's `Clock` struct advances via `NextTick` calls from Python. If the replay feeds signals based on wall-clock time but the sim clock jumps in discrete candle intervals (e.g., 5-minute bars), signals that arrived between bar boundaries get either dropped or processed at the wrong simulated time.

**Why it happens:** The existing backtester loop is candle-driven: Python calls `NextTick`, Go advances the clock by one candle period, returns new candles. Signals in live mode arrive asynchronously at arbitrary wall-clock times. When replaying these signals into the candle-driven sim, there is no natural synchronization point.

**Consequences:**
- Signals fire "too early" (before the candle that caused them has been processed) -- lookahead bias
- Signals fire "too late" (batched into the next tick after their candle) -- missed opportunities that live would have caught
- Backtests produce different P&L than live runs on the same data, destroying the live/sim parity goal

**Prevention:**
- Define a strict ordering contract: signals are only visible to the strategy after the tick that includes their timestamp. During replay, buffer signals and release them to the strategy only when `Clock.GetCurrentTime() >= signal.Timestamp`.
- Store signal timestamps as the candle close time that triggered them, not the wall-clock time the Python process emitted them. This is the "logical time" that the sim clock understands.
- In the `NextTick` RPC handler, include a step that drains buffered signals whose timestamp <= current tick time and returns them alongside candle data.
- Write an integration test that replays known signals and asserts identical order placement to the original run.

**Detection:** Run the same strategy in sim-from-live-signals mode and compare order timestamps. If orders shift by one tick period, you have the off-by-one clock bug.

**Phase:** Address in Phase 2 (signal replay implementation). This is the hardest correctness bug and should be caught by integration tests before any strategy migration.

**Confidence:** HIGH -- directly derived from the existing `Clock` struct in `models/clock.go` and the tick-driven loop architecture.

---

### C3: Strategy Migration Breaks Existing Behavior Silently (CRITICAL)

**What goes wrong:** When migrating `MeanReversionStrategy`, `CoveredCallStrategy`, `WheelStrategy`, etc. to use TradeSignal, subtle behavioral changes creep in. The current strategies call `self.playground.place_order()` inline during `on_tick()`. The new model requires first emitting a TradeSignal, then having the signal-to-order pipeline place the order. If the pipeline introduces even one tick of latency, the order executes at a different price. If signal deduplication logic differs from the original conditional checks, orders get placed or suppressed differently.

**Why it happens:** Current strategies embed signal logic and order placement in a single code path (e.g., `mean_reversion.py` lines 473, 594, 702 all call `place_order` directly). Splitting this into "emit signal" + "react to signal" introduces a seam where timing, deduplication, and state can diverge.

**Consequences:**
- P&L regression that looks like a bug but is actually a behavioral change
- Strategies behave differently in backtest vs. live because the signal pipeline has different latency characteristics
- The `test_demo_covered_call.py` regression test (which pins reference metrics) starts failing for non-obvious reasons

**Prevention:**
- Migrate one strategy at a time, starting with the simplest (CoveredCall or Wheel, not MeanReversion)
- For each migration, run the original strategy and the migrated strategy on the same historical data and diff the order logs. Require identical order placement before declaring migration complete.
- Keep original strategies in `deprecated/` but runnable for comparison until all migrations pass diff tests
- The "single-signal-per-order" rule must be enforced at the pipeline level, not by convention in strategy code
- Do NOT change signal generation logic during migration. Migration is a refactor, not an enhancement.

**Detection:** Run `test_demo_covered_call.py` (or equivalent per-strategy regression test) before and after migration. Any metric drift beyond floating-point noise indicates a behavioral change.

**Phase:** Address in Phase 4+ (strategy migration). Each strategy migration is its own sub-phase with mandatory diff testing.

**Confidence:** HIGH -- directly observed in `strategies/mean_reversion.py`, `strategies/covered_call.py`, and `strategies/wheel.py`.

---

### C4: Live/Sim Signal Source Divergence (CRITICAL)

**What goes wrong:** In live mode, signals come from real-time data sources (Polygon WebSocket, indicator computations on live candles). In sim mode, signals come from either (a) replayed ESDB streams or (b) computed from historical candle data. If the signal generation code differs between these paths -- even slightly -- the "same code, only env vars differ" parity goal is violated.

**Why it happens:** It is tempting to create separate signal producers: a `LiveSignalProducer` that subscribes to real-time data, and a `ReplaySignalProducer` that reads from ESDB. Over time, bug fixes get applied to one but not the other. Or the live producer uses a slightly different indicator library version than the sim producer.

**Consequences:**
- Strategies that work in backtests fail in live, or vice versa
- Debugging requires determining which signal source was active, adding investigation overhead
- Confidence in backtests erodes because "the sim doesn't work like live"

**Prevention:**
- Single signal computation path: both live and sim modes use the same signal generation code. The only difference is the data source (live candle feed vs. historical candle feed).
- Signals should be computed by the strategy, not by a separate infrastructure layer. The strategy calls `detect_signals(candles)` regardless of where the candles came from.
- ESDB persistence is a side-effect of signal generation, not the source of truth for sim. In sim mode, signals are recomputed from candle data. ESDB replay is only for the "replay from persisted signals" mode, which is a separate feature from normal backtesting.
- Document the three modes clearly: (1) backtest = signals computed from historical candles, (2) live = signals computed from live candles + persisted to ESDB, (3) replay = signals read from ESDB. Mode 1 and 2 use the same code path. Mode 3 is a separate, opt-in feature.

**Detection:** If you have an `if environment == "live": ... else: ...` branch in signal generation code, you have diverged.

**Phase:** Address in Phase 1 (architecture decisions) and enforced throughout all phases.

**Confidence:** HIGH -- derived from PROJECT.md requirement "same code, only env vars differ" and existing codebase `PlaygroundEnvironment` enum.

---

## Moderate Pitfalls

### M1: EventStoreDB Schema Evolution Without Upcasters

**What goes wrong:** The TradeSignal struct evolves over phases (new attributes, renamed fields). Old events in ESDB streams become unreadable or silently lose data when deserialized into the latest struct version.

**Why it happens:** EventStoreDB stores raw JSON. There is no schema registry. The existing codebase already uses `SchemaVersion` in `SavedEventParameters` (see `saved_event_parameters.go`), but there is no upcasting infrastructure -- old events are just deserialized and fields that don't match are silently zeroed.

**Prevention:**
- Define `TradeSignal` with a `SchemaVersion` field from day one
- Write an upcaster function `UpcastTradeSignal(version int, raw json.RawMessage) (TradeSignal, error)` that handles version migrations
- Never rename or remove fields -- only add new fields with defaults
- Store the schema version in ESDB event metadata (the existing pattern via `GetSavedEventParameters().SchemaVersion`)

**Phase:** Address in Phase 1 (TradeSignal struct definition). Low effort but high regret if skipped.

**Confidence:** MEDIUM -- based on existing `SchemaVersion` pattern in codebase and general event sourcing best practices.

---

### M2: Composite Signal Complexity Explosion

**What goes wrong:** The v3.0 goal mentions standalone datasource scripts producing signals. Multiple signal sources get composed: "PDF bullish on 1H AND RSI oversold on 5M AND price below deviation band." The composition logic becomes a combinatorial nightmare -- N signals with M possible combinations, each with different timing windows.

**Why it happens:** Each signal is simple in isolation. Composition feels simple ("just AND them together"). But AND requires temporal alignment (both signals must be active within a time window), conflict resolution (what if one signal says buy and another says hold?), and priority ordering (which signal wins when they disagree?).

**Consequences:**
- Strategy code becomes unreadable chains of `if signal_a.active and signal_b.active and not signal_c.active`
- Debugging why a trade was or wasn't taken requires reconstructing the state of 3-5 signals at a specific moment
- Adding a new signal requires updating every composition rule

**Prevention:**
- Start with single-signal strategies. The "single-signal-per-order" rule in PROJECT.md is correct -- enforce it strictly.
- If composition is needed later, build a `SignalComposer` that takes a declarative rule (YAML or config) rather than imperative code
- Log the full signal state at every decision point (already partially done via `record_decision` / `SignalDecision`)
- Defer composite signals to a later milestone. v3.0 should prove the single-signal framework works before adding composition.

**Phase:** Explicitly defer to post-v3.0. If composition creeps into v3.0, flag it as scope creep.

**Confidence:** MEDIUM -- based on MeanReversionStrategy's existing multi-signal logic (HTF + LTF + deviation bands).

---

### M3: ESDB Subscription Reconnection During Signal Replay

**What goes wrong:** During replay mode, the system reads historical signals from ESDB. If the ESDB connection drops mid-replay (network blip, container restart), the existing reconnection logic in `esdb_consumer_stream.go` resubscribes from `lastEventNumber`. But this can cause duplicate signal processing if the strategy already acted on events before the event number was checkpointed.

**Why it happens:** The existing code at line 123-131 of `esdb_consumer_stream.go` resubscribes on drop. It tracks `lastEventNumber` but there is no idempotency guarantee on the consumer side -- if a signal was processed (order placed) but `lastEventNumber` wasn't updated, the reconnection replays that signal and potentially places a duplicate order.

**Prevention:**
- Track processed signal IDs (UUIDs) in the strategy/order pipeline, not just event numbers
- Use the "single-signal-per-order" rule as a natural deduplication check: if an order already exists for signal X, skip
- For replay mode specifically, read all events in a batch before processing (don't use streaming subscriptions for replay -- use `ReadStream` with a read-to-end approach)
- Reserve `SubscribeToStream` for live mode only

**Detection:** Duplicate orders appearing in backtest results. Check if the same signal UUID appears in two order records.

**Phase:** Address in Phase 2 (replay implementation). Use `ReadStream` for replay, `SubscribeToStream` for live.

**Confidence:** HIGH -- directly observed in `esdb_consumer_stream.go` reconnection logic.

---

### M4: Python Strategy Signal Emission Without Go Server Acknowledgment

**What goes wrong:** Python strategies compute signals and call a new RPC to persist them. If the RPC fails (network, server busy), the signal is lost. The strategy has already moved past that tick. The signal never gets persisted to ESDB, creating gaps in the signal stream.

**Why it happens:** The existing `RecordSignal` RPC call in `base_strategy.py` (line 148-159) already treats failures as non-fatal: `_signal_logger.debug("RecordSignal RPC failed (non-fatal): %s", e)`. If signal persistence follows the same pattern, gaps become invisible.

**Prevention:**
- Signal persistence must be synchronous and acknowledged before the strategy proceeds to the next tick
- If persistence fails, the strategy should retry or halt -- not silently continue
- Alternatively, buffer signals client-side and batch-persist with retry. But the buffer must survive process restarts (write to a local file or SQLite)
- For sim mode, signals don't need ESDB persistence (they are in-memory), so this pitfall only applies to live mode

**Detection:** Compare the count of signals in ESDB stream vs. the count of signal log lines in Loki. If they diverge, signals are being lost.

**Phase:** Address in Phase 2-3 (signal persistence implementation).

**Confidence:** MEDIUM -- extrapolated from existing `RecordSignal` error handling pattern.

---

### M5: Order-Signal Traceability Broken Across Process Boundary

**What goes wrong:** The "single-signal-per-order" rule requires every `PlaceOrderRequest` to reference the TradeSignal that caused it. But the signal is generated in Python and the order is placed via Twirp RPC to Go. If the signal ID isn't propagated through the RPC call, the Go server has no way to enforce or audit the signal-to-order link.

**Why it happens:** The current `PlaceOrderRequest` proto doesn't have a signal ID field. Adding it requires a proto change, regeneration of stubs, and updates to every strategy's `place_order` call site.

**Prevention:**
- Add `signal_id` (string UUID) to the `PlaceOrderRequest` proto message from the start
- Make it required in the Go handler -- reject orders without a signal ID
- Store the signal ID on the `OrderRecord` GORM model for full audit trail
- This is the linchpin of the entire framework -- if orders can be placed without signal IDs, the framework provides no value

**Detection:** Query `order_records` table for rows where `signal_id IS NULL`. If any exist after migration, the enforcement is leaking.

**Phase:** Address in Phase 1 (proto changes) and Phase 3 (enforcement in Go handler).

**Confidence:** HIGH -- directly derived from proto definition in `playground.proto` and `PlaceOrder` handler in `grpc.go`.

---

## Minor Pitfalls

### N1: Signal Name Proliferation Without Registry

**What goes wrong:** Different strategies emit signals with ad-hoc names (`"rsi_oversold"`, `"RSI_OVERSOLD"`, `"rsi-oversold"`). Without a registry, querying signals by name becomes unreliable.

**Prevention:**
- Define signal names as a Go `const` enum (like `EventName` in `event_names.go`)
- Python strategies reference the same names via generated protobuf enums or a shared constants file
- Validate signal names on the Go side before persisting to ESDB

**Phase:** Phase 1 (TradeSignal struct definition).

---

### N2: ESDB Stream Retention Unbounded Growth

**What goes wrong:** Signal streams grow indefinitely. Unlike candle data in Postgres (which can be partitioned/archived), ESDB streams have no built-in TTL. Over months of live trading, the `signals-AAPL` stream could have millions of events, making full replay slow.

**Prevention:**
- Set ESDB stream metadata with `$maxAge` or `$maxCount` for signal streams
- Use snapshots: periodically write a "signal state summary" event that allows replay to start from a recent point
- For backtesting, signals should be recomputed from candle data (not replayed from ESDB), keeping ESDB replay as an audit/debugging tool

**Phase:** Phase 2 (ESDB stream configuration).

---

### N3: Telemetry Cardinality Explosion from Signal Attributes

**What goes wrong:** OTel metrics with `signal_name`, `symbol`, `timeframe` labels create high-cardinality metric series. With 10 signal types x 20 symbols x 3 timeframes = 600 series per metric. Prometheus scrape costs grow linearly.

**Prevention:**
- Use OTel logs (Loki) for per-signal detail, not metrics
- Metrics should use low-cardinality labels: `signal_outcome` (fired/suppressed), `strategy_name`
- Reserve high-cardinality data for traces (which are sampled)

**Phase:** Phase 5 (telemetry integration).

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|---|---|---|
| TradeSignal struct + ESDB design | C1 (stream explosion), M1 (schema evolution), N1 (name registry) | Design stream-per-symbol, add schema version, define name enum |
| Signal replay implementation | C2 (clock mismatch), M3 (reconnection duplicates) | Strict tick-gating of signals, use ReadStream for replay |
| Signal persistence + Go enforcement | M4 (lost signals), M5 (order-signal link) | Synchronous persistence, signal_id in proto |
| Strategy migration | C3 (behavioral drift) | One-at-a-time migration with diff testing |
| Live/sim parity | C4 (source divergence) | Single computation path, three-mode documentation |
| Telemetry integration | N3 (cardinality) | Logs for detail, metrics for aggregates |

---

## Sources

- Codebase analysis: `src/go/eventmodels/stream_names.go`, `src/go/eventconsumers/esdb_consumer_stream.go`, `src/go/eventconsumers/tracker_consumer_v3.go`, `src/go/backtester-api/models/playground.go`, `src/clients/python/strategies/base_strategy.py`, `src/clients/python/strategies/mean_reversion.py`
- [Stream partitioning guidance - Kurrent Discuss](https://discuss.kurrent.io/t/stream-partitioning-guidance/341) (MEDIUM confidence)
- [Event Sourcing Pattern - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing) (HIGH confidence)
- [Event Sourcing - Martin Fowler](https://martinfowler.com/eaaDev/EventSourcing.html) (HIGH confidence)
- [Event Sourcing & Audit Trail for Trading Systems](https://durgaanalytics.com/event_sourcing_audit_trading) (MEDIUM confidence)
- [Event Sourcing Explained - BayTech 2025](https://www.baytechconsulting.com/blog/event-sourcing-explained-2025) (MEDIUM confidence)
- [EventStoreDB Event Streams Documentation](https://docs.kurrent.io/server/v22.10/streams) (HIGH confidence)
