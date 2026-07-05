## ADDED Requirements

### Requirement: In-process metric registry with synchronous recording
The `src/go/telemetry` package SHALL provide an in-process metric registry with counter (monotonic) and gauge (last-value) instruments supporting string labels. Recording SHALL be a synchronous function call completing before return — no batching, background export, or external pipeline. The package SHALL NOT import any `go.opentelemetry.io/*` package.

#### Scenario: Counter increments synchronously
- **WHEN** a call site increments a counter instrument with a set of labels
- **THEN** an immediate read of the registry reflects the new value for that (name, label-set) series

#### Scenario: Gauge records last value per label set
- **WHEN** a gauge instrument records values for two distinct label sets
- **THEN** the registry holds the most recent value independently for each label set

#### Scenario: No OTel imports remain in the module
- **WHEN** the import graph of `src/go/telemetry` is inspected after the change
- **THEN** it contains no `go.opentelemetry.io/*` or `github.com/uptrace/opentelemetry-go-extra/*` packages

### Requirement: Existing instrument set preserved at the package seam
The registry SHALL expose the existing instrument set — orders placed, orders filled, orders rejected, candles processed, signals generated, signals consumed, active playgrounds, open orders, and uptime seconds — under their existing exported names, and all in-repo call sites SHALL record through them without OTel types in their signatures.

#### Scenario: Order placement is counted
- **WHEN** the server fills an order through the existing call sites
- **THEN** the orders-placed and orders-filled series in the registry increase accordingly

#### Scenario: Server builds with rewired call sites
- **WHEN** `go build ./...` runs after the swap
- **THEN** the build succeeds with no call site importing OTel metric types

### Requirement: Every series is tagged by mode
Metric series recorded for a playground context SHALL carry a mode label (per the CONTEXT.md glossary: telemetry is emitted in every mode, tagged by mode).

#### Scenario: Simulation and live series are distinguishable
- **WHEN** orders are recorded for playgrounds in different modes
- **THEN** the registry holds separate series distinguished by the mode label

### Requirement: Registry snapshot read-out
The registry SHALL provide a snapshot operation returning all current series (name, labels, kind, value) as plain data, usable by the persistence writer and future query surfaces without exposing registry internals.

#### Scenario: Snapshot reflects recorded state
- **WHEN** instruments have been recorded and a snapshot is taken
- **THEN** the snapshot contains one entry per live (name, label-set) series with its current value

### Requirement: Registry initialization is self-contained
`telemetry.Init()` SHALL construct the registry without requiring any prior SDK, provider, or network setup, and recording before persistence has started SHALL be safe.

#### Scenario: Recording before the writer starts
- **WHEN** an instrument records a value after `Init()` but before the snapshot writer goroutine starts
- **THEN** the value is captured in the registry and included in the first snapshot
