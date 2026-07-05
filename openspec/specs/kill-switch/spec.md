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

#### Scenario: Operator engages the kill switch

- **WHEN** the engage endpoint receives a valid request while the controller is clear
- **THEN** the controller SHALL transition to engaged with source recorded as manual, and the status endpoint SHALL subsequently report engaged

#### Scenario: Operator reads status

- **WHEN** the status endpoint is queried
- **THEN** the response SHALL report the engaged/clear state, the recorded halt reason and source, and whether a cooldown acknowledgment is currently required

#### Scenario: Operator releases the kill switch

- **WHEN** the release endpoint receives a valid request while the controller is engaged and no cooldown acknowledgment is outstanding
- **THEN** the controller SHALL transition to clear and the status endpoint SHALL subsequently report clear

### Requirement: Halt state persisted across restarts

The halt controller SHALL persist its state through a halt-state store abstraction with a file-backed default implementation, and SHALL restore that state on startup so that a server that was halted before a restart comes back up still halted. Persistence SHALL NOT require a live production database connection.

#### Scenario: Restart while halted stays halted

- **WHEN** the halt controller is engaged, its state is written to the store, and a new controller instance is constructed from that same store
- **THEN** the new controller SHALL initialize in the engaged state with the previously recorded reason and source

#### Scenario: Restart while clear stays clear

- **WHEN** the halt controller is clear and a new controller instance is constructed from the same store
- **THEN** the new controller SHALL initialize in the clear state

#### Scenario: Missing or empty store defaults to clear

- **WHEN** a controller is constructed from a store that has no persisted state yet
- **THEN** the controller SHALL initialize in the clear state without error

### Requirement: Taskfile wrapper commands

The kill switch SHALL be operable through Taskfile targets `kill-switch:engage`, `kill-switch:release`, and `kill-switch:status` that call the corresponding REST endpoints, so operators can drive the halt without knowing the underlying HTTP paths, per the project convention that every operator-visible command ships with a matching Taskfile target.

#### Scenario: Engage via Task

- **WHEN** an operator runs `task kill-switch:engage`
- **THEN** the target SHALL call the engage REST endpoint and the kill switch SHALL become engaged

#### Scenario: Status via Task

- **WHEN** an operator runs `task kill-switch:status`
- **THEN** the target SHALL call the status REST endpoint and report the current halt state

