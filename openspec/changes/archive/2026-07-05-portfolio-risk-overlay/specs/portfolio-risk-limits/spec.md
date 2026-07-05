# portfolio-risk-limits

## ADDED Requirements

### Requirement: Pure limit-evaluation engine

The system SHALL provide a pure function `Evaluate(state PortfolioState, order ProposedOrder, limits RiskLimits, crowding CrowdingView) (Decision, error)` that, given a snapshot of current portfolio state, one proposed order, the configured limits, and the current scan cycle's crowding view, returns a `Decision` containing `Allowed bool` and `Breaches []LimitBreach`. Each `LimitBreach` SHALL carry a typed `LimitType` (one of `gross_exposure`, `net_exposure`, `sector_concentration`, `drawdown_breaker`, `strategy_allocation`, `crowding`) and a human-readable reason. `Decision.Allowed` SHALL be true if and only if `Breaches` is empty. The function SHALL NOT query a database, call a broker, or read wall-clock time — it SHALL operate only on data passed to it, so it is fully testable with fixtures.

#### Scenario: A clean order under every limit is allowed

- **WHEN** `Evaluate` is called with a proposed entry that keeps gross exposure, net exposure, sector concentration, drawdown, and the strategy's allocation all within their configured limits and whose ticker is not flagged crowded
- **THEN** `Decision.Allowed` is true and `Breaches` is empty

#### Scenario: An order breaching two limits reports both breaches

- **WHEN** `Evaluate` is called with a proposed entry that would exceed both the gross exposure cap and the strategy allocation cap
- **THEN** `Decision.Allowed` is false and `Breaches` contains a breach of `LimitType` `gross_exposure` and a breach of `LimitType` `strategy_allocation`

### Requirement: Maximum aggregate gross and net exposure limits

The system SHALL compute gross exposure as the sum of the absolute signed notional of all logical positions plus the proposed order's absolute notional, and net exposure as the signed sum of all logical position notionals plus the proposed order's signed notional. The system SHALL reject a proposed entry whose resulting gross exposure would strictly exceed `limits.MaxGrossExposure`, and separately reject a proposed entry whose resulting net exposure absolute value would strictly exceed `limits.MaxNetExposure`. A resulting exposure exactly equal to the cap SHALL NOT be rejected.

#### Scenario: Gross exposure over the cap is rejected

- **WHEN** current gross exposure is 90,000, `MaxGrossExposure` is 100,000, and a proposed entry adds 15,000 of absolute notional
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `gross_exposure` breach

#### Scenario: Gross exposure exactly at the cap is allowed

- **WHEN** current gross exposure is 90,000, `MaxGrossExposure` is 100,000, and a proposed entry adds exactly 10,000 of absolute notional
- **THEN** no `gross_exposure` breach is present

#### Scenario: Net exposure over the cap is rejected while gross is within cap

- **WHEN** existing positions net to +80,000, `MaxNetExposure` is 90,000, `MaxGrossExposure` is 1,000,000, and a proposed long entry adds +20,000 signed notional
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `net_exposure` breach and no `gross_exposure` breach

### Requirement: Sector concentration limit

The system SHALL reject a proposed entry that would push any single sector's share of gross exposure — computed as that sector's absolute notional (including the proposed order when it belongs to the sector) divided by resulting total gross exposure — strictly above `limits.MaxSectorConcentrationPct`. Concentration exactly equal to the limit SHALL NOT be rejected.

#### Scenario: An entry that overconcentrates a sector is rejected

- **WHEN** the `technology` sector already holds 40,000 of gross exposure out of 100,000 total, `MaxSectorConcentrationPct` is 50.0, and a proposed `technology` entry adds 30,000 (making technology 70,000 of 130,000 ≈ 53.8%)
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `sector_concentration` breach naming the `technology` sector

#### Scenario: An entry that keeps every sector at or under the limit is allowed

- **WHEN** the same book has `MaxSectorConcentrationPct` of 60.0 and the proposed `technology` entry results in 53.8% technology share
- **THEN** no `sector_concentration` breach is present

### Requirement: Portfolio drawdown circuit breaker halts entries

The system SHALL compute rolling 5-session portfolio drawdown from a supplied trailing equity series as `(peak - current) / peak * 100`, where `peak` is the maximum equity over the trailing window. When that drawdown strictly exceeds `limits.MaxDrawdownPct`, the system SHALL reject all proposed **entries** (orders that open or increase a logical position) with a `drawdown_breaker` breach. Drawdown exactly equal to the limit SHALL NOT trip the breaker.

#### Scenario: Entries are halted when 5-day drawdown exceeds the limit

- **WHEN** the trailing 5-session equity series has a peak of 100,000 and current equity of 92,000 (8% drawdown), `MaxDrawdownPct` is 5.0, and an entry is proposed
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `drawdown_breaker` breach

#### Scenario: Drawdown exactly at the limit does not trip the breaker

- **WHEN** trailing drawdown is exactly 5.0% and `MaxDrawdownPct` is 5.0 and an entry is proposed
- **THEN** no `drawdown_breaker` breach is present

### Requirement: Risk-reducing orders always pass

The system SHALL classify each proposed order as an entry (opens or increases a logical position in the order's direction) or a reduction (reduces or closes an existing logical position). Reduction orders SHALL always be allowed: they SHALL NOT be evaluated against any exposure, concentration, drawdown, strategy-allocation, or crowding limit, and SHALL return `Decision.Allowed` true with no breaches — even when the drawdown circuit breaker is currently halting entries.

#### Scenario: A closing order passes while the circuit breaker is tripped

- **WHEN** the drawdown circuit breaker is tripped (drawdown far above the limit) and a proposed order reduces an existing long logical position
- **THEN** `Decision.Allowed` is true and `Breaches` is empty

### Requirement: Per-strategy capital allocation caps weighted by EV rank

The system SHALL derive each strategy's capital allocation cap as `deployableCapital * normalizedEvWeight(strategyID)`, where `normalizedEvWeight` is the strategy's `strategy_ev_weights.ev_weight` divided by the sum of all participating strategies' `ev_weight` values (a strategy absent from the EV-weight set SHALL receive a weight of zero). The system SHALL reject a proposed entry whose resulting deployed capital for its strategy would strictly exceed that strategy's cap. A strategy with a zero cap SHALL have every entry rejected. Allocation exactly equal to the cap SHALL NOT be rejected.

#### Scenario: A higher-EV strategy gets a larger cap than a lower-EV strategy

- **WHEN** strategy A has `ev_weight` 0.75 and strategy B has `ev_weight` 0.25, `deployableCapital` is 100,000, and both propose an entry that would deploy 40,000
- **THEN** strategy A's entry has no `strategy_allocation` breach (cap 75,000) and strategy B's entry has a `strategy_allocation` breach (cap 25,000)

#### Scenario: A strategy absent from the EV-weight set is capped at zero

- **WHEN** a proposed entry names a strategy with no row in the supplied EV-weight set
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `strategy_allocation` breach

### Requirement: Crowding overlap consumption

The system SHALL accept a `CrowdingView` describing the current scan cycle's crowding result — whether the cycle is flagged and the set of tickers named in `crowding_flagged_candidates`. When `limits.RejectCrowdedEntries` is true and a proposed entry's ticker is in the flagged set, the system SHALL reject the entry with a `crowding` breach. Reduction orders and entries into non-flagged tickers SHALL be unaffected by the crowding view.

#### Scenario: An entry into a flagged-crowded ticker is rejected

- **WHEN** `RejectCrowdedEntries` is true, the crowding view flags ticker `NVDA`, and a proposed entry names ticker `NVDA`
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `crowding` breach

#### Scenario: An entry into a non-flagged ticker under a flagged cycle is allowed

- **WHEN** `RejectCrowdedEntries` is true, the crowding view flags only `NVDA`, and a proposed entry names ticker `KO` while within all other limits
- **THEN** no `crowding` breach is present

#### Scenario: Crowding rejection is disabled when configured off

- **WHEN** `RejectCrowdedEntries` is false and a proposed entry names a flagged-crowded ticker while within all other limits
- **THEN** no `crowding` breach is present
