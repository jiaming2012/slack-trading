# strategy-capacity-estimate

## ADDED Requirements

### Requirement: Configurable market-impact parameters

The system SHALL extend `CostParams` (defined by the `net-ev-cost-model` capability) with an `impact_coefficient` (dimensionless square-root market-impact constant) and a `max_impact_fraction_of_edge` (the fraction `X`, in `[0, 1]`, of expected per-share edge that estimated impact is allowed to consume). Both SHALL have built-in defaults and SHALL be overridable via the same YAML config file and loader used for the cost parameters.

#### Scenario: Impact parameters load with the rest of CostParams

- **WHEN** the cost-params loader loads a YAML file that sets `impact_coefficient` and `max_impact_fraction_of_edge`
- **THEN** the returned `CostParams` reflects both overridden values alongside any commission/spread/borrow overrides in the same file

### Requirement: Capacity estimate via square-root market impact

The system SHALL provide an `EstimateCapacity` function that, given an expected per-share edge, a reference price, an average daily volume, and a `CostParams`, computes the maximum share size at which estimated market impact — modeled as `impact_coefficient × price × sqrt(shares / avg_daily_volume)` per share — equals `max_impact_fraction_of_edge × edge_per_share`, using the closed-form solution `max_shares = avg_daily_volume × (max_impact_fraction_of_edge × edge_per_share / (impact_coefficient × price))²`. The function SHALL return `0` (not an error, not a negative number) whenever `edge_per_share`, `price`, or `avg_daily_volume` is non-positive.

#### Scenario: Capacity computed from hand-computed fixture inputs

- **WHEN** `EstimateCapacity` is called with `edge_per_share=2.00`, `price=50`, `avg_daily_volume=1000000`, `impact_coefficient=0.1`, and `max_impact_fraction_of_edge=0.2`
- **THEN** it returns `max_shares = 6400`

#### Scenario: Non-positive edge yields zero capacity, not an error

- **WHEN** `EstimateCapacity` is called with `edge_per_share=0` (or negative) and otherwise valid inputs
- **THEN** it returns `0` and a nil error, signaling "no capacity at zero or negative edge" rather than dividing by zero or panicking

#### Scenario: Larger average daily volume increases capacity proportionally

- **WHEN** `EstimateCapacity` is called twice with identical inputs except `avg_daily_volume` doubled
- **THEN** the second call's returned `max_shares` is exactly double the first call's, holding `edge_per_share`, `price`, `impact_coefficient`, and `max_impact_fraction_of_edge` fixed

### Requirement: capacity_shares persisted alongside gross_ev and net_ev

The system SHALL persist the result of `EstimateCapacity` in the `capacity_shares` column added to `strategy_ev_weights` by the `net-ev-cost-model` capability's migration (`MigrateNetEvCostModel`), so capacity travels with the same per-strategy, per-regime row as `gross_ev` and `net_ev`.

#### Scenario: capacity_shares round-trips on the same row as gross_ev and net_ev

- **WHEN** a `strategy_ev_weights` row's `gross_ev`, `net_ev`, and `capacity_shares` columns are set in the same update
- **THEN** reading the row back returns all three values unchanged and attributable to the same `strategy_id` and `regime`
