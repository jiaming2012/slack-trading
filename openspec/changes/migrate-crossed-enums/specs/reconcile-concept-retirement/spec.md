# reconcile-concept-retirement

## ADDED Requirements

### Requirement: No operator-selectable reconcile mode

There SHALL be no `Reconcile` (or `Reconciliation`) value in the `Mode` type and no code path that lets an operator create or select a reconcile playground. Reconciliation SHALL remain an internal netting layer behind the Broker seam per `CONTEXT.md`, invisible to the operator and to strategies.

#### Scenario: Reconcile is not a selectable mode

- **WHEN** the set of valid `Mode` values is enumerated
- **THEN** it contains exactly `Simulation`, `Paper`, `Margin` and no reconcile value

#### Scenario: No operator entry point creates a reconcile playground

- **WHEN** the RPC and REST create-playground entry points are inspected
- **THEN** none of them accept a reconcile mode or expose reconcile as an operator-creatable playground kind

### Requirement: Pre-existing reconcile rows still load

Legacy playground rows persisted with `environment="reconcile"` (and `live_account_type="reconcilation"`) SHALL continue to load without crashing the server, being handled as the internal reconciliation mechanism rather than as an operator mode, so that starting the server against existing production data does not fail.

#### Scenario: Loading a legacy reconcile row does not crash

- **WHEN** the server loads a persisted row with `environment="reconcile"`
- **THEN** the load completes without panicking or aborting startup, and the row is treated as internal reconciliation state, not surfaced as a selectable operator mode

### Requirement: Reconciliation stays behind the Broker seam

The netting translation between logical playground positions and the shared broker account SHALL remain internal to the Broker seam and apply across the three modes, so that removing the operator-facing concept does not remove the internal reconciliation behavior.

#### Scenario: Netting behavior is preserved after retirement

- **WHEN** the backtester test suite exercising order placement and netting is run against the post-migration tree
- **THEN** `task test` passes with no regression in reconciliation/netting behavior (verification gate G2)
