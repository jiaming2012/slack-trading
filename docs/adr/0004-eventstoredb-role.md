# PostgreSQL is the source of truth; EventStoreDB is an audit and replay-testing trail

**Status: Proposed (awaiting operator decision)**

PostgreSQL holds the authoritative operational and analytical state — playgrounds, orders, trades, equity plots, and live accounts — and every downstream decision reads from it. EventStoreDB captures a durable append-only log of trade and market events for audit, debugging, and replay testing, but is never the store any operational read is rebuilt from. This is Pattern B from Finding 10 of the trading-stack gap analysis, and it is also — with one gap — what the code already does today. The one gap is that trade events do not yet flow into EventStoreDB at all; the v4 architecture assumes an "EventStoreDB — trade events" log that has not been built. Adopting Pattern B means building that append path deliberately as a write-*alongside* audit trail, not promoting EventStoreDB to source of truth.

## Current-State Evidence

The stack is already a hybrid tilted toward Pattern B — PostgreSQL is authoritative for all trading state, and EventStoreDB is an event stream for signals and market data with lightweight in-memory projections. It is *not* an event-sourced backbone; no PostgreSQL table is rebuilt from EventStoreDB events.

**PostgreSQL is the operational source of truth for trading state.**
- GORM auto-migrates the core trading tables: `LiveAccount`, `LiveAccountPlot`, `TradeRecord`, `OrderRecord`, `Playground`, `EquityPlotRecord` (`src/go/dbutils/postgres.go:64-84`).
- On startup, playgrounds — with their orders, trades, reconcile trades, and equity plots — are loaded exclusively from PostgreSQL via a deep GORM `Preload` join (`src/go/data/database_service.go:73-95`, `LoadPlaygrounds`). There is no equivalent load from EventStoreDB.
- Writes to trading state go to PostgreSQL through the `DatabaseService` (e.g. `SavePlayground`, `src/go/data/database_service.go:910`).

**EventStoreDB is used as an event stream for signals and market data — not for trades.**
- The producer subscribes/writes streams for signals, trackers, stock ticks, option-chain ticks, accounts, account strategies, option alerts, option contracts, trade *signals*, and fx ticks (`src/go/eventproducers/esdb_producer.go:347-357`; append via `InsertEvent`/`AppendToStream`, `src/go/eventstore/db.go:11-21`).
- At startup only four streams are wired: `AccountsStream`, `OptionAlertsStream`, `OptionChainTickStream`, `StockTickStream` (`cmd/main.go:378-383`).
- The signal repository is a read-through EventStoreDB store: `Write` appends to the global `trade-signals` stream, and reads fetch the whole stream and filter in Go (`src/go/backtester-api/models/signal_repository_esdb.go:31-92`). This is the closest thing to event sourcing in the system, and it is scoped to signals only.

**A projection/replay mechanism exists, but only for EventStoreDB-native streams and only into memory.**
- `esdbConsumer` replays a stream from its last event number and then subscribes for new events, appending each into an in-memory `savedEvents` slice (`src/go/eventconsumers/esdb_consumer.go:143-201`, `replayEvents` + `subscribeToStream`). This rebuilds in-memory caches (trackers, option contracts) from the log — but never rebuilds a PostgreSQL table.

**Trade and order events are not in EventStoreDB at all.**
- No `OrderRecord` or `TradeRecord` is written to any EventStoreDB stream anywhere in `src/go/eventproducers/` or `src/go/eventstore/` (grep: no matches). Trade history lives only in PostgreSQL.

**The v4 architecture assumes a trade-event log that does not yet exist.**
- `todo/full-trading-stack-architecture.md:468` lists "Event log: EventStoreDB — trade events" and "Feature / result store: PostgreSQL". The trade-event log is aspirational; today trades are PostgreSQL-only.

## Considered Options

- **Pattern A — Event-Sourced Backbone.** EventStoreDB becomes the source of truth for trade events; the PostgreSQL trading tables (`sim_outcomes`, order/trade records, and the v4 analytical tables) become projections rebuilt by replaying the event log. Gives full temporal replay, time-travel debugging, and audit-grade lineage.
  - *Cost:* significant — ~2–4 weeks to build and maintain projection infrastructure (projection builders, checkpoint/rebuild tooling, idempotent apply logic, migration of the existing PostgreSQL-authoritative load path off GORM as source of truth).
  - *Benefit:* full temporal replay — invaluable for debugging optimizer behavior and answering regulatory/audit questions by reconstructing exact historical state.
  - *Trade-off:* every schema change becomes a projection-versioning problem; the current straightforward GORM `LoadPlaygrounds` path (`database_service.go:73`) would be replaced by replay-and-project, adding operational surface a solo operator must maintain.

- **Pattern B — PostgreSQL Primary, EventStoreDB as Audit Trail (recommended).** PostgreSQL stays the source of truth for all operational and analytical data. EventStoreDB captures raw trade and market events for compliance, debugging, and replay *testing* only — never as the store operational reads depend on.
  - *Cost:* minimal — mostly building the trade-event append path (write-alongside, not write-through) plus documenting the consistency contract. Reuses the existing `EsdbProducer.Save` / `AppendToStream` plumbing.
  - *Benefit:* keeps the operational store simple and familiar (GORM/PostgreSQL), preserves an immutable audit/replay log, and defers projection complexity until scale or compliance justifies it.
  - *Trade-off:* no *native* replay of operational state — rebuilding PostgreSQL from events is not a supported operation. Replay is a testing/audit affordance, not a recovery mechanism (PostgreSQL backups per gap-analysis Finding 3 remain the recovery path).

## Recommendation

Adopt **Pattern B**. Rationale (three points):

1. **It matches reality and the operator's scale.** The code already treats PostgreSQL as the source of truth and EventStoreDB as a stream for signals and market data; Pattern B ratifies that split and closes the one gap (no trade-event log) with a low-cost write-alongside append, whereas Pattern A would require re-architecting the working GORM load path for a solo-operator stack that has no regulatory replay mandate today.
2. **It keeps the operational surface small.** For a single-operator system, projection infrastructure is standing operational cost — checkpoint management, rebuild tooling, and schema-versioned projections — that buys temporal replay the operator does not yet need; Pattern B spends that budget on the higher-priority gaps (circuit breaker, broker-side stops, portfolio risk overlay) instead.
3. **It is a strict subset of Pattern A, so nothing is foreclosed.** If audit-grade replay or time-travel debugging of the optimizer loops later becomes worth the cost, the same event log becomes the input to projection builders — provided the events are captured with the fields and identity discipline described below, the later move is additive, not a rewrite.

## What Pattern B Means for the v4 Trading Stack Tables

The v4 analytical tables (`scan_results`, `sim_outcomes`, `simulator_fidelity`, `strategy_ev_weights`, `scanner_configs`, `feature_distributions`) are **PostgreSQL source of truth**. Every optimizer read, EV-tracker aggregation, and fidelity join reads from PostgreSQL, exactly as the v4 SQL in `full-trading-stack-architecture.md` assumes. EventStoreDB does **not** back any of these tables and is never queried to satisfy an operational read.

EventStoreDB's role narrows to **capturing trade events for audit and replay testing only**:
- When a live trade fills (and, optionally, when a simulated outcome is written), append an immutable trade event to a dedicated stream (e.g. `trade-events`) *alongside* the PostgreSQL write, reusing `EsdbProducer.Save` / `AppendToStream`.
- The trade-event log is the raw record for: compliance/audit ("prove what happened and when"), post-hoc debugging (replay a period through the fidelity checker), and replay *testing* (re-run the simulator over a captured live window). It is not a recovery store and is not read on the hot path.
- Consistency contract: the PostgreSQL write is authoritative. The EventStoreDB append is best-effort audit; if it fails, log and alert but do not fail the trade. Reconciliation, if ever needed, flows PostgreSQL → EventStoreDB, never the reverse.

## Keeping Pattern A Achievable Later Without a Rewrite

Capture events now so the log is replayable into projections later. Schema-design guidance for the trade-event stream:

- **Events carry stable, explicit identity.** Every trade event embeds the PostgreSQL primary keys it corresponds to (`playground_id`, `order_id`, `trade_id`, and `scan_result_id` / `strategy_id` where relevant) so a future projection can key rows deterministically. The codebase already uses `uuid` primary keys (`Playground`, `OrderRecord`, `TradeRecord`) — reuse those exact IDs in the event payload; never invent event-only surrogate keys.
- **Events are complete and self-describing.** Store the full post-state of the changed entity (or a well-defined delta), not a pointer that requires a PostgreSQL lookup to interpret. A projection must be reconstructable from the stream alone.
- **Version every event.** Continue the existing `SchemaVersion` discipline (`esdb_producer.go:64-66,117-119`) on trade events so a later projection builder can branch on schema version rather than guess.
- **One append per state transition, append-only.** Model the order/trade lifecycle (`pending → filled/rejected/canceled`, fills, closes, reconciles) as discrete events rather than overwrites, so replay reconstructs history, not just the latest snapshot.
- **Deterministic ordering and timestamps.** Include the domain timestamp (fill time, simulated-at) in the payload, not only the EventStoreDB commit time, so replay is order-independent of ingestion timing.
- **Stream-per-aggregate where practical.** Prefer per-playground or per-account streams (mirroring the existing `AccountsStream` pattern) over one giant stream, so a future projection can rebuild a single aggregate without scanning everything.

Following this discipline, promoting to Pattern A later is "write projection builders that consume the existing log," not "re-instrument the trade path."

## Consequences

- A trade-event append path must be added (the v4 log does not exist yet). It is write-*alongside* PostgreSQL, best-effort, and must not gate or fail a trade — reusing existing `EsdbProducer` plumbing.
- EventStoreDB is explicitly demoted from "possible source of truth" to "audit/replay-test log." Operational recovery is PostgreSQL backups (gap-analysis Finding 3), not event replay.
- The existing in-memory-projection consumers (`esdbConsumer` for trackers/contracts, `ESDBSignalRepository` for signals) are unaffected and remain the pattern for EventStoreDB-native streams.
- No operational read may depend on EventStoreDB; any future code that reads trade state from the event log on the hot path is, by this decision, a bug.
- The trade-event schema must follow the identity/versioning/append-only guidance above from day one, so Pattern A stays a purely additive future option.
- Deferred and explicitly *not* covered by this decision: building projection infrastructure, temporal-replay tooling, and any regulatory-grade reconstruction — those are the Pattern A follow-on if and when scale or compliance justifies the cost.
