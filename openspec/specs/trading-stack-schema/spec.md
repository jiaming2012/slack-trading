# trading-stack-schema Specification

## Purpose
TBD - created by archiving change trading-stack-schema. Update Purpose after archive.
## Requirements
### Requirement: Isolated trading-stack package and migration entry point

The system SHALL provide a new Go package `tradingstack` (import path `github.com/jiaming2012/slack-trading/src/go/tradingstack`) that contains the v4 trading-stack domain models and a single migration entry point `MigrateTradingStack(db *gorm.DB) error`. `MigrateTradingStack` SHALL create only the six trading-stack tables and their constraints and SHALL NOT create, alter, or drop any existing playground table (`playgrounds`, `order_records`, `trade_records`, `equity_plot_records`, `live_accounts`, `live_account_plots`). The existing playground migration path (`dbutils.InitPostgres`) SHALL remain unchanged.

#### Scenario: Migration creates exactly the six trading-stack tables

- **WHEN** `MigrateTradingStack` runs against an empty PostgreSQL database
- **THEN** the tables `scan_results`, `sim_outcomes`, `simulator_fidelity`, `strategy_ev_weights`, `scanner_configs`, and `feature_distributions` exist with those exact names
- **AND** no table named `playgrounds`, `order_records`, `trade_records`, `equity_plot_records`, `live_accounts`, or `live_account_plots` is created by the call

#### Scenario: Migration is idempotent

- **WHEN** `MigrateTradingStack` runs twice against the same database
- **THEN** the second call returns a nil error and leaves the six tables and their constraints unchanged

### Requirement: scan_results model with data_as_of invariant

The system SHALL provide a `ScanResult` model mapped to table `scan_results` with fields faithful to the architecture SQL: `id` (UUID primary key, populated on create when unset), `scanned_at` (not null), `ticker` (not null), `regime_tag`, `regime_confidence`, `price`, `volume_ratio`, `rsi_14`, `atr_pct`, `price_vs_50ma`, `compression_score`, `short_interest`, `sector_momentum`, `sector`, `scanner_score`, `scanner_version`, and `data_as_of`. The `price_vs_50ma`, `compression_score`, and `sector_momentum` columns SHALL be nullable numeric columns added additively through the existing tradingstack migration path (`MigrateTradingStack`), idempotently, without altering or rewriting any existing column or row — rows persisted before these columns existed SHALL simply read back NULL for them. The system SHALL enforce the invariant `data_as_of <= scanned_at` both by Go validation before persist (returning a sentinel error) and by a database CHECK constraint, so a violating row cannot be stored.

#### Scenario: Valid scan_result round-trips

- **WHEN** a `ScanResult` with `data_as_of` earlier than or equal to `scanned_at` is saved and re-read by id
- **THEN** the read-back row equals the written row field-for-field, including a non-zero generated `id`

#### Scenario: data_as_of after scanned_at is rejected

- **WHEN** a `ScanResult` whose `data_as_of` is strictly after its `scanned_at` is saved
- **THEN** the save returns a non-nil error
- **AND** no row is written to `scan_results`

#### Scenario: The three widened feature columns round-trip including NULL

- **WHEN** one `ScanResult` is saved with `price_vs_50ma` 2.5, `compression_score` 37.0, and `sector_momentum` 0.04, and another is saved with all three unset, and both are re-read by id
- **THEN** the first row reads back exactly those three values from columns named `price_vs_50ma`, `compression_score`, and `sector_momentum`, and the second row reads back NULL for all three

#### Scenario: Widening migration is additive and idempotent

- **WHEN** `MigrateTradingStack` runs against a database whose `scan_results` table predates the three widened columns, and then runs a second time
- **THEN** the three columns exist as nullable numeric columns, every pre-existing row is preserved with NULL in them, and the second run returns a nil error leaving the schema unchanged

### Requirement: sim_outcomes model with foreign key and exit_reason enum

The system SHALL provide a `SimOutcome` model mapped to table `sim_outcomes` with fields: `id` (UUID primary key), `scan_result_id` (foreign key referencing `scan_results.id`), `simulated_at` (not null), `strategy_id`, `entry_price`, `exit_price`, `stop_price`, `target_price`, `pnl_pct`, `hold_days`, `exit_reason`, `max_drawdown`, and `outcome_label`. The system SHALL constrain `exit_reason` to exactly the set `{stop, target, timeout, signal_exit}` via Go validation and a database CHECK constraint, and SHALL enforce the `scan_result_id → scan_results.id` foreign key at the database level.

#### Scenario: Valid sim_outcome round-trips against an existing scan_result

- **WHEN** a `ScanResult` is saved and a `SimOutcome` referencing its id with `exit_reason` of `target` is saved
- **THEN** the `SimOutcome` re-read by id equals the written row and its `scan_result_id` matches the parent `ScanResult.id`

#### Scenario: Unknown exit_reason is rejected

- **WHEN** a `SimOutcome` with `exit_reason` set to a value outside `{stop, target, timeout, signal_exit}` is saved
- **THEN** the save returns a non-nil error and no row is written

#### Scenario: Orphan scan_result_id is rejected

- **WHEN** a `SimOutcome` referencing a `scan_result_id` that does not exist in `scan_results` is saved
- **THEN** the save returns a foreign-key violation error and no row is written

### Requirement: simulator_fidelity drift model

The system SHALL provide a `SimulatorFidelity` model mapped to table `simulator_fidelity` (singular, exact name) with fields: `id` (UUID primary key), `computed_at`, `strategy_id`, `period_start`, `period_end`, `drift_pnl`, `drift_fill`, `drift_score`, and `within_tolerance` (boolean). `drift_score` SHALL be persisted and read back as a numeric composite value.

#### Scenario: Fidelity row round-trips including boolean and drift scores

- **WHEN** a `SimulatorFidelity` with `drift_score` of 0.42 and `within_tolerance` true is saved and re-read by id
- **THEN** the read-back `drift_score` equals 0.42 and `within_tolerance` is true

### Requirement: strategy_ev_weights model

The system SHALL provide a `StrategyEvWeight` model mapped to table `strategy_ev_weights` with fields: `id` (UUID primary key), `computed_at`, `strategy_id`, `regime`, `ev_30d`, `ev_90d`, `ev_slope`, and `ev_weight`. All four EV fields SHALL round-trip as numeric values, including negative `ev_slope`.

#### Scenario: EV weight row round-trips including negative slope

- **WHEN** a `StrategyEvWeight` with `ev_30d`, `ev_90d`, a negative `ev_slope`, and `ev_weight` in [0.0, 1.0] is saved and re-read by id
- **THEN** all four EV fields equal the written values, preserving the negative `ev_slope`

### Requirement: scanner_configs JSONB versioning and rollback by ID

The system SHALL provide a `ScannerConfig` model mapped to table `scanner_configs` with fields: `id` (UUID primary key), `created_at`, `regime`, `config_json` (stored as JSONB), and `optimizer_run_id`. The system SHALL support rollback by fetching any prior config verbatim by its `id`, returning the stored `config_json` byte-for-byte semantically (same keys and values).

#### Scenario: Config JSONB round-trips verbatim

- **WHEN** a `ScannerConfig` whose `config_json` is `{"min_volume_ratio":1.5,"filters":["rsi","atr"]}` is saved and re-read by its id
- **THEN** the read-back `config_json` deserializes to the same keys and values as written

#### Scenario: Rollback fetches a prior version by id

- **WHEN** two `ScannerConfig` rows with different `config_json` are saved and the first row's id is fetched
- **THEN** the fetched row's `config_json` equals the first row's config, unaffected by the later row

### Requirement: feature_distributions model

The system SHALL provide a `FeatureDistribution` model mapped to table `feature_distributions` with fields: `computed_at`, `feature_name`, `mean`, `std_dev`, `p25`, and `p75`. Because the source SQL declares no primary key, the model SHALL add a surrogate UUID `id` primary key so it can be managed by GORM; this deviation SHALL be documented in design.md.

#### Scenario: Feature distribution row round-trips

- **WHEN** a `FeatureDistribution` for `feature_name` `rsi_14` with `mean`, `std_dev`, `p25`, and `p75` set is saved and re-read by id
- **THEN** the read-back row equals the written row for all six declared fields

### Requirement: Round-trip test task target

The system SHALL provide a `task test:trading-stack` Taskfile target that runs the `tradingstack` package's testcontainers-backed round-trip and invariant tests against an ephemeral PostgreSQL container. The target SHALL exit non-zero if any round-trip, invariant, enum, or foreign-key test fails.

#### Scenario: Task target runs the round-trip suite

- **WHEN** an operator runs `task test:trading-stack` with Docker available
- **THEN** the `tradingstack` package tests execute against a throwaway PostgreSQL container and the command exits zero when all tests pass

