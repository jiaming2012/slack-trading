# kill-switch — delta for wire-companion-stops

## ADDED Requirements

### Requirement: Halt rejection leaves no orphan pending order rows

When a live-Mode (non-Simulation) order is rejected at the halt-gated order-submission path after its database row has been pre-created, that row SHALL be finalized rather than abandoned: its status SHALL be set to rejected with the rejection reason recorded, and the finalized row SHALL be persisted. No order row SHALL remain in the pending state as a result of a halt rejection. The same finalization SHALL apply to any placement failure that occurs after the row is pre-created, so the database never accumulates pending rows for orders that never entered the order pipeline.

#### Scenario: Halt-rejected live order row is finalized as rejected

- **WHEN** the halt is engaged and a Paper or Margin order submission is rejected by the order gate after its database row was pre-created
- **THEN** that row SHALL be persisted with status rejected and a reason recording the halt, and SHALL NOT remain pending

#### Scenario: Simulation submission path is unchanged

- **WHEN** a Simulation-Mode order is rejected by the halt
- **THEN** no database row finalization is required because Simulation orders pre-create no database row, and the Simulation path SHALL behave exactly as before

### Requirement: Option auto-closes defer gracefully during a halt

Option-assignment and option-expiration auto-closes generated during a Tick SHALL NOT fail the Tick while the halt is engaged. The system SHALL consult the order gate before placing an auto-close; while the halt is engaged, the constructed auto-close request SHALL be deferred — retained with its fill parameters and retried on each subsequent Tick — and the Tick SHALL complete normally. Once the halt clears, a deferred auto-close SHALL be placed and committed as it would have been originally. Each deferral SHALL be logged as a warning and reflected in the internal Telemetry registry, and an Alert SHALL be raised while deferred auto-closes are outstanding, because a deferred close is open exposure the operator must know about.

#### Scenario: Assignment auto-close during a halt does not fail the tick

- **WHEN** the halt is engaged and an option-assignment event requires an auto-close during a Tick
- **THEN** the Tick SHALL complete without error, the auto-close SHALL be deferred with its fill parameters, a warning SHALL be logged, and Telemetry SHALL reflect the outstanding deferral

#### Scenario: Deferred auto-close commits after release

- **WHEN** deferred auto-closes are outstanding and the halt is acknowledged and released
- **THEN** the next Tick SHALL place and commit the deferred auto-closes with their retained fill parameters

#### Scenario: Outstanding deferrals raise an Alert

- **WHEN** one or more auto-closes are deferred because of an engaged halt
- **THEN** an Alert SHALL be pushed to the operator identifying the deferred exposure

#### Scenario: Auto-closes unaffected while clear

- **WHEN** the halt is clear and an assignment or expiration event requires an auto-close
- **THEN** the auto-close SHALL be placed and committed within the same Tick exactly as today
