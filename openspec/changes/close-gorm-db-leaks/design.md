# Design — close-gorm-db-leaks

## Problem

Two methods on the public database surface hand a raw `*gorm.DB` transaction handle to the `models` and `services` packages:

- `IDatabaseService.CreateTransaction(func(tx *gorm.DB) error) error` — `data/database_service.go:65-70` (annotated as a known leak).
- `IDatabaseService.SaveOrderRecordTx(tx *gorm.DB, order, forceNew) error` — `data/order_store.go:456-461` (annotated as a known leak).

The handle then propagates transitively through `models.PlaceOrderChanges.Commit func(tx *gorm.DB) error` (`place_order_changes.go`), which the Live and Reconcile broker adapters populate with closures that call `SaveOrderRecordTx(tx, ...)`, and which `services/order_queue.go` executes inside a `CreateTransaction`. Additionally `Playground.CancelOrder`/`RejectOrder` open a `CreateTransaction` and mix `tx.Save` calls with in-memory queue mutations.

### Inventory of leak sites (grepped from annotations + call graph)

| Site | File:line | Kind |
|---|---|---|
| `CreateTransaction` decl | `data/database_service.go:69`; iface `database_service_interface.go:31`; mock `mock_database.go:426` | raw handle out |
| `SaveOrderRecordTx` decl | `data/order_store.go:459`; iface `database_service_interface.go:41`; mock `mock_database.go:98` | raw handle in |
| `PlaceOrderChanges.Commit` | `models/place_order_changes.go:6` | handle carrier |
| Cancel/Reject bodies | `models/playground.go:830`, `:901` | `CreateTransaction` consumer |
| Order-queue commit loop | `services/order_queue.go:162`, `:364` | `CreateTransaction` consumer |
| Broker save closures | `models/live_broker.go:169`, `:187`; `models/reconcile_broker.go:35` | `SaveOrderRecordTx` consumer |
| Internal self-call | `data/database_service.go:730` | store calling its own leak method |

## Approach

Confine all transaction control to the `data` package and replace each raw-handle use with a narrow, intention-revealing method. Bodies move verbatim where possible; only the transaction ownership relocates.

1. **Order-save batching.** Introduce a transaction-free save-intent value in `models` (an order record plus its force-new flag) and a store method that persists a slice of them inside one internally-owned transaction with the existing all-or-nothing semantics. This replaces the `CreateTransaction` + `for change { change.Commit(tx) }` loop in `order_queue.go:162` and the `SaveOrderRecordTx(tx, ...)` closures in the Live/Reconcile brokers.
2. **`PlaceOrderChanges` de-leaked.** Drop the `*gorm.DB` parameter from the `Commit` field. Broker adapters stage their order saves as transaction-free intents (or `func() error` closures whose DB work is a call to the narrow batch method), so the queue can hand the whole batch to the store and let the store own atomicity.
3. **Cancel/Reject.** Move the transaction body behind narrow store methods (cancel-with-reconciles / reject-with-reconciles) that own the transaction and do the DB writes; the in-memory queue mutations (`AddToOrderQueue`, `GetPlayground`) stay in `models` but run outside the handed-out handle. `Playground.CancelOrder`/`RejectOrder` call the narrow method instead of `CreateTransaction`.
4. **Internal self-call.** `database_service.go:730` switches from `CreateTransaction` to a private `s.db.Transaction(...)` — the private `s.db` field stays; only the *public* handout is removed.
5. **Interface + mock.** Remove the two methods from `IDatabaseService` and `MockDatabase`; add the narrow replacements to both.
6. **Guard.** Add `task test:no-gorm-leaks` — a grep over the public-surface files (`database_service_interface.go`, and the exported `*DatabaseService` method decls) that fails if `gorm.DB` appears.

## File / package layout

- `models/database_service_interface.go` — remove 2 methods, add narrow decls; drop `gorm` import.
- `models/place_order_changes.go` — `Commit` loses its `*gorm.DB` param.
- `models/playground.go`, `models/live_broker.go`, `models/reconcile_broker.go`, `models/mock_database.go` — switch to narrow methods.
- `data/database_service.go`, `data/order_store.go` — remove public raw-handle methods, add narrow store methods, keep private `s.db`.
- `services/order_queue.go` — switch to the batch save method.
- `taskfile.yml` — `test:no-gorm-leaks` target.

## Data flow (after)

Order queue / broker → stage transaction-free order-save intents → single narrow `data` method → `data` opens one `s.db.Transaction` internally → all intents persisted or none. No `*gorm.DB` crosses the package boundary.

## Out of scope (explicitly)

- `models.RemapAndSavePlayground(tx *gorm.DB, ...)` (`models/id_remap.go`) — a `models`-package free function invoked by `data/database_service.go:918` *inside a store-owned transaction*. It is not reachable through the public database surface; no handle escapes to a consumer, so it stays as-is.
- The package-internal `*gorm.DB` transaction helpers in `data/in_memory.go` (`saveOrderRecordsTx`, `savePlaygroundTx`, `saveBalance`, `saveEquityPlotRecords`) and `orderStore.saveOrderRecordTx` — these are unexported within `data` and do not cross the boundary.
- Any change to the netting/reconciliation semantics, order lifecycle states, or Mode (Simulation/Paper/Margin) behavior. This change is a pure encapsulation refactor: same fills, same persistence, same Broker seam.
- Persisted schema, RPC contracts, operator commands (beyond the guard task).

## Dependency ordering on other batch changes

- Runs on the **refactor track**, after `rename-event-packages`.
- Shares files (`models/*`, `services/order_queue.go`, `data/*`) with `reconcile-models-packages` and `migrate-crossed-enums`; it MUST be serialized with them (not merged in parallel) to avoid collisions. Recommended sequence: `reconcile-models-packages` → `rename-event-packages` → **`close-gorm-db-leaks`** → `migrate-crossed-enums`.
- No trading-stack-v4 greenfield change depends on this.

## Verification gates

- **G1** `go build ./src/go/... ./cmd/...` is green.
- **G2** `task test` is green with no new failures versus the Wave-0 baseline.
- **G6** Fable adversarial diff review — confirms bodies moved verbatim, the 49 non-leak public signatures are untouched, locking granularity is unchanged (no mutex added/removed/rescoped), and atomicity is preserved on every replaced site.
- Plus the change-local guard: `task test:no-gorm-leaks` exits zero.

## Deferred validation

None. This is a Tier A change: G1, G2, and G6 are all runnable overnight with no external dependency (no Polygon feed, no prod DB, no broker). Nothing is deferred.
