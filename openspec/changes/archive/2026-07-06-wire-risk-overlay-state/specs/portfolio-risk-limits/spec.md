# portfolio-risk-limits — Delta for wire-risk-overlay-state

## MODIFIED Requirements

### Requirement: Portfolio drawdown circuit breaker halts entries

The system SHALL compute rolling 5-session portfolio drawdown from a supplied trailing equity series as `(peak - current) / peak * 100`, where `peak` is the maximum equity over the trailing window. When that drawdown strictly exceeds `limits.MaxDrawdownPct`, the system SHALL reject all proposed **entries** (orders that open or increase a logical position) with a `drawdown_breaker` breach. Drawdown exactly equal to the limit SHALL NOT trip the breaker.

When the trailing equity series is non-empty and its 5-session peak is less than or equal to zero, the system SHALL treat the breaker as tripped and reject all proposed entries with a `drawdown_breaker` breach whose reason names the non-positive peak — a wiped-out or negative-equity book MUST fail safe (halt entries) rather than silently disabling the breaker. When the trailing equity series is empty, the breaker SHALL remain inactive (no `drawdown_breaker` breach can be produced), since absence of data is not evidence of catastrophe. Reduction orders SHALL remain unaffected in every case, per the risk-reducing-orders-always-pass requirement.

#### Scenario: Entries are halted when 5-day drawdown exceeds the limit

- **WHEN** the trailing 5-session equity series has a peak of 100,000 and current equity of 92,000 (8% drawdown), `MaxDrawdownPct` is 5.0, and an entry is proposed
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `drawdown_breaker` breach

#### Scenario: Drawdown exactly at the limit does not trip the breaker

- **WHEN** trailing drawdown is exactly 5.0% and `MaxDrawdownPct` is 5.0 and an entry is proposed
- **THEN** no `drawdown_breaker` breach is present

#### Scenario: All-non-positive equity trips the breaker instead of disabling it

- **WHEN** the trailing equity series is non-empty and every value is less than or equal to zero (peak `<= 0`), and an entry is proposed
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `drawdown_breaker` breach whose reason names the non-positive equity peak

#### Scenario: An empty equity series leaves the breaker inactive

- **WHEN** the trailing equity series is empty and an entry is proposed that satisfies every other limit
- **THEN** no `drawdown_breaker` breach is present

### Requirement: Per-strategy capital allocation caps weighted by EV rank

The system SHALL derive each strategy's capital allocation cap as `deployableCapital * normalizedEvWeight(strategyID)`, where `normalizedEvWeight` is the strategy's `strategy_ev_weights.ev_weight` divided by the sum of all participating strategies' `ev_weight` values (a strategy absent from a non-empty EV-weight set SHALL receive a weight of zero). The system SHALL reject a proposed entry whose resulting deployed capital for its strategy would strictly exceed that strategy's cap. A strategy with a zero cap SHALL have every entry rejected. Allocation exactly equal to the cap SHALL NOT be rejected.

When the supplied EV-weight set is **empty** (no EV-weight data participating at all), the strategy-allocation family SHALL be **pinned inactive**: the system SHALL NOT produce any `strategy_allocation` breach for any entry. This pin is a deliberate data-availability semantic, not enforcement — capping every strategy at zero for missing data would halt all entries platform-wide, which is the drawdown breaker's job. Because the pin silently removes an entire limit family, the pin state MUST be observable by the consuming gate (see the `simulation-risk-gate` capability's EV-pin telemetry and alert requirement) — the engine itself remains pure and signals nothing.

#### Scenario: A higher-EV strategy gets a larger cap than a lower-EV strategy

- **WHEN** strategy A has `ev_weight` 0.75 and strategy B has `ev_weight` 0.25, `deployableCapital` is 100,000, and both propose an entry that would deploy 40,000
- **THEN** strategy A's entry has no `strategy_allocation` breach (cap 75,000) and strategy B's entry has a `strategy_allocation` breach (cap 25,000)

#### Scenario: A strategy absent from a non-empty EV-weight set is capped at zero

- **WHEN** a proposed entry names a strategy with no row in the supplied non-empty EV-weight set
- **THEN** `Decision.Allowed` is false and `Breaches` contains a `strategy_allocation` breach

#### Scenario: An empty EV-weight set pins the allocation family inactive

- **WHEN** the supplied EV-weight set is empty and a proposed entry names any strategy while satisfying every other limit
- **THEN** `Decision.Allowed` is true and no `strategy_allocation` breach is present
