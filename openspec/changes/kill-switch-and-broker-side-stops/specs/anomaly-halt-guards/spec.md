## ADDED Requirements

### Requirement: Order-rejection-rate guard

The system SHALL evaluate the order-rejection rate over a configurable rolling window and SHALL trip the kill switch (engaging the halt with source recorded as automatic and a reason identifying this guard) when the rejection rate exceeds a configured threshold within that window.

#### Scenario: Rejection rate over threshold trips the halt

- **WHEN** the fraction of rejected orders within the rolling window rises above the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the rejection-rate guard

#### Scenario: Rejection rate under threshold does not trip

- **WHEN** the fraction of rejected orders within the rolling window stays at or below the configured threshold
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Fill-price-deviation guard

The system SHALL compare each fill's actual price against the order's expected price and SHALL trip the kill switch when the absolute deviation exceeds a configured percentage of the expected price.

#### Scenario: Deviation beyond threshold trips the halt

- **WHEN** a fill's absolute price deviation from the expected price exceeds the configured percentage
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the fill-deviation guard

#### Scenario: Deviation within tolerance does not trip

- **WHEN** a fill's absolute price deviation from the expected price is within the configured percentage
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Trades-per-hour sigma guard

The system SHALL compute the current trades-per-hour rate and SHALL trip the kill switch when that rate exceeds the historical mean by more than two standard deviations of the historical trades-per-hour norm supplied to the guard.

#### Scenario: Rate beyond two sigma trips the halt

- **WHEN** the current trades-per-hour rate exceeds the historical mean by more than two standard deviations
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the trades-per-hour guard

#### Scenario: Rate within two sigma does not trip

- **WHEN** the current trades-per-hour rate is within two standard deviations of the historical mean
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Data-feed-staleness guard with graceful degradation

The system SHALL trip the kill switch when the age of the most recent Tick from the Feed exceeds a configured staleness threshold, consuming the staleness signal provided by the `feed-health-staleness` capability. When that signal source is not present, the staleness guard SHALL degrade gracefully to an inactive state — it SHALL NOT trip and SHALL NOT block startup or other guards.

#### Scenario: Stale feed trips the halt

- **WHEN** the staleness signal reports the last Tick age exceeding the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the feed-staleness guard

#### Scenario: Fresh feed does not trip

- **WHEN** the staleness signal reports the last Tick age within the configured threshold
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: Absent staleness signal degrades gracefully

- **WHEN** no staleness signal source is registered with the guard
- **THEN** the staleness guard SHALL remain inactive, SHALL NOT engage the halt, and SHALL NOT prevent the other guards from operating

### Requirement: Guard evaluation feeds the shared halt controller

Every guard SHALL engage the same central halt controller used by the manual kill switch, so an auto-halt from any guard rejects all subsequent order submission identically to a manual halt and is reported through the same status surface.

#### Scenario: Auto-halt blocks subsequent orders

- **WHEN** any guard trips and engages the halt
- **THEN** subsequent order-submission requests at the Broker seam SHALL be rejected with the halt error exactly as under a manual halt, and the status surface SHALL report the halt as engaged with source automatic
