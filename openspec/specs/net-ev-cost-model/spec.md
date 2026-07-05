# net-ev-cost-model Specification

## Purpose
TBD - created by archiving change net-ev-cost-model. Update Purpose after archive.
## Requirements
### Requirement: Configurable cost parameters

The system SHALL provide a `CostParams` structure carrying, at minimum, a flat commission per trade, a per-share commission, an average bid/ask spread assumption expressed in basis points of entry price, and an annualized short-borrow rate expressed in basis points. The system SHALL provide built-in default values for every field and SHALL support loading overrides from a YAML file at a path resolved from an optional `NET_EV_COST_CONFIG_PATH` environment variable, falling back to `${TRADING_PROJECT_DIR}/src/go/net-ev-cost-config.yaml`, and falling back further to the built-in defaults when no file is present at the resolved path.

#### Scenario: Defaults load when no config file is present

- **WHEN** the cost-params loader is invoked and no file exists at the resolved config path
- **THEN** it returns the built-in default `CostParams` values and a nil error

#### Scenario: YAML overrides replace defaults field-by-field

- **WHEN** a YAML file at the resolved config path sets `avg_spread_bps` and `commission_per_share` but omits `borrow_rate_bps_annual`
- **THEN** the loaded `CostParams` reflects the two overridden values and retains the built-in default for `borrow_rate_bps_annual`

### Requirement: Per-trade transaction cost calculation

The system SHALL provide a `TradeCost` function that computes the total estimated transaction cost of a single closed trade as the sum of: a commission cost (`commission_per_trade + commission_per_share × quantity`), a spread cost (`avg_spread_bps / 10000 × entry_price × quantity`), and — only when the trade's side is `short` — a borrow cost (`borrow_rate_bps_annual / 10000 × entry_price × quantity × hold_days / 360`). For `long` trades the borrow cost component SHALL be exactly zero.

#### Scenario: Long trade cost has no borrow component

- **WHEN** `TradeCost` is called for a long trade with entry price 100, quantity 100, hold days 2, and `CostParams` of `commission_per_trade=1.00`, `commission_per_share=0.01`, `avg_spread_bps=20`, `borrow_rate_bps_annual=360`
- **THEN** it returns a total cost of 22.00 (commission 2.00 + spread 20.00 + borrow 0.00)

#### Scenario: Short trade cost includes borrow accrued over hold days

- **WHEN** `TradeCost` is called for a short trade with entry price 10.00, quantity 1000, hold days 300, and the same `CostParams` as above
- **THEN** it returns a total cost of 331.00 (commission 11.00 + spread 20.00 + borrow 300.00)

### Requirement: Gross EV and net EV computed and classified independently

The system SHALL provide a `ComputeEV` function that, given a set of closed trades and a `CostParams`, computes `GrossEV` from each trade's cost-blind P&L and win/loss classification, and separately computes `NetEV` from each trade's cost-adjusted P&L (`gross P&L − TradeCost`) and win/loss classification. A trade whose gross P&L is positive but whose net P&L is negative after subtracting its `TradeCost` SHALL count as a win toward `GrossEV` and as a loss toward `NetEV`.

#### Scenario: A trade that is gross-profitable but net-unprofitable flips classification

- **WHEN** `ComputeEV` is run over three fixture trades — a long win (gross +500, net +478), a long loss (gross −200, net −223), and a short trade with gross +100 but net −231 after a large borrow cost — using the `CostParams` from the trade-cost scenarios above
- **THEN** `GrossEV` classifies the third trade as a win (2 wins, 1 loss; `GrossEV` = 133.33, rounded to 2 decimals)
- **AND** `NetEV` classifies the third trade as a loss (1 win, 2 losses; `NetEV` = 8.00)

#### Scenario: Empty trade set yields zero EV without a division error

- **WHEN** `ComputeEV` is called with an empty trade slice
- **THEN** it returns `GrossEV = 0` and `NetEV = 0` with a nil error, and does not panic or divide by zero

### Requirement: gross_ev and net_ev persisted as separate, additive columns

The system SHALL provide a `MigrateNetEvCostModel(db *gorm.DB) error` function that idempotently adds `gross_ev NUMERIC`, `net_ev NUMERIC`, and `capacity_shares NUMERIC` columns to the existing `strategy_ev_weights` table (created by the `trading-stack-schema` migration) without altering, dropping, or recreating any existing column, row, or constraint on that table, and without touching any other table.

#### Scenario: Migration adds three columns without disturbing existing data

- **WHEN** `MigrateNetEvCostModel` runs against a database where `trading-stack-schema`'s `MigrateTradingStack` has already created `strategy_ev_weights` with an existing row
- **THEN** the table gains `gross_ev`, `net_ev`, and `capacity_shares` columns
- **AND** the pre-existing row's `id`, `strategy_id`, `regime`, `ev_30d`, `ev_90d`, `ev_slope`, and `ev_weight` values are unchanged

#### Scenario: Migration is idempotent

- **WHEN** `MigrateNetEvCostModel` runs twice in succession against the same database
- **THEN** the second call returns a nil error and the table still has exactly one `gross_ev`, one `net_ev`, and one `capacity_shares` column

### Requirement: Downstream decisions must read net_ev, not gross_ev

The system SHALL document, and `ComputeEV`'s returned result type SHALL expose unambiguously (via separately named `GrossEV` and `NetEV` fields with doc comments), that EV-weight and retirement-signal computations are contractually required to use `NetEV`. `GrossEV` SHALL be retained on the result and in the persisted column solely for display and diagnostic comparison.

#### Scenario: Result type exposes both values with an explicit decision-value contract

- **WHEN** a caller inspects the `EVResult` returned by `ComputeEV`
- **THEN** both `GrossEV` and `NetEV` are present as distinct fields
- **AND** the field-level documentation states that `NetEV` is the only value downstream EV-weight and retirement-signal logic may use for decisions

### Requirement: Taskfile target for cost-model tests

The system SHALL provide a `task test:net-ev-cost-model` Taskfile target that runs the `costmodel` package's Go unit tests and exits non-zero if any test fails.

#### Scenario: Task target runs the cost-model unit test suite

- **WHEN** an operator runs `task test:net-ev-cost-model`
- **THEN** the `src/go/tradingstack/costmodel` package tests execute
- **AND** the command exits zero only when every trade-cost, EV, and migration test passes

