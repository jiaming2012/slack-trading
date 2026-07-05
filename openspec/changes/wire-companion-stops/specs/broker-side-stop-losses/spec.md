# broker-side-stop-losses — delta for wire-companion-stops

## MODIFIED Requirements

### Requirement: Companion broker-held stop placed on live entry fill

The companion-stop placement routine SHALL, given a live-Mode (Paper or Margin) entry fill, construct and submit a companion stop order at the Broker seam at a configured stop distance from the fill price, so the exit is held by the Broker and can execute even if our own infrastructure is completely down. The stop order SHALL use the Broker's stop order type and SHALL be sized to close the filled quantity.

The routine SHALL be invoked from the live fill pipeline: when the live
order-update pipeline commits a Paper- or Margin-Mode fill, an eligible fill
SHALL trigger companion-stop placement after the fill has been committed and
saved. Eligible fills are equity entry fills that open or increase a position;
closes, adjustment orders, system auto-closes, reconciliation-role fills, and
fills of companion-stop orders themselves SHALL NOT trigger placement (a
companion stop must never spawn another companion stop). Placement SHALL be
idempotent per entry order: a redelivered fill event for an entry that already
has a companion stop SHALL NOT place a second one. A placement failure SHALL NOT
fail or roll back the fill itself.

#### Scenario: Stop placed after a live long entry fills

- **WHEN** the live fill pipeline commits a Paper or Margin entry fill that opened a long position
- **THEN** it SHALL place a companion stop order at the Broker for the filled quantity at the configured stop distance below the fill price

#### Scenario: Stop placed after a live short entry fills

- **WHEN** the live fill pipeline commits a Paper or Margin entry fill that opened a short position
- **THEN** it SHALL place a companion stop order at the Broker for the filled quantity at the configured stop distance above the fill price

#### Scenario: Close fills place no companion stop

- **WHEN** the live fill pipeline commits a fill for a closing order, an adjustment order, or a system auto-close
- **THEN** it SHALL NOT place a companion stop order

#### Scenario: A companion stop's own fill places no further stop

- **WHEN** the live fill pipeline commits the fill of a companion-stop order
- **THEN** it SHALL NOT place another companion stop order

#### Scenario: Redelivered fill event places no duplicate stop

- **WHEN** a fill event for an entry order that already has a companion stop is delivered again
- **THEN** the pipeline SHALL NOT place a second companion stop for that entry order

### Requirement: Stop-distance configuration

The stop distance SHALL be configurable and SHALL be validated as a positive value; an invalid or missing stop-distance configuration SHALL cause the companion-stop placement to error rather than place a stop at or through the fill price.

Companion-stop placement SHALL be explicit opt-in at the deployment level: when
no stop distance is configured, the live-fill invocation SHALL be disabled
entirely and the disabled state SHALL be logged loudly at startup; when a
configured distance is non-positive, the server SHALL refuse to start rather
than error on every fill.

#### Scenario: Positive stop distance accepted

- **WHEN** the stop distance is configured to a positive value and the placement routine is given a live entry fill
- **THEN** the companion stop SHALL be placed at that distance from the fill price on the protective side of the position

#### Scenario: Non-positive stop distance rejected

- **WHEN** the stop distance is configured to zero or a negative value
- **THEN** the server SHALL refuse to start, and the placement routine invoked directly with a non-positive distance SHALL return an error and SHALL NOT place a stop at or through the fill price

#### Scenario: Unconfigured distance disables the feature loudly

- **WHEN** no stop distance is configured and the server starts
- **THEN** companion-stop placement SHALL be disabled, a loud startup warning SHALL be logged, and live entry fills SHALL commit normally without stops

### Requirement: Implemented and unit-tested against MockBroker

The companion-stop placement SHALL be implemented and tested against the existing MockBroker, which SHALL record the stop orders it receives so tests can assert the presence, side, quantity, and stop price of the companion stop without contacting any real broker. The wired live-fill invocation SHALL be verified end-to-end with the Broker seam bound to MockBroker: driving the live fill pipeline with synthetic fill events SHALL be sufficient to assert placement, eligibility exclusions, idempotency, and halt bypass. Verification against a real broker (Tradier sandbox Paper account, then Margin) SHALL NOT be performed by any autonomous run and SHALL be an explicit operator-only step.

#### Scenario: MockBroker records the companion stop

- **WHEN** the placement routine is given a live entry fill with the Broker seam bound to MockBroker
- **THEN** MockBroker SHALL record a stop order whose side, quantity, and stop price match the configured stop distance and the filled position, and a unit test SHALL be able to assert those values

#### Scenario: Pipeline verified end-to-end against MockBroker

- **WHEN** the live fill pipeline is driven with a synthetic Paper-Mode entry-fill event and the Broker seam is bound to MockBroker
- **THEN** the companion stop SHALL be recorded by MockBroker without any real broker being contacted

#### Scenario: No autonomous live verification

- **WHEN** the change's verification gates are executed autonomously
- **THEN** no order SHALL be placed against a live or Paper broker; real-broker verification SHALL remain an operator-only task

## ADDED Requirements

### Requirement: Companion stops bypass an engaged halt

Protective companion stops are risk-reducing: they attach a broker-held exit to a position the system already holds. Companion-stop placement SHALL bypass an engaged halt — the kill switch must never strand an open position without its protective exit. The placement SHALL reach the Broker through the Broker seam directly, below the halt-gated order-submission path, so the bypass holds by construction rather than by an exemption in the halt controller.

#### Scenario: Companion stop placed while the halt is engaged

- **WHEN** the halt controller is engaged and the live fill pipeline commits an eligible Paper or Margin entry fill
- **THEN** the companion stop SHALL still be placed at the Broker, while ordinary new-order submission remains rejected by the halt

#### Scenario: Halt controller carries no companion-stop exemption

- **WHEN** the halt controller's order gate is consulted by the ordinary submission path
- **THEN** it SHALL reject uniformly while engaged; the companion-stop bypass SHALL exist only as routing below the gated seam, not as a conditional inside the gate

### Requirement: Companion-stop placement failure raises an Alert

A companion-stop placement failure leaves a live position without its protective exit and SHALL be loud: the failure SHALL be logged with the entry-order context, recorded in the internal Telemetry registry (counters for placed and failed companion stops), and pushed to the operator as an Alert identifying the unprotected symbol and quantity. The fill that triggered the placement SHALL remain committed. Telemetry SHALL use only the internal telemetry module; no OpenTelemetry API or exporter SHALL be introduced.

#### Scenario: Placement failure alerts the operator

- **WHEN** companion-stop placement returns an error for a committed live entry fill
- **THEN** an Alert SHALL be pushed naming the symbol and quantity left unprotected, the failure counter SHALL be incremented, and the fill SHALL remain committed

#### Scenario: Successful placement is counted

- **WHEN** a companion stop is placed successfully
- **THEN** the placed counter in the internal Telemetry registry SHALL be incremented
