# Trading Stack Schema (v4 foundation)

## Why

The v4 trading stack (scanner → simulator → optimizers → fidelity checker → EV tracker) needs a persistent, faithful home for its scan, outcome, fidelity, and EV data before any of those components can be built. This change lays that foundation: six new PostgreSQL tables as Go domain models, migrated and round-trip tested, with zero impact on the existing playground tables.

## What Changes

- Add a new Go package for the v4 trading-stack domain models, isolated from the existing `backtester-api` playground models.
- Add six GORM models faithful to the architecture doc SQL: `scan_results`, `sim_outcomes`, `simulator_fidelity`, `strategy_ev_weights`, `scanner_configs`, `feature_distributions`.
- Enforce the `data_as_of <= scanned_at` invariant on `scan_results` (Go validation + DB CHECK constraint).
- Enforce the `sim_outcomes.exit_reason` enum (`stop | target | timeout | signal_exit`) and the foreign key `sim_outcomes.scan_result_id → scan_results.id`.
- Store `scanner_configs.config_json` as JSONB and support rollback (fetch a prior config verbatim by ID).
- Add a dedicated migration entry point (`MigrateTradingStack`) that creates only the six new tables and their constraints — the existing playground migration path is untouched.
- Add testcontainers round-trip tests (insert → read back, plus invariant/enum/FK violation cases) and a `task test:trading-stack` Taskfile target to run them.
- No changes to any existing playground table, model, or migration. **Not BREAKING.**

## Capabilities

### New Capabilities

- `trading-stack-schema`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/` (models, migration, errors, tests). No existing files modified other than `taskfile.yml` (adds `test:trading-stack`).
- New dependency surface only within existing modules already vendored: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/google/uuid`, `github.com/testcontainers/testcontainers-go`. No new external infrastructure.
- Downstream v4 changes depend on this foundation: `db-partitioning-retention` (partitions these tables), `net-ev-cost-model`, `ev-tracker`, `fidelity-checker`, `optimizer-validation-pipeline`, `scanner-l1-l2`, and `crowding-detection` all read/write these models. This change must land and archive before those can build against the models.
- Verification is local-only (testcontainers Postgres). No prod DB, no live/paper broker, no infra provisioning.
