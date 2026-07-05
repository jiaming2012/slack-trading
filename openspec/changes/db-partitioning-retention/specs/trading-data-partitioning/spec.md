## ADDED Requirements

### Requirement: Monthly Range Partitioning for scan_results
`scan_results` SHALL be a native PostgreSQL partitioned table, `PARTITION BY RANGE (scanned_at)`, with monthly partition boundaries and a composite primary key `(id, scanned_at)`.

#### Scenario: row lands in the partition matching its scanned_at month
- **WHEN** a row is inserted into `scan_results` with `scanned_at` in a given calendar month whose partition already exists
- **THEN** the row is physically stored in that month's partition (verifiable via `tableoid::regclass` or `information_schema.partitions`), and querying `scan_results` for that month's date range returns the row

### Requirement: Monthly Range Partitioning for sim_outcomes
`sim_outcomes` SHALL be a native PostgreSQL partitioned table, `PARTITION BY RANGE (simulated_at)`, with monthly partition boundaries and a composite primary key `(id, simulated_at)`. The `scan_result_id` column SHALL be retained, but no database-level foreign key constraint to `scan_results.id` SHALL be defined; referential integrity between `sim_outcomes.scan_result_id` and `scan_results.id` is enforced at the application/service layer only.

#### Scenario: row lands in the partition matching its simulated_at month
- **WHEN** a row is inserted into `sim_outcomes` with `simulated_at` in a given calendar month whose partition already exists
- **THEN** the row is physically stored in that month's partition, and querying `sim_outcomes` for that month's date range returns the row

#### Scenario: no database-level foreign key blocks or is required for inserts
- **WHEN** a row is inserted into `sim_outcomes` referencing a `scan_result_id` that does not exist in `scan_results`
- **THEN** the insert succeeds at the database level (no FK constraint violation is raised), because referential integrity for this relationship is an application-layer concern documented as a trade-off of partitioning, not a database constraint

### Requirement: Idempotent Monthly Partition Provisioning
The system SHALL provide a provisioning routine that ensures partitions exist for the current calendar month and a configurable horizon of future months (default: 3) for both `scan_results` and `sim_outcomes`, creating any missing partition with `CREATE TABLE IF NOT EXISTS ... PARTITION OF ... FOR VALUES FROM (...) TO (...)`. Running the routine when all required partitions already exist SHALL be a no-op that does not error.

#### Scenario: missing partitions are created ahead of need
- **WHEN** the provisioning routine is run with a 3-month horizon and no partitions yet exist for the current or next 3 months
- **THEN** exactly 4 monthly partitions are created for `scan_results` and 4 for `sim_outcomes`, each with correct `FOR VALUES FROM/TO` month boundaries

#### Scenario: re-running provisioning is idempotent
- **WHEN** the provisioning routine is run twice in succession with the same horizon and no time has elapsed
- **THEN** the second run creates zero new partitions and does not error

### Requirement: No Catch-All Default Partition
Neither `scan_results` nor `sim_outcomes` SHALL have a `DEFAULT` partition. An insert whose partition-key value falls in a month with no provisioned partition SHALL fail with a database error rather than being silently absorbed into a catch-all partition.

#### Scenario: insert into an unprovisioned month fails loudly
- **WHEN** a row is inserted into `scan_results` (or `sim_outcomes`) with a partition-key timestamp in a month for which no partition has been provisioned
- **THEN** the insert fails with a Postgres "no partition of relation found for row" error, and no row is written to any partition

### Requirement: Operator-Visible Partition Provisioning Command
The system SHALL expose partition provisioning as an operator-runnable Taskfile target, `db:partition:ensure`, that invokes the provisioning routine against the configured database and reports which partitions (if any) were created.

#### Scenario: operator runs the provisioning task
- **WHEN** an operator runs `task db:partition:ensure`
- **THEN** the command completes successfully and prints the set of partitions that exist (or were newly created) for `scan_results` and `sim_outcomes`

### Requirement: No Impact to Non-Partitioned or Pre-Existing Tables
`feature_distributions`, `strategy_ev_weights`, `simulator_fidelity`, `scanner_configs`, and every pre-existing (pre-v4) database table SHALL remain unpartitioned and structurally unchanged by this capability.

#### Scenario: schema diff shows only the two named tables partitioned
- **WHEN** the migration introduced by this change is applied to a fresh database that already has the `trading-stack-schema` tables
- **THEN** only `scan_results` and `sim_outcomes` appear as partitioned relations in `pg_partitioned_table`, and no other table's DDL (columns, indexes, constraints) changes as a result of applying this migration
