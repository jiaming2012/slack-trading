# anomaly-halt-guards — delta for wire-anomaly-guard-feeds

## MODIFIED Requirements

### Requirement: Order-rejection-rate guard

The order-rejection-rate guard SHALL, when fed a rolling sequence of order
outcomes (rejected or accepted), evaluate the rejection rate over its configured
rolling window and SHALL trip the kill switch (engaging the halt with source
recorded as automatic and a reason identifying this guard) when the rejection
rate exceeds the configured threshold within that window. The guard SHALL be fed
from the live order path: every Paper- or Margin-Mode broker order rejection
SHALL be observed as a rejected outcome and every live fill SHALL be observed as
an accepted outcome, at the live order-update pipeline. Simulation-Mode order
outcomes SHALL NOT feed the guard.

#### Scenario: Rejection rate over threshold trips the halt

- **WHEN** the guard observes order outcomes whose rejected fraction within the rolling window rises above the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the rejection-rate guard

#### Scenario: Rejection rate under threshold does not trip

- **WHEN** the guard observes order outcomes whose rejected fraction within the rolling window stays at or below the configured threshold
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: Live broker rejection is observed by the guard

- **WHEN** the live order-update pipeline applies a broker rejection to a Paper or Margin order
- **THEN** the rejection-rate guard SHALL receive a rejected observation from that same pipeline event

#### Scenario: Simulation outcomes do not feed the guard

- **WHEN** a Simulation-Mode order is filled or invalidated by the Simulated Broker
- **THEN** the rejection-rate guard SHALL receive no observation

### Requirement: Fill-price-deviation guard

The fill-price-deviation guard SHALL, when handed a fill's actual price and the
order's expected price, compare them and SHALL trip the kill switch when the
absolute deviation exceeds a configured percentage of the expected price. The
guard SHALL be fed from live fills: each Paper- or Margin-Mode fill committed by
the live order-update pipeline SHALL be observed with the order's requested
price as the expected price and the trade's price as the actual price. Fills
whose orders carry no positive requested price SHALL be skipped rather than
evaluated against a meaningless expectation.

#### Scenario: Deviation beyond threshold trips the halt

- **WHEN** the guard observes a fill whose absolute price deviation from the expected price exceeds the configured percentage
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the fill-deviation guard

#### Scenario: Deviation within tolerance does not trip

- **WHEN** the guard observes a fill whose absolute price deviation from the expected price is within the configured percentage
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: Live fill is observed by the guard

- **WHEN** the live order-update pipeline commits a Paper or Margin fill whose order has a positive requested price
- **THEN** the fill-deviation guard SHALL receive that fill's requested price and actual trade price as one observation

### Requirement: Trades-per-hour sigma guard

The trades-per-hour sigma guard SHALL, when handed a stream of executed-trade
observations, compute the current trades-per-hour rate and SHALL trip the kill
switch when that rate exceeds the historical mean by more than two standard
deviations of the historical trades-per-hour norm supplied to the guard. The
guard SHALL be fed from the live trade stream: each new Paper- or Margin-Mode
trade committed by the live order-update pipeline SHALL be observed; Simulation
trades SHALL NOT feed the guard.

The guard SHALL NOT trip before a configurable minimum number of trades has been
observed within its rolling window, and when the supplied historical norm is
degenerate (mean and standard deviation both zero — no σ history), the guard
SHALL remain unarmed: it SHALL observe without ever tripping and SHALL log the
unarmed condition at startup.

#### Scenario: Rate beyond two sigma trips the halt

- **WHEN** the guard has at least the configured minimum samples in its window and the observed trades push the current trades-per-hour rate above the historical mean by more than two standard deviations
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the trades-per-hour guard

#### Scenario: Rate within two sigma does not trip

- **WHEN** the guard's observed trades keep the current trades-per-hour rate within two standard deviations of the historical mean
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: First trade with zero sigma history does not trip

- **WHEN** the guard was constructed with a degenerate historical norm (mean and standard deviation both zero) and observes its first trade
- **THEN** the guard SHALL NOT engage the halt, regardless of how many trades follow

#### Scenario: Below minimum sample count does not trip

- **WHEN** the guard has a valid historical norm but fewer than the configured minimum trades in its rolling window, and the current rate exceeds mean plus two standard deviations
- **THEN** the guard SHALL NOT engage the halt until the minimum sample count is reached

### Requirement: Data-feed-staleness guard with graceful degradation

The data-feed-staleness guard SHALL trip the kill switch when the age of the
most recent Tick from the Feed exceeds a configured staleness threshold,
consuming the last-Tick-age signal provided by the `feed-health-staleness`
capability. That signal source SHALL be wired: a feed-health heartbeat monitor
SHALL be constructed at server startup and SHALL observe a heartbeat (at
wall-clock receipt time, per asset class) whenever the live candle-ingestion
path appends new bars for a realtime Playground. When the signal source is not
present, the staleness guard SHALL degrade gracefully to an inactive state — it
SHALL NOT trip and SHALL NOT block startup or other guards.

The staleness guard SHALL be evaluated periodically on a ticker, and evaluation
SHALL be suppressed while the market is closed or while no realtime Playground
is loaded, so that a quiet feed outside trading hours can never engage the halt.
A failure of the market-calendar check itself SHALL skip that evaluation cycle
with a warning rather than tripping the guard.

#### Scenario: Stale feed trips the halt

- **WHEN** the market is open, a realtime Playground is loaded, and the wired staleness signal reports the last Tick age exceeding the configured threshold
- **THEN** the guard SHALL engage the halt with source automatic and a reason naming the feed-staleness guard

#### Scenario: Fresh feed does not trip

- **WHEN** a staleness signal source is wired and reports the last Tick age within the configured threshold
- **THEN** the guard SHALL NOT engage the halt

#### Scenario: Absent staleness signal degrades gracefully

- **WHEN** no staleness signal source is registered with the guard
- **THEN** the staleness guard SHALL remain inactive, SHALL NOT engage the halt, and SHALL NOT prevent the other guards from operating

#### Scenario: Closed market never trips the staleness guard

- **WHEN** the market is closed and no Ticks have arrived for longer than the staleness threshold
- **THEN** the periodic evaluation SHALL be suppressed and the guard SHALL NOT engage the halt

#### Scenario: Live candle ingestion feeds the heartbeat

- **WHEN** the live candle-ingestion path appends new bars for a realtime Playground's repository
- **THEN** the feed-health heartbeat monitor SHALL record an observation for that repository's asset class at wall-clock receipt time

### Requirement: Guard evaluation feeds the shared halt controller

Every guard SHALL engage the same central halt controller used by the manual kill
switch, so that a trip from any guard rejects all subsequent order submission
identically to a manual halt and is reported through the same status surface. The
guard registry (`NewGuardRegistry`) SHALL bind all guards to that one shared
controller. The registry SHALL be constructed at server startup, bound to the
same halt controller instance that the manual kill switch REST surface and the
Broker-seam order gate use, with guard thresholds sourced from environment
configuration carrying documented conservative defaults; the effective guard
configuration SHALL be logged at startup, an individual guard SHALL be
disableable via configuration (logged loudly), and an unparseable guard
configuration SHALL refuse startup rather than arm a misconfigured guard.

#### Scenario: Auto-halt blocks subsequent orders

- **WHEN** any guard bound to the shared controller trips and engages the halt
- **THEN** subsequent order-submission requests at the Broker seam SHALL be rejected with the halt error exactly as under a manual halt, and the status surface SHALL report the halt as engaged with source automatic

#### Scenario: Registry constructed at startup against the shared controller

- **WHEN** the server starts
- **THEN** the guard registry SHALL be constructed against the same halt controller the REST surface and order gate use, and a trip from any wired guard SHALL be visible through `GET /kill-switch/status` as an engaged automatic halt

#### Scenario: Disabled guard never observes or trips

- **WHEN** a guard is disabled via its configuration sentinel
- **THEN** that guard SHALL NOT trip regardless of observations, the disablement SHALL be logged at startup, and the remaining guards SHALL operate normally

#### Scenario: Invalid guard configuration refuses startup

- **WHEN** a guard threshold environment variable is set to an unparseable value
- **THEN** the server SHALL refuse to start with an error naming the variable

## ADDED Requirements

### Requirement: Guard activity is observable through Telemetry

Guard activity SHALL be recorded in the internal Telemetry registry: a counter of
observations per guard, a counter of trips per guard, and a gauge reflecting
whether the halt is currently engaged (updated on every halt-controller
transition and on startup state restore). When a guard trips, an Alert SHALL be
pushed to the operator identifying the guard and the trip reason. Telemetry
SHALL use only the internal telemetry module; no OpenTelemetry API or exporter
SHALL be introduced.

#### Scenario: Guard trip increments the trip counter and pushes an Alert

- **WHEN** any guard trips and engages the halt
- **THEN** the per-guard trip counter SHALL be incremented, the halt-engaged gauge SHALL report engaged, and an Alert naming the guard and reason SHALL be pushed to the operator

#### Scenario: Halt release updates the gauge

- **WHEN** the halt is released after acknowledgment
- **THEN** the halt-engaged gauge SHALL report clear
