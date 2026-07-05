# database-transaction-encapsulation

## ADDED Requirements

### Requirement: Public database surface exposes no gorm transaction handles

The public database surface — the `IDatabaseService` interface, the concrete `*DatabaseService` methods, and any implementation of `IDatabaseService` such as `MockDatabase` — MUST NOT declare, accept, or return a `*gorm.DB` value in any method signature. The methods `CreateTransaction(func(tx *gorm.DB) error) error` and `SaveOrderRecordTx(tx *gorm.DB, order *OrderRecord, forceNew bool) error` MUST be removed from that surface. No consumer in the `models` or `services` packages may hold a `*gorm.DB` obtained from the database surface.

#### Scenario: Interface declares no gorm.DB

- **WHEN** `src/go/backtester-api/models/database_service_interface.go` is scanned for the token `gorm.DB`
- **THEN** it appears zero times, and the `gorm` import is no longer present in that file

#### Scenario: Mock implementation still satisfies the interface

- **WHEN** the project is compiled with `go build ./src/go/... ./cmd/...`
- **THEN** the build succeeds and `MockDatabase` compiles as a complete `IDatabaseService` implementation with the two raw-handle methods removed

#### Scenario: Leak-guard task passes on the closed surface

- **WHEN** the operator runs `task test:no-gorm-leaks`
- **THEN** the task exits zero because no `gorm.DB` token is found on the public database surface, and the same task would exit non-zero if `gorm.DB` were reintroduced into the interface or the `*DatabaseService` public methods

### Requirement: Atomic multi-order persistence without a shared transaction handle

A narrow store method on the database surface MUST persist a batch of order-save intents (each an order record plus its force-new flag) within a single transaction owned entirely inside the `data` package. The method MUST be all-or-nothing: if persisting any one intent fails, none of the batch is committed. The order queue and the Simulated, Live, and Reconcile broker paths MUST persist their staged changes through this method instead of threading a caller-supplied `tx` into per-change commit closures.

#### Scenario: Whole batch commits together

- **WHEN** the order queue commits a set of staged order-save intents and every intent succeeds
- **THEN** all order records in the batch are persisted, exactly as the pre-change shared-`tx` path persisted them

#### Scenario: One failure rolls back the whole batch

- **WHEN** one order-save intent in the batch fails
- **THEN** no order record from that batch is committed, matching the rollback behavior of the removed shared-`tx` transaction

#### Scenario: PlaceOrderChanges no longer carries a gorm handle

- **WHEN** `src/go/backtester-api/models/place_order_changes.go` is inspected
- **THEN** the `Commit` field's signature contains no `*gorm.DB` parameter, and the broker adapters build their staged changes without referencing a transaction handle

### Requirement: Order cancel and reject own their transactions internally

The cancel and reject flows for an order (including any reconciliation orders it touches) MUST perform their database writes inside a transaction opened and owned by the `data` package, exposed to `models` only through transaction-free methods. `Playground.CancelOrder` and `Playground.RejectOrder` MUST NOT call any method that yields a `*gorm.DB`.

#### Scenario: Cancel persists atomically with no handle exposed

- **WHEN** an order with reconciliation orders is canceled
- **THEN** the order and its reconciliation orders reach their canceled state and are persisted in one transaction, and no `*gorm.DB` is passed to or from `models` during the flow

#### Scenario: Reject persists atomically with no handle exposed

- **WHEN** an order with reconciliation orders is rejected with a reason
- **THEN** the order and its reconciliation orders reach their rejected state with the reason recorded, persisted in one transaction, and no `*gorm.DB` is passed to or from `models` during the flow

### Requirement: Remaining public surface and locking granularity are preserved

Apart from removing the two raw-handle methods and adding the narrow replacements, every other public method of `*DatabaseService` MUST retain its exact existing signature, and the locking behavior of the database layer MUST be unchanged — no mutex is added, removed, or have its critical section widened or narrowed relative to the pre-change code.

#### Scenario: Non-leak public methods are byte-for-byte unchanged

- **WHEN** the set of exported `*DatabaseService` method signatures is compared before and after the change
- **THEN** the only differences are the removal of `CreateTransaction` and `SaveOrderRecordTx` and the addition of the narrow replacement methods; all other signatures are identical

#### Scenario: Existing test suite passes unchanged

- **WHEN** `task test` is run against the changed tree
- **THEN** the Go unit-test suite passes with no new failures relative to the pre-change baseline, demonstrating that behavior and locking are preserved

### Requirement: Leak-guard verification target exists in the Taskfile

The change MUST add a `test:no-gorm-leaks` target to `taskfile.yml` that statically checks the public database surface for the `gorm.DB` token and exits non-zero if it is present. This target is the deterministic regression guard for the boundary.

#### Scenario: Target is invocable and documented

- **WHEN** the operator lists tasks or runs `task test:no-gorm-leaks`
- **THEN** the target exists, runs the static check over the public database surface files, and returns a clear pass/fail exit code
