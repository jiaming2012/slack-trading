# Tasks — close-gorm-db-leaks

## 1. Inventory and baseline

- [x] 1.1 Grep the annotations and call graph to confirm the complete leak-site list (`CreateTransaction`, `SaveOrderRecordTx`, `PlaceOrderChanges.Commit`, and their consumers); record it against the table in design.md.
- [x] 1.2 Capture the pre-change exported `*DatabaseService` method-signature list (all 51) as the baseline to diff against at closeout.
- [x] 1.3 Confirm `go build ./src/go/... ./cmd/...` and `task test` are green at HEAD before editing.

## 2. Narrow store methods (data package)

- [x] 2.1 Add the transaction-free order-save-intent value in `models` (order record + force-new flag) and a narrow batch method on `*DatabaseService`/`orderStore` that persists a slice of intents inside one internally-owned transaction, preserving all-or-nothing semantics.
- [x] 2.2 Add narrow cancel-with-reconciles and reject-with-reconciles store methods that own their transaction, doing the DB writes currently in `Playground.CancelOrder`/`RejectOrder`.
- [x] 2.3 Switch the internal self-call at `database_service.go:730` from `CreateTransaction` to a private `s.db.Transaction(...)`.

## 3. De-leak the carrier and consumers (models / services)

- [x] 3.1 Remove the `*gorm.DB` parameter from `PlaceOrderChanges.Commit` in `place_order_changes.go`.
- [x] 3.2 Rewrite `live_broker.go` and `reconcile_broker.go` to stage transaction-free order-save intents instead of `SaveOrderRecordTx(tx, ...)` closures.
- [x] 3.3 Rewrite `services/order_queue.go` (`:162` and `:364` paths) to persist staged changes through the narrow batch method instead of `CreateTransaction`.
- [x] 3.4 Rewrite `Playground.CancelOrder`/`RejectOrder` to call the narrow store methods; keep the in-memory queue mutations in `models` but outside any handed-out handle.

## 4. Remove the raw-handle public surface

- [x] 4.1 Delete `CreateTransaction` and `SaveOrderRecordTx` from `IDatabaseService` and drop the now-unused `gorm` import from `database_service_interface.go`.
- [x] 4.2 Delete `CreateTransaction` (`database_service.go`) and `SaveOrderRecordTx` (`order_store.go`) from `*DatabaseService`; add the narrow replacement decls; leave all other 49 public signatures untouched.
- [x] 4.3 Update `MockDatabase` to drop the two removed methods and implement the narrow replacements.

## 5. Leak-guard task

- [x] 5.1 Add `test:no-gorm-leaks` to `taskfile.yml`: a static check over the public database-surface files that exits non-zero if the `gorm.DB` token appears on the interface or the exported `*DatabaseService` methods.
- [x] 5.2 Verify the guard fails when `gorm.DB` is temporarily reintroduced and passes on the closed surface.

## 6. Verification and closeout

- [x] 6.1 G1: `go build ./src/go/... ./cmd/...` is green.
- [x] 6.2 G2: `task test` is green with no new failures versus the Wave-0 baseline.
- [x] 6.3 `task test:no-gorm-leaks` exits zero.
- [x] 6.4 Diff the exported `*DatabaseService` signatures against the 6.2 baseline: only the two removed methods and the narrow additions differ; confirm no mutex was added, removed, or rescoped (locking granularity unchanged).
- [ ] 6.5 G6: Fable adversarial diff review approves (bodies moved verbatim, atomicity preserved on every replaced site, public surface and locking preserved).
- [ ] 6.6 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
