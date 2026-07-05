# kill-switch — delta for wire-anomaly-guard-feeds

## MODIFIED Requirements

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
