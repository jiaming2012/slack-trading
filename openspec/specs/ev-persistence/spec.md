# ev-persistence Specification

## Purpose
TBD - created by archiving change ev-tracker. Update Purpose after archive.
## Requirements
### Requirement: Persist one weight row per strategy per regime

The system SHALL persist the computed results into the `strategy_ev_weights` table (via the `trading-stack-schema` `StrategyEvWeight` model), writing exactly one row per computed `(strategy_id, regime)` group. Each row SHALL carry the run's `computed_at` (the as-of timestamp), `strategy_id`, `regime`, `ev_30d`, `ev_90d`, `ev_slope`, and `ev_weight` matching the values the computation engine produced. A null slope SHALL be stored as SQL NULL in `ev_slope`.

#### Scenario: Computed values round-trip into strategy_ev_weights

- **WHEN** a recompute runs against a testcontainers Postgres for a fixture with two `(strategy_id, regime)` groups
- **THEN** exactly two rows exist in `strategy_ev_weights`, and each row's `ev_30d`, `ev_90d`, `ev_slope`, and `ev_weight` equal the engine's computed values for that group

#### Scenario: Null slope persists as SQL NULL

- **WHEN** a group has insufficient data for a slope (fewer than two non-empty buckets)
- **THEN** its persisted row has `ev_slope` as SQL NULL and `ev_weight` 0.7

### Requirement: Retired strategies are excluded from persistence

The system SHALL accept a set of retired strategy identifiers, and SHALL NOT compute or persist any row for a retired strategy — retirement means zero rows (effective weight 0), never a row with weight 0.

#### Scenario: A retired strategy produces no row

- **WHEN** a recompute runs with `strat-B` marked retired and fixture trades present for both `strat-A` and `strat-B`
- **THEN** `strategy_ev_weights` contains rows for `strat-A` only and no row references `strat-B`

### Requirement: Recompute reads trades through a pluggable repository

The system SHALL obtain closed-trade outcomes through a repository interface rather than a hard-coded data source, so an in-memory fixture repository can drive the full recompute-and-persist path in tests while a live-trade-backed repository can be supplied later without changing the engine.

#### Scenario: An in-memory fixture repository drives recompute end to end

- **WHEN** an in-memory repository returning a synthetic trade set is passed to the recompute entry point along with a Postgres handle and an as-of timestamp
- **THEN** the recompute completes and writes the expected `strategy_ev_weights` rows without any live or external data source

### Requirement: Recompute is reproducible for a given as-of timestamp

Re-running the recompute for the same as-of timestamp and the same trade inputs SHALL yield a persisted result set whose latest `computed_at` rows reflect those inputs deterministically, so a re-run never leaves stale or duplicated weights that disagree with the inputs.

#### Scenario: Re-running with changed inputs reflects the new inputs

- **WHEN** a recompute runs, then the fixture trades change and a second recompute runs for the same as-of timestamp
- **THEN** reading the latest `computed_at` rows for each `(strategy_id, regime)` yields the weights derived from the second run's inputs

### Requirement: Taskfile target runs the EV tracker suite

The project SHALL provide a `task test:ev-tracker` target that runs the `evtracker` package unit and testcontainers tests and exits non-zero if any test fails.

#### Scenario: Task target runs the suite and fails on error

- **WHEN** an operator runs `task test:ev-tracker`
- **THEN** the EV computation unit tests and the testcontainers persistence tests execute, and the command exits non-zero if any of them fail

