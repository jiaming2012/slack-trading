# mode-presets Specification

## Purpose
TBD - created by archiving change migrate-crossed-enums. Update Purpose after archive.
## Requirements
### Requirement: Single Mode type with exactly three values

The code SHALL define one `Mode` type whose only valid values are `Simulation`, `Paper`, and `Margin`, each being a named preset that binds a feed source to a broker execution venue per `CONTEXT.md`. A `Validate` (or equivalent) method SHALL accept exactly those three values and reject any other value.

#### Scenario: The three presets validate

- **WHEN** `Mode` is validated against each of `Simulation`, `Paper`, and `Margin`
- **THEN** validation returns no error for all three

#### Scenario: An unknown mode is rejected

- **WHEN** `Mode` is validated against a value that is not one of the three presets (for example `"reconcile"` or `"mock"`)
- **THEN** validation returns a non-nil error

### Requirement: Crossed enums removed from the operator-facing code surface

The Go type `PlaygroundEnvironment` SHALL NOT be declared or referenced anywhere in `src/go/**` non-test source after this change, and the `Meta` playground metadata SHALL carry a single `Mode` field in place of the former `Environment` + `LiveAccountType` field pair. The `LiveAccountType` identifier SHALL likewise not remain: its operator-facing uses are replaced by `Mode`, and its internal per-order / per-account tag uses (order routing on `reconcilation`, trade linking on `simulator`, broker selection on `mock`, store map keys) are renamed to an explicitly internal `AccountRole` type carrying the same five persisted string values (`mock`, `simulator`, `paper`, `margin`, `reconcilation`). `AccountRole` SHALL be documented as internal-behind-the-Broker-seam and SHALL never appear in operator-facing RPC surfaces or playground-creation flows.

*(Amended 2026-07-05 during implementation: the original requirement collapsed both the Meta pairing AND the internal tag into `Mode`; the internal tag is behaviorally load-bearing and persisted in `order_records.account_type` / `live_accounts.account_type`, which must stay byte-identical. See design.md "Design amendment".)*

#### Scenario: No references to the retired enum types remain

- **WHEN** the `src/go/**` Go sources are searched for the identifiers `PlaygroundEnvironment` and `LiveAccountType`
- **THEN** zero non-test source files contain either identifier

#### Scenario: Playground metadata exposes Mode

- **WHEN** a playground's `Meta` is inspected in code
- **THEN** it has a single `Mode` field and no `Environment` or `LiveAccountType` field

#### Scenario: Internal AccountRole preserves persisted bytes

- **WHEN** an order record or live account is persisted after the change with the same logical state as before the change
- **THEN** the `order_records.account_type` and `live_accounts.account_type` column values are byte-identical to the pre-change values, including `reconcilation` and `simulator`

#### Scenario: Reconcile branching behavior preserved

- **WHEN** a reconcile-stamped order's trades are read, filled, or rolled back
- **THEN** routing between `ReconcileTrades`/`Trades` and `ReconcileOrderID`/`OrderID` behaves exactly as before the change, now keyed on `AccountRole` instead of `LiveAccountType`

### Requirement: Illegal feed/venue combinations are unrepresentable

Because a `Mode` is a single value rather than a cross-product of two independent enums, the code SHALL make contradictory combinations (such as a live venue paired with a simulator account) impossible to construct through the `Mode` type. Every construction path that formerly took an `(environment, liveAccountType)` pair SHALL take a single `Mode`.

#### Scenario: No construction path accepts an environment/account-type pair

- **WHEN** the playground and account construction functions are inspected
- **THEN** none of them accept both a `PlaygroundEnvironment` and a `LiveAccountType` argument; each takes a single `Mode`

#### Scenario: Whole tree builds after enum removal

- **WHEN** `go build ./src/go/... ./cmd/...` runs against the post-migration tree
- **THEN** the command exits 0 with no compilation errors (verification gate G1)

