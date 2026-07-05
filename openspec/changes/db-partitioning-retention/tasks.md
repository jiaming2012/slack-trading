## 1. Migration DDL for partitioned tables

- [ ] 1.1 Confirm `trading-stack-schema` has merged and inspect the actual `scan_results` / `sim_outcomes` GORM models/columns it created; note any deviation from `todo/full-trading-stack-architecture.md`'s Database Schema section
- [ ] 1.2 Write a new idempotent SQL migration file (e.g. `src/go/tradingstack/db/partitioning.sql`) that converts `scan_results` into a `PARTITION BY RANGE (scanned_at)` parent with composite primary key `(id, scanned_at)`, and creates the current month's partition
- [ ] 1.3 In the same file, convert `sim_outcomes` into a `PARTITION BY RANGE (simulated_at)` parent with composite primary key `(id, simulated_at)`; drop the `scan_result_id -> scan_results(id)` foreign key constraint (keep the column) and add a comment documenting that referential integrity is now application-enforced
- [ ] 1.4 Register the new file in `infra/migrate.py`'s `SQL_FILES` list, after the existing entries
- [ ] 1.5 Verify (by reading the diff) that no pre-existing table, `src/go/backtester-api/db/init.sql`, `infra/analytics-schema.sql`, or any of `feature_distributions` / `strategy_ev_weights` / `simulator_fidelity` / `scanner_configs` is modified by this migration file

## 2. Partition provisioning

- [ ] 2.1 Create Go package `src/go/tradingstack/partitioning` with `EnsureMonthlyPartitions(ctx, db, tableName, partitionKeyCol string, horizonMonths int) error` that builds and executes `CREATE TABLE IF NOT EXISTS <table>_yYYYY_mMM PARTITION OF <table> FOR VALUES FROM (...) TO (...)` for the current month through `horizonMonths` ahead, for both `scan_results` and `sim_outcomes`
- [ ] 2.2 Ensure the routine is idempotent (re-running with no elapsed time creates zero new partitions and does not error) and never creates a `DEFAULT` partition
- [ ] 2.3 Add a small callable entry point (e.g. a `cmd/` subcommand or a script under `src/go/tradingstack/`) that invokes `EnsureMonthlyPartitions` against the configured database connection, printing which partitions exist/were created
- [ ] 2.4 Add Taskfile target `db:partition:ensure` wired to the entry point from 2.3

## 3. Retention and archival

- [ ] 3.1 Create Go package `src/go/tradingstack/retention` with `FindEligiblePartitions(ctx, db, tableName string, retentionWindow time.Duration) ([]string, error)` returning partitions whose full date range ends before `now() - retentionWindow` (default 12 months)
- [ ] 3.2 Implement `DetachPartition(ctx, db, tableName, partitionName string) error` issuing `ALTER TABLE <table> DETACH PARTITION <partition>`, and confirm (by design/comment plus test in group 4) that it never drops or deletes the detached table
- [ ] 3.3 Define the `ColdStorageArchiver` interface (single method, e.g. `Archive(ctx, partitionName string) error`) and a `StubArchiver` implementation that logs/records intent and returns `nil` without any network call
- [ ] 3.4 Wire a retention job entry point that: finds eligible partitions for `scan_results` and `sim_outcomes`, detaches each, and invokes `StubArchiver.Archive` once per detached partition; support a dry-run mode that only reports `FindEligiblePartitions` output without detaching
- [ ] 3.5 Confirm `feature_distributions` and `strategy_ev_weights` are never passed to `FindEligiblePartitions` or the archiver (they are not partitioned and are out of scope for this job)
- [ ] 3.6 Add Taskfile targets `db:retention:dry-run` (report only) and `db:retention:run` (detach + stub-archive)

## 4. Testcontainers verification tests

- [ ] 4.1 Write a testcontainers Postgres test that provisions partitions for 3 distinct months (via `EnsureMonthlyPartitions`), inserts one row per month into `scan_results` and `sim_outcomes`, and asserts each row is physically stored in the expected partition (via `tableoid::regclass` or `information_schema.partitions`)
- [ ] 4.2 Extend the test to attempt an insert into a month with no provisioned partition and assert it fails with Postgres's "no partition of relation found for row" error (proves there is no silent `DEFAULT` partition)
- [ ] 4.3 Write a testcontainers test that manufactures an old (>12-month) partition, runs the retention dry-run and asserts it reports the partition without detaching it, then runs the live retention job and asserts: the partition is detached, querying the parent table for that date range returns zero rows, and querying the detached table directly still returns its original rows unchanged
- [ ] 4.4 Extend the retention test to run the live job a second time immediately after and assert it finds zero newly-eligible partitions, completes without error, and does not re-invoke the archiver for the already-detached partition

## 5. Verification and closeout

- [ ] 5.1 Testcontainers partition test suite green (partition-routing tests from 4.1/4.2 and detach/retention tests from 4.3/4.4)
- [ ] 5.2 G1: `go build ./src/go/... ./cmd/...` green
- [ ] 5.3 G6: Fable adversarial review approves the diff (DDL correctness, composite-PK/dropped-FK trade-offs, stub archiver never deletes data)
- [ ] 5.4 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time)
