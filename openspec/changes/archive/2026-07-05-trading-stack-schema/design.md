# Design — trading-stack-schema

## Approach

Greenfield GORM models for the six v4 trading-stack tables, kept faithful to the verbatim SQL in `todo/full-trading-stack-architecture.md` (Database Schema section). The models live in one new, self-contained package with its own migration entry point, so nothing in the existing `backtester-api` playground path is touched. Verification is a testcontainers Postgres round-trip suite: write a row, read it back, and assert equality — plus negative tests for the three enforced constraints (data_as_of invariant, exit_reason enum, scan_result_id foreign key).

## Package and file layout

New package `src/go/tradingstack/` (package name `tradingstack`, import path `github.com/jiaming2012/slack-trading/src/go/tradingstack`):

- `scan_result.go` — `ScanResult` model + `TableName() "scan_results"` + `BeforeSave` hook enforcing `data_as_of <= scanned_at`.
- `sim_outcome.go` — `SimOutcome` model + `TableName() "sim_outcomes"` + `BeforeSave` hook validating `exit_reason` against the allowed set.
- `simulator_fidelity.go` — `SimulatorFidelity` model + `TableName() "simulator_fidelity"` (singular).
- `strategy_ev_weights.go` — `StrategyEvWeight` model + `TableName() "strategy_ev_weights"`.
- `scanner_config.go` — `ScannerConfig` model + `TableName() "scanner_configs"` + a `FetchScannerConfigByID(db, id)` helper for rollback.
- `feature_distribution.go` — `FeatureDistribution` model + `TableName() "feature_distributions"`.
- `ids.go` — shared `BeforeCreate` UUID generation (set `id` when zero) reused by every model.
- `errors.go` — sentinel errors: `ErrScanDataAsOfAfterScannedAt`, `ErrInvalidExitReason`.
- `migrate.go` — `MigrateTradingStack(db *gorm.DB) error`: `AutoMigrate` the six models, then add the CHECK constraints (`data_as_of <= scanned_at`; `exit_reason IN (...)`) and the `scan_result_id` foreign key idempotently.
- `tradingstack_test.go` — testcontainers Postgres round-trip + negative tests.

Because `TableName()` is overridden on every model, GORM pluralization (which would wrongly yield `simulator_fidelities`) is bypassed and the exact architecture-doc names are guaranteed.

## Field mapping decisions

- **Primary keys**: SQL declares `UUID PRIMARY KEY`. Models use `uuid.UUID` with a shared `BeforeCreate` hook that assigns `uuid.New()` when the id is zero — no dependency on the `uuid-ossp` Postgres extension, keeping the testcontainers path self-contained and deterministic.
- **Nullable numerics**: the SQL leaves most `NUMERIC` columns nullable. To round-trip NULL faithfully, nullable numeric/text columns use pointer types (`*float64`, `*string`, `*time.Time`); `NOT NULL` columns (`scanned_at`, `ticker`, `simulated_at`) use value types with `not null` tags.
- **JSONB**: `scanner_configs.config_json` is stored with `gorm:"type:jsonb"` over a `[]byte` (raw JSON). Rollback compares by deserializing both sides to a map so key/value equality is asserted regardless of byte ordering.
- **feature_distributions surrogate key**: the source SQL has no primary key. GORM requires one for CRUD, so the model adds a surrogate `id uuid.UUID` primary key. This is the single intentional deviation from the verbatim SQL and is called out in the spec.
- **Booleans / integers**: `within_tolerance` → `bool`; `hold_days` → `*int`.

## Data flow

Downstream v4 components (`ev-tracker`, `fidelity-checker`, `net-ev-cost-model`, `optimizer-validation-pipeline`, `scanner-l1-l2`, `crowding-detection`) import `tradingstack` and read/write these models through their own GORM sessions. This change delivers only the schema, models, migration, and tests — no business logic that populates them.

`MigrateTradingStack` is deliberately NOT wired into `dbutils.InitPostgres`. Callers (and the test suite) invoke it explicitly. This preserves the hard rule of not touching the existing playground migration path and lets the v4 stack own its own migration lifecycle.

## Out of scope

- Any logic that computes or populates scan results, outcomes, fidelity, or EV weights (owned by later v4 changes).
- Partitioning / retention DDL for these tables (owned by `db-partitioning-retention`, which chains on this change).
- Wiring `MigrateTradingStack` into server startup / `cmd/main.go`.
- Any change to existing playground tables or `dbutils`.
- Python-side stubs or clients.

## Dependency ordering within the batch

This is the foundation card. It has no upstream dependency inside the v4 batch and can rebase onto either the post-refactor tree or HEAD if the refactor track halts (greenfield, low collision surface). The following batch changes depend on the models this change lands and must be authored after it merges: `db-partitioning-retention`, `net-ev-cost-model`, `ev-tracker`, `fidelity-checker`, `optimizer-validation-pipeline`, `scanner-l1-l2`, `crowding-detection`.

## Verification gates

- **Testcontainers round-trip tests** — `go test` in `src/go/tradingstack` (via `task test:trading-stack`): migration creates exactly the six tables; each model round-trips write→read; negative tests confirm the data_as_of invariant, the exit_reason enum, and the scan_result_id foreign key each reject bad rows.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package compiles, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected).
- **G6** — Fable adversarial review of the diff before commit (schema faithfulness to the architecture SQL, no leakage into playground tables).

## Deferred validation

None. All gates for this Tier A change run fully overnight against a local testcontainers Postgres — no external dependency is deferred.
