## ADDED Requirements

### Requirement: Auto-halt requires manual acknowledgment before resuming

After any automatic halt (a guard trip), the system SHALL enter a cooldown state that requires an explicit manual acknowledgment before order submission can resume. While a cooldown acknowledgment is outstanding, a release request SHALL be rejected and the controller SHALL remain engaged.

#### Scenario: Release rejected while acknowledgment outstanding

- **WHEN** a guard has auto-halted the controller and the release endpoint is called without a prior acknowledgment
- **THEN** the release SHALL be rejected and the controller SHALL remain engaged

#### Scenario: Acknowledge then release resumes submission

- **WHEN** an operator submits the cooldown acknowledgment and then calls release after an auto-halt
- **THEN** the controller SHALL transition to clear and subsequent order submission SHALL be forwarded to the Broker again

### Requirement: Cooldown never self-clears

While halted, the system SHALL NOT automatically return to the clear state even if the condition that triggered the auto-halt subsequently returns to normal. The transition back to clear SHALL happen only through explicit acknowledgment plus release.

#### Scenario: Anomaly subsides but halt persists

- **WHEN** a guard auto-halts, and then the underlying metric returns within its threshold on the next evaluation
- **THEN** the controller SHALL remain engaged and SHALL require acknowledgment plus release to clear

### Requirement: Cooldown status is observable

The status surface SHALL report whether a cooldown acknowledgment is currently required, so an operator can distinguish an auto-halt awaiting acknowledgment from a manual halt that can be released directly.

#### Scenario: Status reports acknowledgment required after auto-halt

- **WHEN** a guard has auto-halted the controller and the status surface is queried
- **THEN** it SHALL report that a cooldown acknowledgment is required

#### Scenario: Status reports no acknowledgment required after manual halt

- **WHEN** the controller was engaged manually (not by a guard) and the status surface is queried
- **THEN** it SHALL report that no cooldown acknowledgment is required
