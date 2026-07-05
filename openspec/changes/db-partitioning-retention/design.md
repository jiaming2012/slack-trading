## Context

`trading-stack-schema` introduces six new v4 tables (`scan_results`, `sim_outcomes`, `simulator_fidelity`, `strategy_ev_weights`, `scanner_configs`, `feature_distributions`) as plain GORM-managed tables. Gap analysis Finding 12 (`todo/trading-stack-gap-analysis.md`) flags that `scan_results` and `sim_outcomes` will grow linearly and indefinitely under daily multi-strategy scanning, with no partitioning or retention policy defined, and recommends monthly native PostgreSQL partitioning plus a 12-month cold-storage retention policy, while keeping `feature_distributions` and `strategy_ev_weights` indefinitely (small, high analytical value). No existing/production table is affected — this is purely additive to the not-yet-merged v4 schema.

This design assumes `trading-stack-schema` ships the column layout exactly as specified in `todo/full-trading-stack-architecture.md`'s Database Schema section (`scan_results.scanned_at`, `sim_outcomes.simulated_at`, both keyed by UUID `id`, with `sim_outcomes.scan_result_id` referencing `scan_results.id`). If that change lands with different column names/types, the DDL and Go structs here must be updated to match before this change can be implemented — see Dependency Ordering below.

## Goals / Non-Goals

**Goals:**
- Native PostgreSQL monthly RANGE partitioning for `scan_results` (on `scanned_at`) and `sim_outcomes` (on `simulated_at`) only.
- Idempotent, loud-failure-on-gap partition provisioning (no silent catch-all `DEFAULT` partition).
- A 12-month retention job that detaches old partitions and hands them to a cold-storage archive hook.
- A stub `ColdStorageArchiver` implementation — proves the hook shape without shipping a real integration.
- `feature_distributions` and `strategy_ev_weights` (and the other two v4 tables) remain plain, unpartitioned, retained indefinitely, and are never touched by the retention job.
- A testcontainers proof that (a) rows route to the correct monthly partition and (b) detaching an old partition works without data loss.
- Operator-visible Taskfile entry points for both provisioning and retention (no scheduler/cron).

**Non-Goals:**
- Touching any pre-existing production table, or any v4 table other than `scan_results`/`sim_outcomes`.
- A real cold-storage backend (S3/GCS/etc.) — the archiver is a stub for the life of this change. Wiring an actual upload target is a future, separate change.
- Any scheduler, cron job, or new external infrastructure to run provisioning/retention automatically — overnight constraints prohibit new infra; both jobs are manually operator-invoked via Taskfile in this change.
- Preserving a database-level foreign key from `sim_outcomes.scan_result_id` to `scan_results.id` — Postgres requires a partitioned table's referenced unique/PK constraint to include the partition key, which a bare `id` no longer satisfies once `scan_results` is partitioned. This relationship becomes an application-layer concern.
- Partitioning `simulator_fidelity` or `scanner_configs` — the gap analysis and this change's brief only call out `scan_results`/`sim_outcomes`.
- Choosing or building a permanent partition-horizon-monitoring alert — out of scope; the observability project (per CLAUDE.md) can pick this up separately if the operator wants alerting on "partitions about to run out."

## Decisions

**1. Raw SQL migration, not GORM AutoMigrate.** GORM cannot express `PARTITION BY RANGE` or per-partition `FOR VALUES FROM/TO` DDL. This change adds a new SQL file (e.g. `src/go/tradingstack/db/partitioning.sql`) and registers it in `infra/migrate.py`'s `SQL_FILES` list, following the project's existing idempotent-SQL-file migration pattern (`IF NOT EXISTS` / re-runnable). The file converts the plain tables `trading-stack-schema` creates into partitioned parents plus an initial partition for the current month.

**2. Composite primary keys.** `scan_results` PK becomes `(id, scanned_at)`; `sim_outcomes` PK becomes `(id, simulated_at)`. This is a hard Postgres requirement for native partitioning (any unique/PK constraint on a partitioned table must include all partition-key columns), not a stylistic choice. `id` remains application-generated UUID and unique in practice; code that looked up rows by `id` alone still works for reads, it just can no longer be the sole target of a foreign key from another table.

**3. Drop the DB-level FK, keep the column.** `sim_outcomes.scan_result_id` remains as a plain UUID column; the `REFERENCES scan_results(id)` constraint is dropped. Enforcement of "every `scan_result_id` should exist in `scan_results`" moves to the service/application layer (e.g. the code path that writes `sim_outcomes` looks up the `scan_results` row first). This is the direct, unavoidable cost of partitioning `scan_results` and is called out as **BREAKING** in the proposal.

**4. Partition provisioning lives in Go, not only SQL.** Because partition boundaries depend on "now" and a rolling horizon, provisioning is a Go routine (`src/go/tradingstack/partitioning`) that builds and executes `CREATE TABLE IF NOT EXISTS <table>_yYYYY_mMM PARTITION OF <table> FOR VALUES FROM (...) TO (...)` statements via the existing GORM `*gorm.DB` connection, rather than a static SQL file (which cannot express "the next 3 months from whenever this runs"). Default horizon: current month + 3 months ahead, configurable via a function parameter.

**5. No `DEFAULT` partition, by design.** A `DEFAULT` partition would silently absorb any row whose timestamp falls outside all explicit ranges — masking a provisioning gap. Instead, an insert into an unprovisioned month fails loudly with Postgres's native "no partition of relation found for row" error, which is easy to alert on and impossible to miss silently. The trade-off: the operator (or the app's startup path) must run provisioning ahead of need; this change does not add a scheduler to guarantee that (see Non-Goals).

**6. Retention as detach-first, archive-second.** `src/go/tradingstack/retention` implements `FindEligiblePartitions` (partitions whose full range ends before `now() - 12mo`, both bounds configurable), `DetachPartition` (issues `ALTER TABLE ... DETACH PARTITION`), and a `ColdStorageArchiver` interface invoked once per detached partition. Detach happens unconditionally and immediately once a partition is eligible; archiving is a separate, swappable step. The stub `StubArchiver` logs the partition name and returns success — it never drops or deletes the detached table, so no data is ever destroyed by this change. Actually dropping/uploading detached partitions is deliberately left to a future change with a real cold-storage target.

**7. Dry-run mode.** Both the report path (`FindEligiblePartitions`) and the action path (`DetachPartition` + archive) are separated so a `--dry-run`-style entry point can report without mutating, and the live entry point is safe to re-run (idempotent: a partition already detached is no longer "attached" so it's never found again by `FindEligiblePartitions`, which only inspects the parent table's current partition set).

**8. Taskfile wiring.** Three new targets, matching the project convention that every operator-visible command gets a Taskfile entry in the same change:
   - `db:partition:ensure` — runs the provisioning routine against the configured database.
   - `db:retention:dry-run` — runs `FindEligiblePartitions` and prints the report.
   - `db:retention:run` — runs detach + stub-archive for all eligible partitions.

## Data Flow

1. `trading-stack-schema` creates `scan_results` / `sim_outcomes` as plain tables (upstream dependency, not part of this change).
2. This change's migration SQL converts them to partitioned parents with a composite PK and creates the current month's partition.
3. `task db:partition:ensure` (or an equivalent call made at server startup by a future change, out of scope here) creates partitions for the current + horizon months before the scanner/simulator ever writes into a new month.
4. Scanner/simulator code (owned by other changes) inserts into `scan_results`/`sim_outcomes` as before — partition routing is transparent to writers as long as the target month's partition exists.
5. Periodically, an operator runs `task db:retention:dry-run` to see what would be archived, then `task db:retention:run` to detach eligible partitions and invoke the stub archiver.

## Dependency Ordering

This change depends on `trading-stack-schema` and cannot be implemented (task group 1 onward) until that change's `scan_results`/`sim_outcomes` GORM models exist in the tree with the column names assumed above. If `trading-stack-schema` lands with different column names/types, this change's DDL (task 1) and Go structs (tasks 2–3) must be updated to match as part of implementation, not deferred.

## Deferred validation

None — this is a Tier A change (full completion expected overnight). All verification gates below are achievable overnight with local Docker/testcontainers Postgres; no live data, prod DB, or external infra is required. The only thing intentionally *not* built (not merely deferred) is a real cold-storage backend — see Non-Goals; that is a distinct future change, not a validation gap in this one.

## Verification Gates

- **Testcontainers partition tests**: prove (a) rows route to the correct monthly partition for both tables, (b) an insert into an unprovisioned month fails loudly (no DEFAULT partition), (c) an old partition can be detached without data loss and disappears from parent-table queries while remaining directly queryable, and (d) the retention job is idempotent across repeated runs.
- **G1**: `go build ./src/go/... ./cmd/...` green.
- **G6**: Fable adversarial review of the diff (partitioning DDL correctness, composite-PK fallout, dropped-FK trade-off, stub archiver never deleting data) before commit.

## Risks / Trade-offs

- Composite PKs mean any future code that assumed a single-column `id` PK/unique constraint on these two tables (e.g. for a new FK from a table not yet designed) will hit the same limitation this change already accepts for `sim_outcomes` → `scan_results`.
- Dropping the DB-level FK shifts a correctness guarantee to application code; a bug in the write path could insert an orphaned `sim_outcomes` row with no database-level safety net. Mitigated by keeping the column and documenting the expectation clearly for whoever implements the simulator write path.
- No scheduler means partition provisioning is a manual/operational responsibility; forgetting to run it before a new month starts causes loud insert failures (chosen deliberately over silent data loss via a DEFAULT partition) rather than a graceful degradation.
- Detach-without-drop means disk usage is not reduced by the retention job in this change — only query-plan/scan surface is reduced. Actual space reclamation waits for a future change that drops or truly uploads detached partitions.
