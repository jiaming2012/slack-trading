# simulation-risk-gate Specification

## Purpose
TBD - created by archiving change portfolio-risk-overlay. Update Purpose after archive.
## Requirements
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

### Requirement: Gate installed at startup and default-enabled for Simulation

The system SHALL install the Simulation risk gate (`models.SetRiskGate`) during server startup, wired with the production portfolio snapshot builder, the crowding lookup, the EV-weight lookup, and the sector lookup, and composed at the Broker seam AFTER the kill-switch order gate so the overlay can never mask or disable a halt. The `riskOverlay.enabled` flag SHALL default to **true**, and the shipped default limit values SHALL be permissive (documented no-op values), so default enablement changes no order outcome until an operator narrows a limit. The gate SHALL be consulted only on the Simulation order-placement path; Paper and Margin order-placement paths SHALL remain ungated by this change, and any future Paper or Margin enablement SHALL require an explicit operator-approved change. A present-but-invalid risk-overlay config SHALL fail startup with the existing sentinel validation error. A present-but-**unreadable** config (any read failure other than the file not existing — permissions, a bind-mount fault, the path resolving to a directory) SHALL likewise fail startup rather than silently applying the permissive, enabled defaults: the unreadable file may carry the operator's `enabled: false` rollback or narrowed limits, and defaulting would reverse that intent. Only a genuinely ABSENT config SHALL yield the documented defaults and start normally.

#### Scenario: Startup installs an enabled gate under default configuration

- **WHEN** the server starts with no `risk-overlay-config.yaml` present or with the shipped sample defaults
- **THEN** the risk gate is installed, reports enabled, and every Simulation order is evaluated against the documented permissive default limits with unchanged outcomes

#### Scenario: Kill switch is consulted before the risk overlay

- **WHEN** the kill switch is engaged and a Simulation order is placed
- **THEN** the order is rejected by the kill-switch gate before the risk overlay is consulted, regardless of the overlay's configuration

#### Scenario: Operator disables the gate with one config line

- **WHEN** the operator sets `riskOverlay.enabled: false` and restarts the server
- **THEN** the gate permits every Simulation order without evaluation (permissive-blind) and Simulation behavior is identical to the pre-installation state

#### Scenario: Paper and Margin paths remain ungated

- **WHEN** an order is placed on the Paper or Margin order-placement path with the gate installed and enabled
- **THEN** the risk gate is not consulted and the order path behaves exactly as it does today

#### Scenario: An unreadable config fails startup instead of reversing operator intent

- **WHEN** the risk-overlay config file exists but cannot be read (for example a permissions failure or the path resolving to a directory)
- **THEN** server startup fails loudly, and the gate is never silently booted with the permissive, enabled defaults

### Requirement: Portfolio state feed built from live playground state

The system SHALL provide a production portfolio snapshot builder that maps a live Simulation playground and an incoming order into the engine's inputs: (1) logical positions carrying ticker, sector, and signed notional derived from position quantity and current price (falling back to cost basis when no current price is available), with the ×100 contract multiplier applied to option positions; (2) per-strategy deployed capital, attributing each open position's absolute notional to a strategy identity derived from the opening order's tag, falling back to the playground's client ID when the tag is empty; (3) the trailing 5-session equity series derived from the playground's in-memory equity plot (one closing value per session date, ending with current equity), without any database read. EV weights SHALL be obtained through an `EvWeightLookup` seam (GORM-backed production implementation reading the most recent `strategy_ev_weights` row per `strategy_id`, plus an in-memory fake for tests), and sector attribution through a `SectorLookup` seam (GORM-backed implementation reading the latest non-null `scan_results.sector` for a ticker, plus an in-memory fake). A ticker with no known sector SHALL map to an empty sector — leaving it exempt from the sector-concentration family — and SHALL be recorded as degradation per the telemetry requirement below.

#### Scenario: Snapshot reflects seeded playground state

- **WHEN** the snapshot builder runs against a playground fixture holding known positions, orders with strategy tags, and an equity plot spanning more than five sessions, with fakes seeded with EV weights and sectors
- **THEN** the resulting `PortfolioState` carries the expected signed notionals, sectors, per-strategy deployed capital, EV-weight map, and exactly the last five sessions' equity values ending with current equity

#### Scenario: Unknown sector degrades visibly instead of failing

- **WHEN** the sector lookup has no sector for a proposed entry's ticker
- **THEN** the order is evaluated with an empty sector (the sector-concentration family does not apply to it) and the degradation counter is incremented with reason `sector_unknown`

#### Scenario: In-memory fakes make the feed testable without a database

- **WHEN** unit tests seed the fake EV-weight and sector lookups and invoke the gate end-to-end against a playground fixture
- **THEN** gate decisions are exercised deterministically with no live database

### Requirement: Option orders carry the contract multiplier in notional mapping

The system SHALL apply the ×100 option contract multiplier when deriving notional from an order or position of option class: an option order's absolute notional SHALL be `|quantity| × price × 100`, and option positions in the portfolio snapshot SHALL use the same multiplier, so both sides of every exposure comparison agree. Equity-class orders and positions (including orders with an empty class, which historically default to equity) SHALL remain `|quantity| × price`.

#### Scenario: An option order's notional includes the multiplier

- **WHEN** an option-class order for 3 contracts at a price of 2.50 is mapped to a `ProposedOrder`
- **THEN** its absolute signed notional is 750.00 (3 × 2.50 × 100), not 7.50

#### Scenario: An equity order's notional is unchanged

- **WHEN** an equity-class order for 100 shares at 50.00 is mapped to a `ProposedOrder`
- **THEN** its absolute signed notional is 5,000.00

### Requirement: Reduction orders bypass the gate before any I/O

The system SHALL classify reduction orders (any `sell`, `*_to_close`, or `buy_to_cover` side) from the raw order and permit them BEFORE building any portfolio snapshot or consulting any lookup, so a risk-reducing exit can never be blocked, delayed, or misclassified by a snapshot error, database failure, or lookup outage. This preserves the existing reduction-orders-bypass-all-limit-paths decision intact.

#### Scenario: A reduction order passes while every lookup is failing

- **WHEN** the snapshot builder and every lookup are configured to return errors and a sell-to-close Simulation order is placed
- **THEN** the order is permitted, no lookup is consulted, and no degradation is recorded for it

### Requirement: System-generated orders bypass the risk overlay

The system SHALL NOT evaluate system-generated orders (order records flagged `IsSystemOrder` — exercise settlement legs, deferred auto-closes, and other broker-internal placements) against the portfolio risk overlay: the gate SHALL permit them BEFORE building any portfolio snapshot or consulting any lookup, exactly like reduction orders. Settlement and auto-close placements are mechanical consequences of positions already held, not new risk decisions — an ITM exercise settlement leg is emitted as a plain `buy`, which side classification alone would treat as a new entry, and rejecting it under an operator-narrowed limit would error the tick with no deferral and wedge every subsequent tick on the unsettled expiry. The kill-switch order gate's treatment of system orders SHALL remain unchanged: it still runs first for every order in every mode.

#### Scenario: An ITM exercise settlement leg passes under a narrowed limit

- **WHEN** an operator has narrowed `max_gross_exposure` below the book's post-settlement gross exposure and a held call expires in the money, producing a system-generated stock-delivery `buy` order
- **THEN** the risk overlay permits the settlement leg without evaluation, no degradation or rejection is recorded for it, and tick processing completes

#### Scenario: The same order without the system flag is still evaluated

- **WHEN** an identical non-system `buy` order is placed under the same narrowed limit
- **THEN** the overlay evaluates it and rejects it with the gross-exposure breach

### Requirement: Fail-permissive degradation is recorded in internal telemetry

When the gate permits a non-reducing order because it could not evaluate it — a snapshot-build error, a crowding-lookup error, or an unknown sector — the system SHALL increment a `grodt.riskoverlay.degraded` counter in the internal telemetry registry (`telemetry.Default`), labeled with a `reason` of `snapshot_error`, `crowding_lookup_error`, or `sector_unknown`, in addition to the existing Warn log, so degradation is visible to database-backed alerting without any external observability stack. The alert engine SHALL raise an operator Alert when degradation events within its evaluation window exceed a configurable threshold. Terminology SHALL be corrected in code comments and operator-facing documentation: the **disabled** gate state is **permissive-blind** (no evaluation occurs and nothing is recorded); the fail-permissive path is permissive-observed via this counter. The phrase "permissive-observe" SHALL NOT be used to describe the disabled state.

#### Scenario: A snapshot error permits the order and counts degradation

- **WHEN** the snapshot builder returns an error for a non-reducing Simulation order
- **THEN** the order is permitted, a Warn is logged, and `grodt.riskoverlay.degraded` is incremented with reason `snapshot_error`

#### Scenario: A crowding-lookup error permits the order and counts degradation

- **WHEN** the crowding lookup returns an error while evaluating a non-reducing Simulation order
- **THEN** the order is permitted and `grodt.riskoverlay.degraded` is incremented with reason `crowding_lookup_error`

#### Scenario: Sustained degradation raises an operator alert

- **WHEN** degradation events within the alert engine's evaluation window exceed the configured threshold
- **THEN** an Alert is raised through the existing alert engine and delivered to the operator

#### Scenario: A disabled gate is permissive-blind and records nothing

- **WHEN** the gate is disabled and a Simulation order is placed
- **THEN** the order is permitted without evaluation and no degradation counter or rejection counter is recorded

### Requirement: Gate decisions are telemetered

The system SHALL record gate activity in the internal telemetry registry: a `grodt.riskoverlay.rejections` counter incremented once per rejected order per breached limit family, labeled with `limit_type` (one of `gross_exposure`, `net_exposure`, `sector_concentration`, `drawdown_breaker`, `strategy_allocation`, `crowding`), and a `grodt.riskoverlay.enabled` gauge reflecting the gate's enablement state (1 enabled, 0 disabled or not installed), so an operator can always tell whether the overlay is live and what it is blocking.

#### Scenario: A rejection increments the rejections counter per breached family

- **WHEN** the gate rejects a Simulation order that breaches both the gross-exposure and strategy-allocation limits
- **THEN** `grodt.riskoverlay.rejections` is incremented for label `limit_type=gross_exposure` and for label `limit_type=strategy_allocation`

#### Scenario: The enabled gauge reflects gate state

- **WHEN** the server starts with the gate installed and enabled
- **THEN** the `grodt.riskoverlay.enabled` gauge reads 1, and reads 0 after the operator disables the gate and restarts

### Requirement: EV allocation family pin state is telemetered and alert-worthy

The system SHALL expose a `grodt.riskoverlay.ev_family_active` gauge in the internal telemetry registry: 1 when the most recent EV-weight lookup returned a non-empty set (the strategy-allocation family is enforcing), 0 when it returned an empty set (the family is pinned inactive per the `portfolio-risk-limits` empty-set semantics). The alert engine SHALL raise an operator Alert when the gate is enabled while the EV allocation family is pinned inactive, following the existing alert semantics (Slack delivery, re-notify until acknowledged), because an enabled gate silently missing an entire limit family MUST be operator-visible.

#### Scenario: An empty EV-weight set pins the family and raises an alert

- **WHEN** the gate is enabled and the EV-weight lookup returns an empty set during evaluation
- **THEN** `grodt.riskoverlay.ev_family_active` reads 0 and an Alert is raised through the existing alert engine

#### Scenario: A populated EV-weight set reports the family active

- **WHEN** the gate is enabled and the EV-weight lookup returns at least one strategy weight
- **THEN** `grodt.riskoverlay.ev_family_active` reads 1 and no EV-pin alert is active

