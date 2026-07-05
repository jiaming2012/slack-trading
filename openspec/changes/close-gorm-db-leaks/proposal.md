# Close gorm.DB leaks through IDatabaseService

## Why

The database layer hands raw `*gorm.DB` transaction handles out to the `models` and `services` packages through the public `IDatabaseService` surface, so business-logic code writes SQL-level transaction code and the "database owns persistence" boundary is broken. Closing these leaks confines all transaction control to the `data` package while keeping every other behavior identical.

## What Changes

- Remove the two raw-handle methods from the public database surface (`IDatabaseService` and `*DatabaseService`): `CreateTransaction(func(tx *gorm.DB) error) error` and `SaveOrderRecordTx(tx *gorm.DB, ...) error`. **BREAKING** (internal Go API only — no operator-facing, RPC, or on-disk change; the `models`/`services` packages are the sole callers and are updated in the same change).
- Replace every raw-handle call site with an intention-revealing, transaction-free store method that owns its own transaction internally and preserves the current all-or-nothing atomicity:
  - The `PlaceOrderChanges.Commit func(tx *gorm.DB) error` closure field — the carrier that threads a shared `tx` from the order queue into the Simulated/Live/Reconcile broker seam — is replaced with a transaction-free representation, and the order queue persists a batch of order-save intents through one narrow store method.
  - The `Playground.CancelOrder` / `Playground.RejectOrder` bodies that open a `CreateTransaction` and mix `tx.Save` with in-memory queue mutation are moved behind narrow store methods that own the transaction.
- Keep the concrete `*DatabaseService` 51-method public surface otherwise byte-for-byte identical: only the two raw-handle methods are removed and the narrow replacements added; all other method signatures are unchanged.
- **Locking granularity is unchanged** — no mutex is added, removed, widened, or narrowed.
- Update `MockDatabase` to match the new interface (drop the two removed methods, add the narrow replacements).
- Add a deterministic leak-guard Taskfile target (`task test:no-gorm-leaks`) that fails if `gorm.DB` reappears on the public database surface, so the boundary cannot silently regress.

## Capabilities

### New Capabilities

- `database-transaction-encapsulation`

### Modified Capabilities

(none — this repository has no existing specs; everything is new)

## Impact

- **Interface**: `src/go/backtester-api/models/database_service_interface.go` (remove 2 methods, add narrow replacements).
- **Concrete surface**: `src/go/data/database_service.go`, `src/go/data/order_store.go` (remove `CreateTransaction`, `SaveOrderRecordTx`; add narrow store methods; internal callers switched off the raw handle).
- **Carrier type**: `src/go/backtester-api/models/place_order_changes.go` (`Commit` field loses its `*gorm.DB` parameter).
- **Consumers rewritten**: `src/go/backtester-api/models/playground.go` (Cancel/Reject), `src/go/backtester-api/models/live_broker.go`, `src/go/backtester-api/models/reconcile_broker.go`, `src/go/backtester-api/services/order_queue.go`, `src/go/backtester-api/models/mock_database.go`.
- **Build/verify**: `taskfile.yml` gains `test:no-gorm-leaks`.
- **Explicitly untouched**: the package-internal `*gorm.DB` transaction helpers in `src/go/data/in_memory.go` and the `models.RemapAndSavePlayground(tx, ...)` helper — neither is exposed *through* the public database surface (see design.md, Out of scope).
- **Dependencies on other batch changes**: sits on the refactor track after `rename-event-packages`; touches the same `models`/`services`/`data` files as `reconcile-models-packages` and `migrate-crossed-enums`, so it must be sequenced (not run in parallel) with those to avoid merge collisions. No greenfield-v4 change depends on it.
