# legacy-enum-compat-mapping

## ADDED Requirements

### Requirement: Legacy persisted values map to Mode on read

At the persistence boundary, when a playground row is loaded from Postgres, the code SHALL translate its legacy `environment` and `live_account_type` column values into a single `Mode` using a fixed mapping: `(simulator, mock)` and `(simulator, simulator)` map to `Simulation`; `(live, paper)` maps to `Paper`; `(live, margin)` maps to `Margin`. The mapping SHALL be total over the value combinations that exist in production data, and an unrecognized combination SHALL surface a clear error rather than a silent default.

#### Scenario: Simulation legacy rows map to Simulation

- **WHEN** a row with `environment="simulator"` and `live_account_type` of either `"mock"` or `"simulator"` is read
- **THEN** the resulting playground's `Mode` is `Simulation`

#### Scenario: Paper and Margin legacy rows map correctly

- **WHEN** a row with `environment="live"` and `live_account_type="paper"` is read, and separately a row with `environment="live"` and `live_account_type="margin"`
- **THEN** the first yields `Mode` = `Paper` and the second yields `Mode` = `Margin`

#### Scenario: Unrecognized combination is an explicit error

- **WHEN** a row with an `(environment, live_account_type)` combination outside the documented mapping is read
- **THEN** the read returns a non-nil error naming the offending values rather than silently choosing a mode

### Requirement: Mode writes back to legacy string values without mutating stored data

When a playground is persisted, the code SHALL write the legacy `environment` and `live_account_type` column values that correspond to its `Mode`, choosing the canonical legacy pair (`Simulation`→`(simulator, simulator)`, `Paper`→`(live, paper)`, `Margin`→`(live, margin)`). This change SHALL NOT run any data migration, backfill, or bulk `UPDATE` against existing rows; stored production values are left exactly as they are and are only reinterpreted at the boundary.

#### Scenario: Round-trip preserves an existing row's mode

- **WHEN** a legacy row is read into a `Mode` and then written back out
- **THEN** the written `environment` / `live_account_type` pair maps to the same `Mode` on a subsequent read (mode round-trip is idempotent)

#### Scenario: No migration or bulk update ships in this change

- **WHEN** this change's diff is inspected for schema migrations or bulk `UPDATE`/backfill statements against playground rows
- **THEN** none are present — the persisted column values and schema are unchanged

### Requirement: RPC request fields map to Mode at the router boundary

The `CreatePlayground` RPC handler SHALL translate the request's `environment` and optional `live_account_type` fields into a `Mode` using the same mapping, and SHALL reject with a clear error any request whose combination does not correspond to one of the three mode presets.

#### Scenario: Valid request combination resolves to a mode

- **WHEN** a `CreatePlayground` request carries `environment="live"` and `live_account_type="paper"`
- **THEN** the handler resolves it to `Mode` = `Paper` and proceeds

#### Scenario: Invalid request combination is rejected

- **WHEN** a `CreatePlayground` request carries a contradictory combination (for example `environment="live"` with `live_account_type="simulator"`)
- **THEN** the handler returns an error and does not create a playground

### Requirement: Operator-visible verification task

A `taskfile.yml` target SHALL run the mode round-trip / diff-test verification so the operator can confirm the migration is behavior-preserving with one command.

#### Scenario: Verification task exists and runs the round-trip test

- **WHEN** the added `task` target is invoked
- **THEN** it executes the mode round-trip / diff-test and exits non-zero if any legacy combination fails to map to the same mode on re-read (verification gate G4)
