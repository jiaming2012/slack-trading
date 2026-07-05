# simulation-risk-gate

## ADDED Requirements

### Requirement: Limits loaded from the options-config YAML pattern with defaults

The system SHALL load all risk limits from a `riskOverlay` configuration block following the existing options-config YAML conventions, producing a `RiskLimits` value with fields `MaxGrossExposure`, `MaxNetExposure`, `MaxSectorConcentrationPct`, `MaxDrawdownPct`, `DeployableCapital`, and `RejectCrowdedEntries`. When the config block or an individual field is absent, the system SHALL apply documented defaults and SHALL NOT fail to start. The loader SHALL reject a config in which any numeric limit is negative or in which a percentage limit is outside `0–100`, returning a sentinel error.

#### Scenario: Missing config block yields documented defaults

- **WHEN** the config source has no `riskOverlay` block
- **THEN** the loader returns a `RiskLimits` populated with the documented default values and no error

#### Scenario: An invalid limit is rejected

- **WHEN** the config specifies a negative `MaxGrossExposure` or a `MaxSectorConcentrationPct` greater than 100
- **THEN** the loader returns a sentinel validation error and no `RiskLimits` value is used

### Requirement: Gate invoked on the Simulation order path only

The system SHALL invoke the risk gate on the Simulation-mode order-placement path and SHALL NOT invoke it on the Paper or Margin order-placement paths. When the gate is invoked and returns a rejecting `Decision`, the proposed order SHALL NOT be submitted to the Broker seam and the rejection reason SHALL be surfaced to the caller. When the gate returns an allowing `Decision`, order placement SHALL proceed exactly as it does today.

#### Scenario: A rejected order in Simulation is not placed

- **WHEN** a Simulation-mode order is proposed and the gate returns `Decision.Allowed` false
- **THEN** the order is not committed to the order queue and the rejection reason (the breach list) is returned to the caller

#### Scenario: An allowed order in Simulation is placed unchanged

- **WHEN** a Simulation-mode order is proposed and the gate returns `Decision.Allowed` true
- **THEN** order placement proceeds and produces the same result it would without the gate

#### Scenario: Paper and Margin paths are never gated by this change

- **WHEN** an order is proposed on the Paper or Margin order-placement path
- **THEN** the risk gate is not invoked and the order path behaves exactly as it does today

### Requirement: Live and Paper enablement is operator-gated and off by default

The system SHALL expose an explicit enablement flag for the risk gate that defaults to enabled for Simulation and disabled for Paper and Margin. Extending the gate to Paper or Margin SHALL require the operator to opt in explicitly; no default configuration, and no path introduced by this change, SHALL cause the gate to block or alter a Paper or Margin order.

#### Scenario: Default configuration leaves Paper and Margin ungated

- **WHEN** the platform starts with default risk-overlay configuration
- **THEN** the gate is active for Simulation and inactive for Paper and Margin, and no operator action is required for Simulation

### Requirement: Crowding view supplied through a lookup seam with a test fake

The system SHALL obtain the `CrowdingView` for a scan cycle through a `CrowdingLookup` interface with a GORM-backed production implementation that reads `crowding_metrics` / `crowding_flagged_candidates` for the relevant `scanned_at` and an in-memory fake implementation used by unit tests, so gate wiring is testable without a live database.

#### Scenario: The in-memory fake returns a preset crowding view

- **WHEN** the fake `CrowdingLookup` is seeded with a flagged cycle naming tickers `NVDA` and `SMCI` and the gate requests the crowding view for that scan cycle
- **THEN** the returned `CrowdingView` is flagged and its ticker set contains exactly `NVDA` and `SMCI`

### Requirement: Taskfile target for portfolio-risk-overlay unit tests

The system SHALL provide a `task test:portfolio-risk-overlay` Taskfile target that runs `go test` for the `src/go/tradingstack/riskoverlay` package. The target SHALL exit non-zero if any limit-breach, boundary, reduction-order, crowding, or config-loader test fails.

#### Scenario: Task target runs the risk-overlay unit-test suite

- **WHEN** an operator runs `task test:portfolio-risk-overlay`
- **THEN** the `riskoverlay` package's unit tests execute and the command exits zero when all tests pass, non-zero when any fails
