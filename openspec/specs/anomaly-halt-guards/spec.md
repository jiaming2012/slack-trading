# anomaly-halt-guards Specification

## Purpose
TBD - created by archiving change kill-switch-and-broker-side-stops. Update Purpose after archive.
## Requirements
### Requirement: Order-rejection-rate guard

The order-rejection-rate guard SHALL, when fed a rolling sequence of order
outcomes (rejected or accepted), evaluate the rejection rate over its configured
rolling window and SHALL trip the kill switch (engaging the halt with source
recorded as automatic and a reason identifying this guard) when the rejection
rate exceeds the configured threshold within that window. Feeding the guard from
the live order path is deferred to `wire-anomaly-guard-feeds`.

#### Scenario: Rejection rate over threshold trips the halt

- **WHEN** the guard observes order outcomes whose rejected fraction within the rolling window rises above the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the rejection-rate guard

#### Scenario: Rejection rate under threshold does not trip

- **WHEN** the guard observes order outcomes whose rejected fraction within the rolling window stays at or below the configured threshold
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Fill-price-deviation guard

The fill-price-deviation guard SHALL, when handed a fill's actual price and the
order's expected price, compare them and SHALL trip the kill switch when the
absolute deviation exceeds a configured percentage of the expected price.
Feeding the guard from live fills is deferred to `wire-anomaly-guard-feeds`.

#### Scenario: Deviation beyond threshold trips the halt

- **WHEN** the guard observes a fill whose absolute price deviation from the expected price exceeds the configured percentage
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the fill-deviation guard

#### Scenario: Deviation within tolerance does not trip

- **WHEN** the guard observes a fill whose absolute price deviation from the expected price is within the configured percentage
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Trades-per-hour sigma guard

The trades-per-hour sigma guard SHALL, when handed a stream of executed-trade
observations, compute the current trades-per-hour rate and SHALL trip the kill
switch when that rate exceeds the historical mean by more than two standard
deviations of the historical trades-per-hour norm supplied to the guard. Feeding
the guard from the live trade stream is deferred to `wire-anomaly-guard-feeds`.

#### Scenario: Rate beyond two sigma trips the halt

- **WHEN** the guard's observed trades push the current trades-per-hour rate above the historical mean by more than two standard deviations
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the trades-per-hour guard

#### Scenario: Rate within two sigma does not trip

- **WHEN** the guard's observed trades keep the current trades-per-hour rate within two standard deviations of the historical mean
- **THEN** the guard SHALL NOT engage the halt

### Requirement: Data-feed-staleness guard with graceful degradation

The data-feed-staleness guard SHALL, when a staleness signal source is wired into
it, trip the kill switch when the age of the most recent Tick from the Feed
exceeds a configured staleness threshold, consuming the last-Tick-age signal
provided by the `feed-health-staleness` capability. When that signal source is
not present, the staleness guard SHALL degrade gracefully to an inactive state —
it SHALL NOT trip and SHALL NOT block startup or other guards. Wiring the
`feed-health-staleness` signal source into the guard is deferred to
`wire-anomaly-guard-feeds`.

#### Scenario: Stale feed trips the halt

- **WHEN** a staleness signal source is wired and reports the last Tick age exceeding the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the feed-staleness guard

#### Scenario: Fresh feed does not trip

- **WHEN** a staleness signal source is wired and reports the last Tick age within the configured threshold
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: Absent staleness signal degrades gracefully

- **WHEN** no staleness signal source is registered with the guard
- **THEN** the staleness guard SHALL remain inactive, SHALL NOT engage the halt, and SHALL NOT prevent the other guards from operating

### Requirement: Guard evaluation feeds the shared halt controller

Every guard SHALL engage the same central halt controller used by the manual kill
switch, so that a trip from any guard rejects all subsequent order submission
identically to a manual halt and is reported through the same status surface. The
guard registry (`NewGuardRegistry`) SHALL bind all guards to that one shared
controller. Constructing the registry at startup and driving it from live feeds
is deferred to `wire-anomaly-guard-feeds`.

#### Scenario: Auto-halt blocks subsequent orders

- **WHEN** any guard bound to the shared controller trips and engages the halt
- **THEN** subsequent order-submission requests at the Broker seam SHALL be rejected with the halt error exactly as under a manual halt, and the status surface SHALL report the halt as engaged with source automatic

