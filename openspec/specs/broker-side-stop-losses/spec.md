# broker-side-stop-losses Specification

## Purpose
TBD - created by archiving change kill-switch-and-broker-side-stops. Update Purpose after archive.
## Requirements
### Requirement: Companion broker-held stop placed on live entry fill

The companion-stop placement routine SHALL, given a live-Mode (Paper or Margin) entry fill, construct and submit a companion stop order at the Broker seam at a configured stop distance from the fill price, so the exit is held by the Broker and can execute even if our own infrastructure is completely down. The stop order SHALL use the Broker's stop order type and SHALL be sized to close the filled quantity.

The routine SHALL be invoked from the live fill pipeline: when the live
order-update pipeline commits a Paper- or Margin-Mode fill, an eligible fill
SHALL trigger companion-stop placement after the fill has been committed and
saved. Eligible fills are equity entry fills that open or increase a position;
closes, adjustment orders, system auto-closes, reconciliation-role fills, and
fills of companion-stop orders themselves SHALL NOT trigger placement (a
companion stop must never spawn another companion stop). The reserved
companion-stop tag prefix SHALL be rejected for client-supplied orders at
order-request validation — inherited by every placement ingress, single-leg
and multi-leg alike — and additionally at the PlaceOrder RPC boundary with an
invalid-argument error, so a client tag can never poison the recursion guard
or the idempotency lookup.

Placement SHALL be CUMULATIVE per entry order: one strategy order can net into
multiple broker trades, each committing through the pipeline separately, and
each committed trade SHALL top up the protective coverage by the
still-unprotected delta between the entry's total filled quantity and the
quantity already covered by placed stops — so the total stop quantity tracks
the total FILLED quantity, never the requested total (an over-sized stop
reverses instead of flattening). A redelivered fill event carries no new
filled quantity and SHALL NOT place a duplicate stop. An unprotected remainder
smaller than one whole share SHALL be skipped with a warning (not the failure
alert) and SHALL remain tracked so later fills accumulate into whole-share
protection. A placement failure SHALL NOT fail or roll back the fill itself.

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
- **THEN** the pipeline SHALL NOT place a second companion stop for that entry order, because the redelivered event carries no new filled quantity

#### Scenario: Multi-trade entry fill is protected in full

- **WHEN** one entry order nets into multiple broker trades (for example buy 10 filling as buy_to_cover 4 plus buy 6) and each trade commits through the live fill pipeline
- **THEN** each committed trade SHALL top up the companion-stop coverage by its still-unprotected delta, and the total stop quantity SHALL equal the entry's total filled quantity

#### Scenario: Top-up placement failure alerts for the unprotected delta

- **WHEN** a later trade's top-up stop placement fails after earlier coverage was placed
- **THEN** the failure SHALL be recorded and alerted for the unprotected DELTA quantity, and the previously placed coverage SHALL remain tracked so a retry sizes only the missing remainder

#### Scenario: Client-supplied reserved tag rejected at the API boundary

- **WHEN** a client submits an order — single-leg or any leg of a multi-leg order — whose tag carries the reserved companion-stop prefix
- **THEN** the order placement SHALL be rejected with an invalid-argument error before reaching any Broker

#### Scenario: Fractional remainder below one share skips without alerting

- **WHEN** an entry's unprotected remainder is smaller than one whole share
- **THEN** placement SHALL be skipped with a warning, no failure SHALL be recorded, and the fraction SHALL remain tracked so accumulation across fills still reaches whole-share protection

### Requirement: No broker-side stop in Simulation Mode

Simulation-Mode fills SHALL NOT trigger a companion broker-held stop order, because Simulation exits are handled by the Simulated Broker's fill engine and there is no external broker holding the position.

#### Scenario: Simulation fill places no companion stop

- **WHEN** the placement routine is given a Simulation-Mode entry fill
- **THEN** it SHALL NOT place a companion broker-held stop order

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

