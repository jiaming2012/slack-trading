# Tasks — trading-stack-schema

## 1. Package scaffold

- [ ] 1.1 Create `src/go/tradingstack/` package with `ids.go` (shared `BeforeCreate` UUID generation for zero ids) and `errors.go` (`ErrScanDataAsOfAfterScannedAt`, `ErrInvalidExitReason`).

## 2. Models (faithful to architecture SQL)

- [ ] 2.1 `scan_result.go` — `ScanResult` model, `TableName() "scan_results"`, all 14 fields with correct GORM tags (pointers for nullable), `BeforeSave` enforcing `data_as_of <= scanned_at`.
- [ ] 2.2 `sim_outcome.go` — `SimOutcome` model, `TableName() "sim_outcomes"`, `scan_result_id` FK field, `BeforeSave` validating `exit_reason ∈ {stop,target,timeout,signal_exit}`.
- [ ] 2.3 `simulator_fidelity.go` — `SimulatorFidelity` model, `TableName() "simulator_fidelity"` (singular), `within_tolerance` bool, drift fields.
- [ ] 2.4 `strategy_ev_weights.go` — `StrategyEvWeight` model, `TableName() "strategy_ev_weights"`, `ev_30d/ev_90d/ev_slope/ev_weight`.
- [ ] 2.5 `scanner_config.go` — `ScannerConfig` model, `TableName() "scanner_configs"`, `config_json` as `[]byte` with `type:jsonb`, plus `FetchScannerConfigByID` rollback helper.
- [ ] 2.6 `feature_distribution.go` — `FeatureDistribution` model, `TableName() "feature_distributions"`, surrogate UUID `id` primary key (documented deviation).

## 3. Migration

- [ ] 3.1 `migrate.go` — `MigrateTradingStack(db *gorm.DB) error`: `AutoMigrate` the six models, then idempotently add the `data_as_of <= scanned_at` CHECK, the `exit_reason` enum CHECK, and the `scan_result_id → scan_results.id` foreign key. Confirm it does not touch playground tables.

## 4. Tests (testcontainers round-trip)

- [ ] 4.1 `tradingstack_test.go` — testcontainers Postgres helper + test asserting `MigrateTradingStack` creates exactly the six named tables and no playground table.
- [ ] 4.2 Round-trip write→read tests for all six models (including JSONB verbatim, negative `ev_slope`, boolean `within_tolerance`, nullable NULLs).
- [ ] 4.3 Negative tests: `data_as_of > scanned_at` rejected; unknown `exit_reason` rejected; orphan `scan_result_id` FK violation rejected; `scanner_configs` rollback-by-id fetches the prior version.

## 5. Taskfile wiring

- [ ] 5.1 Add `test:trading-stack` target to `taskfile.yml` running the `tradingstack` package tests (`go test -count=1 ./src/go/tradingstack/...`), exiting non-zero on any failure.

## 6. Verification and closeout

- [ ] 6.1 Testcontainers round-trip tests pass: `task test:trading-stack` green (migration, six round-trips, three negative/constraint cases, rollback).
- [ ] 6.2 G1 — `go build ./src/go/... ./cmd/...` green.
- [ ] 6.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [ ] 6.4 G6 — Fable adversarial review approves the diff (schema faithful to architecture SQL, no leakage into playground tables) before commit.
- [ ] 6.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
