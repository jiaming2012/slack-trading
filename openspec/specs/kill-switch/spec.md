# kill-switch Specification

## Purpose
TBD - created by archiving change kill-switch-and-broker-side-stops. Update Purpose after archive.
## Requirements
### Requirement: Central halt gate at the Broker seam

The system SHALL provide a halt controller consulted before any order reaches the Broker seam through which every order in every Mode is placed. When the halt is engaged, the controller SHALL cause the order-submission path to reject the order with a distinct halt error and SHALL NOT forward it to any Broker, in Simulation, Paper, or Margin Mode.

#### Scenario: Order rejected while halt is engaged

- **WHEN** the halt controller is in the engaged state and an order-submission request arrives at the Broker seam
- **THEN** the order SHALL be rejected with a distinct halt error and SHALL NOT be forwarded to any Broker

#### Scenario: Order forwarded while halt is clear

- **WHEN** the halt controller is in the clear (not engaged) state and an order-submission request arrives at the Broker seam
- **THEN** the order SHALL be forwarded to the Broker exactly as it is today, with no halt error

#### Scenario: Halt applies across every Mode

- **WHEN** the halt controller is engaged and order-submission requests arrive for Simulation, Paper, and Margin Playgrounds
- **THEN** every one of those requests SHALL be rejected with the halt error regardless of Mode

### Requirement: Forced Simulation liquidations are exempt from the halt

Forced maintenance-margin liquidations in Simulation Mode SHALL remain exempt from the halt: they are risk-*reducing* closes that commit directly through the Simulated fill engine (`CommitOrderQueue`) and never cross the halt-gated Broker seam (`Playground.PlaceOrder`) that new, risk-*increasing* order submission flows through. This is deliberate: a kill switch exists to stop the system from opening or adding to exposure, not to strand a Simulation account above its maintenance-margin limit by blocking the very closes that de-risk it.

#### Scenario: Simulation liquidation proceeds while the halt is engaged

- **WHEN** the halt controller is engaged and a Simulation Playground breaches its maintenance margin, triggering a forced liquidation
- **THEN** the liquidating (position-closing) orders SHALL still be committed through the Simulated fill engine, because they are risk-reducing closes that do not cross the halt-gated Broker seam

### Requirement: Manual engage and release via REST

The system SHALL expose REST endpoints to engage the kill switch, release it, and read its status. Engaging SHALL move the controller to the engaged state and record the source as manual; releasing SHALL move it to the clear state; the status endpoint SHALL report whether the halt is engaged, the recorded reason and source, and whether a cooldown acknowledgment is required.

The mutating endpoints SHALL be guarded by a shared-secret token sourced from
configuration and compared in constant time. The halt-weakening operations
(release and acknowledge) SHALL always require a valid token and SHALL be
refused when no token is configured (fail-closed). Engage SHALL require the
token when one is configured, but SHALL remain available when no token is
configured — stopping trading is the safe direction and must never be blocked by
missing configuration — with the unauthenticated-engage condition logged loudly
at startup. The status endpoint SHALL remain readable without a token. A request
with a missing or invalid token SHALL be rejected with an authentication error
and SHALL NOT change the controller state.

#### Scenario: Operator engages the kill switch

- **WHEN** the engage endpoint receives a valid request (bearing the configured token, when one is configured) while the controller is clear
- **THEN** the controller SHALL transition to engaged with source recorded as manual, and the status endpoint SHALL subsequently report engaged

#### Scenario: Operator reads status

- **WHEN** the status endpoint is queried
- **THEN** the response SHALL report the engaged/clear state, the recorded halt reason and source, and whether a cooldown acknowledgment is currently required, without requiring a token

#### Scenario: Operator releases the kill switch

- **WHEN** the release endpoint receives a request bearing a valid token while the controller is engaged and no cooldown acknowledgment is outstanding
- **THEN** the controller SHALL transition to clear and the status endpoint SHALL subsequently report clear

#### Scenario: Mutating request without a valid token is rejected

- **WHEN** a token is configured and an engage, release, or acknowledge request arrives with a missing or invalid token
- **THEN** the request SHALL be rejected with an authentication error and the controller state SHALL be unchanged

#### Scenario: Release refused when no token is configured

- **WHEN** no kill-switch token is configured and a release or acknowledge request arrives
- **THEN** the request SHALL be refused with an error stating that the token is not configured, and the controller SHALL remain engaged

#### Scenario: Engage still works when no token is configured

- **WHEN** no kill-switch token is configured and an engage request arrives
- **THEN** the controller SHALL transition to engaged with source recorded as manual

### Requirement: Halt state persisted across restarts

The halt controller SHALL persist its state through a halt-state store abstraction with a file-backed default implementation, and SHALL restore that state on startup so that a server that was halted before a restart comes back up still halted. Persistence SHALL NOT require a live production database connection.

The file-backed store's save SHALL be crash-durable: it SHALL flush the newly
written state to stable storage before renaming it into place, and SHALL flush
the containing directory after the rename, so that a crash or power loss
immediately after a successful save cannot roll the persisted halt state back to
its previous value or leave it truncated. A flush failure SHALL surface as a
save error rather than being silently ignored.

#### Scenario: Restart while halted stays halted

- **WHEN** the halt controller is engaged, its state is written to the store, and a new controller instance is constructed from that same store
- **THEN** the new controller SHALL initialize in the engaged state with the previously recorded reason and source

#### Scenario: Restart while clear stays clear

- **WHEN** the halt controller is clear and a new controller instance is constructed from the same store
- **THEN** the new controller SHALL initialize in the clear state

#### Scenario: Missing or empty store defaults to clear

- **WHEN** a controller is constructed from a store that has no persisted state yet
- **THEN** the controller SHALL initialize in the clear state without error

#### Scenario: Save flushes to stable storage before and after the rename

- **WHEN** the file-backed store saves a halt state
- **THEN** the state file's contents SHALL be flushed to stable storage before the rename into place and the containing directory SHALL be flushed after the rename, and any flush failure SHALL be returned as a save error

### Requirement: Taskfile wrapper commands

The kill switch SHALL be operable through Taskfile targets `kill-switch:engage`, `kill-switch:release`, and `kill-switch:status` that call the corresponding REST endpoints, so operators can drive the halt without knowing the underlying HTTP paths, per the project convention that every operator-visible command ships with a matching Taskfile target. The mutating targets SHALL pass the configured shared-secret token from the operator's environment to the endpoints.

#### Scenario: Engage via Task

- **WHEN** an operator runs `task kill-switch:engage`
- **THEN** the target SHALL call the engage REST endpoint, passing the configured token from the environment, and the kill switch SHALL become engaged

#### Scenario: Status via Task

- **WHEN** an operator runs `task kill-switch:status`
- **THEN** the target SHALL call the status REST endpoint and report the current halt state

#### Scenario: Release via Task passes the token

- **WHEN** an operator runs `task kill-switch:release` with the token present in the environment
- **THEN** the target SHALL call the release REST endpoint bearing that token

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
process restart SHALL NOT drop a deferred auto-close. Staleness at reload SHALL be
judged per request kind: a deferred CLOSE request is stale when its source
order no longer has remaining open quantity (the close already committed —
replaying it would reverse the position); a deferred EXERCISE leg (the
exercised stock delivery, carrying no close linkage) is independent of the
source option order's remaining quantity and is stale only when an order
carrying the leg's own exercise tag already exists (the leg itself was
already placed). Stale deferrals SHALL NOT be replayed.

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

- **WHEN** a persisted CLOSE deferral's source order no longer has remaining open quantity at reload
- **THEN** the deferral SHALL be discarded as stale and SHALL NOT be placed again

#### Scenario: Exercise leg survives its option close committing

- **WHEN** a deferred option close commits and fills while its companion exercise stock leg is still deferred, and the process then restarts
- **THEN** the exercise leg SHALL be restored and retried at reload — it SHALL NOT be discarded merely because the source option order has no remaining open quantity — and it SHALL be discarded only when an order carrying its exercise tag already exists

#### Scenario: Deferred-auto-close Alert is sticky until acknowledged

- **WHEN** the deferred closes commit (the outstanding set becomes empty) before the operator has acknowledged the Alert
- **THEN** the Alert SHALL NOT auto-resolve — it SHALL keep notifying until acknowledged, and SHALL resolve only once acknowledged with the deferral set empty

#### Scenario: Auto-closes unaffected while clear

- **WHEN** the halt is clear and an assignment or expiration event requires an auto-close
- **THEN** the auto-close SHALL be placed and committed within the same Tick exactly as today

