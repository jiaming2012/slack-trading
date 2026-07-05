## ADDED Requirements

> **Amendment note (2026-07-05, review-driven — see the `scanner-l1-l2` precedent).**
> Adversarial review found that this change delivers companion-stop placement as a
> tested library — `PlaceCompanionStop` exercised against `MockBroker` — but does
> NOT invoke it from the live fill pipeline: `PlaceCompanionStop` has no
> production caller (nothing in `DrainTradierOrderQueue` or the live fill path
> calls it). The original wording ("When a live-Mode entry order fills, the system
> SHALL place…") over-claimed a live end-to-end behavior. The requirements and
> scenarios below are amended in place to describe the **library-level** behavior
> actually delivered: given a live entry fill, the placement routine constructs
> and submits the correct companion stop through the Broker seam. Wiring the live
> fill pipeline (`DrainTradierOrderQueue`) to call `PlaceCompanionStop` on each
> real fill is explicitly DEFERRED to a named follow-up change,
> **`wire-companion-stops`** (it may be folded into `wire-anomaly-guard-feeds` if
> the feeds land together). The wiring is deferred because invoking it overnight
> would place real, unverifiable stop orders at a live/Paper broker, which the
> overnight run prohibits.

### Requirement: Companion broker-held stop placed on live entry fill

The companion-stop placement routine SHALL, given a live-Mode (Paper or Margin) entry fill, construct and submit a companion stop order at the Broker seam at a configured stop distance from the fill price, so the exit is held by the Broker and can execute even if our own infrastructure is completely down. The stop order SHALL use the Broker's stop order type and SHALL be sized to close the filled quantity. Invoking this routine from the live fill pipeline is deferred to `wire-companion-stops`.

#### Scenario: Stop placed after a live long entry fills

- **WHEN** the placement routine is given a Paper or Margin entry fill that opened a long position
- **THEN** it SHALL place a companion stop order at the Broker for the filled quantity at the configured stop distance below the fill price

#### Scenario: Stop placed after a live short entry fills

- **WHEN** the placement routine is given a Paper or Margin entry fill that opened a short position
- **THEN** it SHALL place a companion stop order at the Broker for the filled quantity at the configured stop distance above the fill price

### Requirement: No broker-side stop in Simulation Mode

Simulation-Mode fills SHALL NOT trigger a companion broker-held stop order, because Simulation exits are handled by the Simulated Broker's fill engine and there is no external broker holding the position.

#### Scenario: Simulation fill places no companion stop

- **WHEN** the placement routine is given a Simulation-Mode entry fill
- **THEN** it SHALL NOT place a companion broker-held stop order

### Requirement: Stop-distance configuration

The stop distance SHALL be configurable and SHALL be validated as a positive value; an invalid or missing stop-distance configuration SHALL cause the companion-stop placement to error rather than place a stop at or through the fill price.

#### Scenario: Positive stop distance accepted

- **WHEN** the stop distance is configured to a positive value and the placement routine is given a live entry fill
- **THEN** the companion stop SHALL be placed at that distance from the fill price on the protective side of the position

#### Scenario: Non-positive stop distance rejected

- **WHEN** the stop distance is configured to zero or a negative value and the placement routine is given a live entry fill
- **THEN** the companion-stop placement SHALL return an error and SHALL NOT place a stop at or through the fill price

### Requirement: Implemented and unit-tested against MockBroker

The companion-stop placement SHALL be implemented and unit-tested against the existing MockBroker, which SHALL record the stop orders it receives so tests can assert the presence, side, quantity, and stop price of the companion stop without contacting any real broker. Verification against the Tradier sandbox is out of scope for this change.

#### Scenario: MockBroker records the companion stop

- **WHEN** the placement routine is given a live entry fill with the Broker seam bound to MockBroker
- **THEN** MockBroker SHALL record a stop order whose side, quantity, and stop price match the configured stop distance and the filled position, and a unit test SHALL be able to assert those values
