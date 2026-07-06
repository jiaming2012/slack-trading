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

Deferrals originate from drain-once assignment/expiration events, so they
SHALL be persisted when deferred, deleted when their close successfully
commits, and reloaded when the playground is loaded — a halt followed by a
process restart SHALL NOT drop a deferred auto-close. A persisted deferral
whose source order no longer has remaining open quantity SHALL be treated as
stale at reload and SHALL NOT be replayed (replaying a committed close would
reverse the position).

The deferred-auto-close Alert SHALL be sticky: it SHALL NOT auto-resolve
merely because the in-memory condition cleared. It SHALL keep notifying until
the operator acknowledges it, and it SHALL resolve only when the outstanding
deferral set is actually empty AND the alert has been acknowledged — a
restart or reset metric can never fake an all-clear that the persisted
deferral set contradicts.

#### Scenario: Assignment auto-close during a halt does not fail the tick

- **WHEN** the halt is engaged and an option-assignment event requires an auto-close during a Tick
- **THEN** the Tick SHALL complete without error, the auto-close SHALL be deferred with its fill parameters, a warning SHALL be logged, and Telemetry SHALL reflect the outstanding deferral

#### Scenario: Deferred auto-close commits after release

- **WHEN** deferred auto-closes are outstanding and the halt is acknowledged and released
- **THEN** the next Tick SHALL place and commit the deferred auto-closes with their retained fill parameters

#### Scenario: Outstanding deferrals raise an Alert

- **WHEN** one or more auto-closes are deferred because of an engaged halt
- **THEN** an Alert SHALL be pushed to the operator identifying the deferred exposure

#### Scenario: Deferred auto-closes survive a restart

- **WHEN** auto-closes are deferred under an engaged halt and the process restarts before the halt clears
- **THEN** the deferrals SHALL be reloaded from their persisted records at playground load, and once the halt clears they SHALL be placed and committed with their retained fill parameters, with the persisted records deleted on commit

#### Scenario: Stale persisted deferral is not replayed

- **WHEN** a persisted deferral's source order no longer has remaining open quantity at reload
- **THEN** the deferral SHALL be discarded as stale and SHALL NOT be placed again

#### Scenario: Deferred-auto-close Alert is sticky until acknowledged

- **WHEN** the deferred closes commit (the outstanding set becomes empty) before the operator has acknowledged the Alert
- **THEN** the Alert SHALL NOT auto-resolve — it SHALL keep notifying until acknowledged, and SHALL resolve only once acknowledged with the deferral set empty

#### Scenario: Auto-closes unaffected while clear

- **WHEN** the halt is clear and an assignment or expiration event requires an auto-close
- **THEN** the auto-close SHALL be placed and committed within the same Tick exactly as today
