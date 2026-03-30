# Project Research Summary

**Project:** TradeSignal Framework (v3.0 milestone)
**Domain:** Event-sourced trading signal framework for Go+Python platform
**Researched:** 2026-03-30
**Confidence:** HIGH

## Executive Summary

The TradeSignal framework is an additive layer on the existing slack-trading platform that introduces a canonical signal type, decouples signal detection from order placement, and creates an auditable signal-to-order chain. The platform already has nearly everything needed: EventStoreDB with generic consumer/producer infrastructure, Twirp RPC, environment-based interface swapping (IBroker pattern), and FIFOQueue-based tick delivery. The only new external dependency is `esdbclient` for Python -- a single pip package. This is fundamentally an architecture refactor, not a technology adoption project.

The recommended approach is to build the Go-side foundation first (TradeSignal struct, repository interface, proto changes), validate with in-memory simulation, then layer on ESDB persistence for live mode, and finally migrate existing strategies one at a time with mandatory behavioral diff testing. The entire framework follows patterns already proven in the codebase: SavedEvent for ESDB types, FIFOQueue for tick-synchronized delivery, and interface-based repository swapping for live/sim parity.

The primary risks are: (1) ESDB stream naming -- researchers disagree on per-signal-name vs. per-symbol partitioning, and the wrong choice forces a migration later; (2) replay clock synchronization -- signals must be gated to the tick clock to avoid lookahead bias; (3) strategy migration behavioral drift -- splitting signal detection from order placement can introduce subtle timing changes that silently alter P&L. All three are manageable with upfront design decisions and integration testing, but none should be deferred.

## Key Findings

### Recommended Stack

Zero new Go dependencies. One new Python dependency: `esdbclient` (official EventStoreDB Python gRPC client). Everything else leverages existing infrastructure.

**Core technologies (all already present):**
- EventStore-Client-Go v4.1.0: Signal persistence via existing `EsdbProducer`/`esdbConsumer` generics
- Twirp RPC v8.1.3: New `WriteSignal` and `GetSignals` endpoints alongside existing `RecordSignal`
- FIFOQueue (eventmodels): Tick-synchronized signal delivery, same pattern as candles and trades
- SavedEvent interface: TradeSignal implements it, plugs into all existing ESDB infrastructure

**New addition:**
- `esdbclient` (Python, >=1.0): For standalone datasource scripts writing signals directly to ESDB. Official client, Python 3.10 compatible. Verify `grpcio` version alignment with existing OTel packages.

**Key disagreement resolved:** STACK.md proposes Python datasources write to ESDB directly via `esdbclient`. ARCHITECTURE.md recommends Python writes via `WriteSignal` Twirp RPC (single-writer principle). The ARCHITECTURE.md approach is safer -- route all writes through the Go server to maintain OTel instrumentation, schema validation, and a single writer to ESDB. `esdbclient` should still be added for diagnostic/replay tooling but not as the primary write path for datasources.

### Expected Features

**Must have (table stakes):**
- TradeSignal canonical struct (Name + Attributes + Timestamp + Symbol) -- replaces 5+ incompatible signal types
- Single-signal-per-order rule -- `signal_id` on `PlaceOrderRequest`, enforced server-side
- Signal persistence to EventStoreDB in live mode
- In-memory signal repository for simulation mode
- Signal emission from Python strategies via RPC
- Live/sim parity -- same strategy code, different repository backend
- Strategy migration -- all 7 existing strategies converted (highest effort item: 7-10 days)

**Should have (differentiators):**
- Replay mode from persisted signal streams
- `GetProcessedSignals` RPC endpoint for debugging
- Telemetry integration (signals in OTel/Grafana)
- Standalone datasource scripts producing signals

**Defer (v3.1+):**
- Composite signals (AND/OR combinations)
- Signal queryability via custom ESDB projections
- ML-based signal scoring
- Real-time signal streaming via WebSocket/gRPC streaming

### Architecture Approach

The framework is additive to the existing two-process architecture. Go server gains a `TradeSignal` domain type, `ISignalRepository` interface (in-memory and ESDB implementations), two new RPC endpoints, and signal delivery via `TickDelta`. Python gains a signal abstraction layer and optional datasource scripts. The key architectural invariant: signal detection stays in Python, signal storage and delivery is managed by Go, and the repository interface abstracts the sim/live boundary.

**Major components:**
1. `TradeSignal` struct (eventmodels) -- canonical domain type implementing SavedEvent
2. `ISignalRepository` interface + two implementations -- in-memory (sim) and ESDB (live)
3. `WriteSignal` / `GetSignals` RPC endpoints -- signal ingestion and query
4. `TickDelta.new_signals` -- clock-synchronized signal delivery to Python strategies
5. `PlaceOrderRequest.signal_id` -- audit chain linking every order to its originating signal
6. Python `SignalRepository` + datasource scripts -- signal production and consumption abstractions

### Critical Pitfalls

1. **ESDB stream naming (C1)** -- Per-signal-name streams cause query explosion. Use per-symbol streams (`signals-AAPL`) with signal name as a field inside the event. This contradicts STACK.md's recommendation of `trade-signals-{signal_name}` -- the per-symbol approach is correct because the primary query pattern is "all signals for symbol X in time range Y."

2. **Replay clock mismatch (C2)** -- Persisted signals carry wall-clock timestamps but the sim clock advances in discrete candle intervals. Signals must be gated: only visible to strategy when `Clock.GetCurrentTime() >= signal.Timestamp`. Integration test required before strategy migration begins.

3. **Strategy migration behavioral drift (C3)** -- Splitting signal detection from order placement introduces a seam where timing can diverge. Migrate one strategy at a time, run original and migrated on same data, diff order logs. Zero tolerance for metric drift.

4. **Live/sim signal source divergence (C4)** -- Must maintain single computation path for both modes. The only difference is data source (live candles vs. historical candles), not signal generation code. Document three modes: backtest (compute from candles), live (compute + persist), replay (read from ESDB).

5. **Order-signal traceability across process boundary (M5)** -- `signal_id` must be added to proto from day one and enforced server-side. Without this, the framework provides no auditable value.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Signal Foundation (Go-side types and proto)
**Rationale:** Everything depends on the TradeSignal struct, ESDB stream naming, and proto messages. This is the foundation that unblocks all other work.
**Delivers:** `TradeSignal` struct implementing SavedEvent, stream name constants, proto messages (TradeSignalProto, WriteSignalRequest, GetSignalsRequest/Response), `signal_id` field on PlaceOrderRequest, signal name registry as Go constants.
**Addresses:** TradeSignal struct, single-signal-per-order rule (proto-level), signal name registry
**Avoids:** C1 (stream naming decided upfront), M1 (schema version from day one), N1 (name registry), M5 (signal_id in proto)

### Phase 2: Signal Repository + Sim Mode
**Rationale:** In-memory repository enables simulation testing without ESDB dependency. Must be stable before ESDB implementation.
**Delivers:** `ISignalRepository` interface, `InMemorySignalRepository`, signal delivery via `TickDelta.new_signals` and `newSignalsQueue` on Playground, unit tests.
**Addresses:** In-memory signal repository, live/sim parity (interface), signal delivery in tick loop
**Avoids:** C2 (clock synchronization tested here)

### Phase 3: RPC Endpoints + Python Integration
**Rationale:** Proto must be generated (Phase 1) and repository interface stable (Phase 2) before wiring RPC handlers. Python client wrappers enable datasource scripts and strategy integration.
**Delivers:** `WriteSignal` and `GetSignals` RPC handlers in `grpc.go`, Python `SignalRepository` wrapper, `write_signal()` and `get_signals()` client methods, datasource script skeleton.
**Addresses:** Signal emission from Python, GetProcessedSignals endpoint
**Avoids:** M4 (synchronous persistence with acknowledgment)

### Phase 4: ESDB Persistence (Live Mode)
**Rationale:** ESDB repository implements the same interface as in-memory (Phase 2). Environment-based injection connects the pieces.
**Delivers:** `ESDBSignalRepository`, environment-based repository injection, signal consumer subscribing to ESDB streams, `esdbclient` installation for Python diagnostic tooling.
**Uses:** Existing `esdbConsumerStream[T]` generics, `EsdbProducer.insert()`, `eventservices.FetchAll[T]()`
**Avoids:** M3 (use ReadStream for replay, SubscribeToStream for live), N2 (set stream $maxAge)

### Phase 5: First Strategy Migration (Validation)
**Rationale:** Migrate the simplest strategy first to validate the full pipeline end-to-end before committing to bulk migration.
**Delivers:** One strategy (CoveredCall or Wheel) fully migrated, original moved to deprecated/, behavioral diff test passing.
**Addresses:** Strategy migration (1 of 7), live/sim parity validation
**Avoids:** C3 (diff testing catches behavioral drift), C4 (single computation path enforced)

### Phase 6: Remaining Strategy Migrations
**Rationale:** With the pipeline validated in Phase 5, migrate remaining strategies. Each is its own sub-unit with mandatory diff testing.
**Delivers:** All 7 strategies migrated, originals in deprecated/, full integration test suite.
**Addresses:** Strategy migration (6 remaining), framework adoption complete

### Phase 7: Replay Mode + Telemetry
**Rationale:** Replay and telemetry are production-readiness features. They depend on ESDB persistence (Phase 4) and migrated strategies (Phase 6).
**Delivers:** Replay mode reading signals from ESDB into InMemorySignalRepository, OTel signal attributes, Grafana dashboard panel, "zero signals" alert.
**Addresses:** Replay mode, telemetry integration
**Avoids:** N3 (use logs for detail, metrics for aggregates only)

### Phase Ordering Rationale

- Phases 1-4 are strictly dependency-ordered: struct -> interface -> RPC -> persistence
- Phase 5 before Phase 6: validate full pipeline with one strategy before bulk migration (de-risks C3)
- Phase 7 after Phase 6: replay mode is meaningless without migrated strategies producing signals
- Phases 1-4 are purely additive Go-side changes with no breaking impact on existing strategies
- Strategy migration (Phases 5-6) is the highest-effort, highest-risk work and should only begin on a proven foundation

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** Stream naming decision needs final validation. STACK.md says per-signal-name, PITFALLS.md says per-symbol. Recommend per-symbol but confirm with a spike reading/writing 1000 events.
- **Phase 4:** ESDB subscription vs. batch read semantics for live mode. Verify `esdbConsumerStream[T]` reconnection behavior with signal deduplication.
- **Phase 5:** First strategy migration will surface unknown integration issues. Budget extra time for discovery.

Phases with standard patterns (skip research-phase):
- **Phase 2:** In-memory repository + FIFOQueue delivery follows exact existing patterns (candles, trades). No unknowns.
- **Phase 3:** RPC handler pattern is well-established in `grpc.go`. Proto changes are mechanical.
- **Phase 6:** Follows the validated pattern from Phase 5. Each strategy migration is repetitive work.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Nearly zero new dependencies. `esdbclient` is the only addition, official and version-compatible. |
| Features | HIGH | Grounded in direct codebase inspection of 5+ existing signal types and 7 strategies. Feature list derived from actual code gaps. |
| Architecture | HIGH | All patterns (SavedEvent, FIFOQueue, IBroker interface swapping) verified against existing implementations in the codebase. |
| Pitfalls | MEDIUM-HIGH | Critical pitfalls derived from codebase analysis. Stream naming disagreement between researchers is a real unresolved tension. |

**Overall confidence:** HIGH

### Gaps to Address

- **Stream naming disagreement:** STACK.md recommends `trade-signals-{signal_name}`, ARCHITECTURE.md recommends `trade-signals-{symbol}`, PITFALLS.md warns against per-signal-name. Resolve in Phase 1 planning with a concrete spike. Recommendation: per-symbol.
- **`esdbclient` + `grpcio` version compatibility:** Not yet tested in the `grodt` conda environment. Test during Phase 4 before relying on it.
- **Attributes type:** STACK.md uses `map[string]interface{}`, ARCHITECTURE.md uses `map[string]string`. Proto supports `map<string, string>`. Recommend `map[string]string` for simplicity and proto compatibility -- strategies can serialize complex values as JSON strings in individual attribute values.
- **`RecordSignal` RPC deprecation timeline:** Both RPCs coexist during migration. Need a clear decision on when (or if) to sunset `RecordSignal` after all strategies are migrated.
- **Signal enforcement strictness:** Should `signal_id` be required on ALL orders or only on orders from migrated strategies? During incremental migration, non-migrated strategies cannot provide signal IDs. Recommend: optional during migration phases, required after Phase 6 completion.

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis of `esdb_consumer.go`, `esdb_consumer_stream.go`, `esdb_producer.go`, `stream_names.go`, `event_names.go`, `saved_event.go`, `playground.go`, `grpc.go`, `playground.proto`, `base_strategy.py`, `trading_engine.py`
- [esdbclient on PyPI](https://pypi.org/project/esdbclient/) -- official Python ESDB client
- [EventStore-Client-Go on GitHub](https://github.com/EventStore/EventStore-Client-Go) -- Go client v4
- [EventStoreDB Category Projections](https://docs.kurrent.io/clients/tcp/dotnet/21.2/projections) -- $by_category stream behavior
- [Event Sourcing - Martin Fowler](https://martinfowler.com/eaaDev/EventSourcing.html) -- foundational patterns
- [NautilusTrader Strategies Documentation](https://nautilustrader.io/docs/latest/concepts/strategies/) -- research-to-live parity patterns

### Secondary (MEDIUM confidence)
- [Event Sourcing & Audit Trail for Trading Systems](https://durgaanalytics.com/event_sourcing_audit_trading) -- signal lifecycle patterns
- [Stream partitioning guidance - Kurrent Discuss](https://discuss.kurrent.io/t/stream-partitioning-guidance/341) -- per-entity vs. per-type streams
- [MQL5: Composite Signals](https://www.mql5.com/en/articles/7759) -- composite signal aggregation (deferred feature)
- [QuantStart: Backtesting Considerations](https://www.quantstart.com/articles/backtesting-systematic-trading-strategies-in-python-considerations-and-open-source-frameworks/) -- live/sim parity

---
*Research completed: 2026-03-30*
*Ready for roadmap: yes*
