## Why

`scan_results` and `sim_outcomes` (the new v4 scanner/simulator tables) will grow linearly and indefinitely under daily multi-strategy scanning, and today nothing bounds that growth or protects query performance as it happens.

## What Changes

- Convert `scan_results` and `sim_outcomes` (created by `trading-stack-schema`) to native PostgreSQL monthly RANGE partitioning, keyed on `scanned_at` / `simulated_at` respectively. **BREAKING**: the primary key on both tables becomes composite (`id, scanned_at` / `id, simulated_at`) instead of `id` alone — a Postgres requirement for partitioned tables, not a style choice.
- **BREAKING**: drop the database-level foreign key from `sim_outcomes.scan_result_id` to `scan_results.id`. Native partitioning requires a referenced unique/PK constraint that includes the partition key, which `id` alone can no longer satisfy once `scan_results` is partitioned. The relationship is enforced at the application/service layer instead.
- Add idempotent, automatic monthly partition provisioning (default: current month + 3-month horizon). No catch-all `DEFAULT` partition is created — an insert targeting an unprovisioned month fails loudly rather than silently mis-filing data.
- Add a 12-month retention/archival job: detaches (`ALTER TABLE ... DETACH PARTITION`) any partition whose entire date range is older than the retention window, then hands the detached partition to a `ColdStorageArchiver` hook. The shipped implementation is a stub that records intent only — no real upload, no new external infrastructure this change.
- `feature_distributions`, `strategy_ev_weights`, `simulator_fidelity`, and `scanner_configs` remain ordinary, unpartitioned tables and are explicitly excluded from the retention job — kept indefinitely.
- No existing/production table is touched by this change; only the two new v4 tables are affected.
- New Taskfile targets: `db:partition:ensure`, `db:retention:dry-run`, `db:retention:run`.

## Capabilities

### New Capabilities
- `trading-data-partitioning`: native monthly PostgreSQL range partitioning of `scan_results` and `sim_outcomes`, with idempotent, loud-failure-on-gap partition provisioning.
- `trading-data-archival`: 12-month retention policy that detaches aged partitions and invokes a stubbed cold-storage archive hook, while leaving small analytical tables (`feature_distributions`, `strategy_ev_weights`) retained indefinitely and untouched.

### Modified Capabilities
(none — this repo has no existing specs; everything above is new)

## Impact

- New code: `src/go/tradingstack/partitioning/` (provisioning), `src/go/tradingstack/retention/` (detach + archive hook + stub archiver).
- New migration SQL registered in `infra/migrate.py`'s `SQL_FILES` list, following the existing idempotent-SQL-file pattern (no changes to `src/go/backtester-api/db/init.sql` or `infra/analytics-schema.sql` themselves).
- `taskfile.yml` gains `db:partition:ensure`, `db:retention:dry-run`, `db:retention:run`.
- **Depends on `trading-stack-schema`**: this change consumes the `scan_results` / `sim_outcomes` GORM models and column definitions that change creates. It cannot be implemented or merged until `trading-stack-schema` lands; if that change's column names or types differ from `todo/full-trading-stack-architecture.md`, this change's DDL and Go structs must be adjusted to match before merge.
- No changes to any pre-existing production table, no new external infrastructure (cold storage is a stub), no cron/scheduler introduced — provisioning and retention are operator-invoked via Taskfile, matching the overnight-run prohibition on new infra.
