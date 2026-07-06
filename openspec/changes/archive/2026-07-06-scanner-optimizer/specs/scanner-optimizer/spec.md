# scanner-optimizer Specification (delta)

## ADDED Requirements

### Requirement: Scanner config payload type with verbatim JSON fidelity

The system SHALL provide, in package `src/go/tradingstack/scannercfg`, a `Payload` type faithful to the architecture doc's Scanner Config Payload: a `version` string, a `regime_models` map keyed by regime tag whose values carry `feature_weights` (map of feature name to non-negative weight), `drop_features` (list of feature names), `hard_filter_overrides` (`volume_ratio_floor`, `atr_pct_ceiling`, each optional), and an optional `score_threshold`, plus a `global` section with `top_n_candidates` and `min_labeled_samples`. The package SHALL provide `Parse` and `Marshal` functions such that parsing a payload JSON document and re-marshalling it preserves every key and value (semantic round-trip), and a `Validate` method that rejects negative or non-finite feature weights and a `score_threshold` outside `[0, 1]`. Payload JSON SHALL be storable unchanged in the existing `scanner_configs.config_json` JSONB column.

#### Scenario: The architecture-doc example payload round-trips

- **WHEN** the architecture doc's example payload JSON (regime models for `trending` and `high_vol` plus a `global` section) is parsed and re-marshalled
- **THEN** the result deserializes to the same keys and values as the original document

#### Scenario: An invalid payload is rejected

- **WHEN** a payload with a negative feature weight or a `score_threshold` of 1.5 is validated
- **THEN** `Validate` returns a non-nil error identifying the offending field

### Requirement: Tuning consumes only validation-pipeline output

The scanner optimizer SHALL accept its training data exclusively as `[]optvalidation.WeightedTrainingRow` — the clean weighted dataset produced by the optimizer-validation-pipeline — and SHALL expose no entry point that tunes from raw `scan_results`/`sim_outcomes` rows that have not passed through `optvalidation.Run`. The DB-backed run SHALL load joined rows and reference tables via a loader into `optvalidation.Input`, run the pipeline, and tune only on the pipeline's `Result.Clean`.

#### Scenario: The DB-backed run threads rows through the validation pipeline

- **WHEN** a DB-backed optimizer run executes against a database seeded with joined rows that include one timestamp violation and one low-regime-confidence row
- **THEN** the tuner's input excludes both excluded rows, matching the validation pipeline's documented filtering

#### Scenario: Loader joins only rows with outcomes inside the window

- **WHEN** the loader runs with a half-open `[from, to)` window over a database containing a scan result with no sim outcome, a joined pair inside the window, and a joined pair outside the window
- **THEN** exactly the one inside-window joined pair is loaded as a `TrainingRow`

### Requirement: Deterministic per-regime tuning method (v1)

The system SHALL derive, per regime group of the weighted training rows (grouped by `RegimeTag`, excluding rows with an empty tag), a proposed `RegimeModel` as follows. Labels: a row with `PnlPct > 0` is a win, `PnlPct < 0` a loss, and `PnlPct == 0` is excluded from label-conditioned statistics; all statistics are weighted by each row's `EVWeight`. Feature weights over `rsi_14`, `volume_ratio`, and `atr_pct`: per-feature effect size `|weightedMean(win) − weightedMean(loss)| / pooledWeightedStdDev`, normalized so proposed weights sum to 1; a feature with zero pooled dispersion gets weight 0, and if every effect size is 0 the baseline's feature weights are carried forward. Hard-filter overrides: `volume_ratio_floor` equal to the EV-weighted 25th percentile of winners' `volume_ratio`, and `atr_pct_ceiling` equal to the EV-weighted 75th percentile of winners' `atr_pct`. `score_threshold` SHALL be carried forward from the baseline payload unchanged (not tuned in this change). A regime with fewer decided rows than the payload's `min_labeled_samples` SHALL produce no derived model, carrying the baseline's model forward instead. The derivation SHALL be deterministic: identical inputs produce an identical proposed payload.

#### Scenario: A fixture regime produces the hand-computed weights and overrides

- **WHEN** the tuner runs over a fixture regime group with known EV-weighted win/loss feature means, dispersions, and winner percentiles
- **THEN** the proposed feature weights equal the hand-computed normalized effect sizes and the proposed `volume_ratio_floor` and `atr_pct_ceiling` equal the hand-computed weighted percentiles, within a small numeric tolerance

#### Scenario: A thin regime is skipped and the baseline carried forward

- **WHEN** the tuner runs over a regime group with fewer decided rows than `min_labeled_samples`
- **THEN** the proposed payload's model for that regime equals the baseline payload's model for that regime

#### Scenario: Derivation is deterministic

- **WHEN** the tuner runs twice over the same weighted rows and baseline payload
- **THEN** both runs produce identical proposed payloads

### Requirement: Walk-forward evidence submitted to the overfitting gate

Every optimizer run SHALL build `overfitting.Evidence` for its proposed payload and submit it to the `overfitting-countermeasures` gate before persisting a proposal. Evidence construction SHALL order the weighted rows chronologically by `ScannedAt`, hold out the final 20% as the out-of-sample segment, split the remaining in-sample segment into chronological walk-forward folds (at least the gate config's minimum fold count), and re-derive the proposal parameters per fold (each fold's `Params` carrying that fold's derived `volume_ratio_floor`, `atr_pct_ceiling`, and feature weights). The per-sample performance measure SHALL be `EVWeight × PnlPct` over rows the proposed overrides would have admitted (`volume_ratio >= floor AND atr_pct <= ceiling`). `TrialsCount` SHALL equal the number of candidate parameterizations evaluated (1 for this direct derivation). `BaselineParams` SHALL come from the current active payload, or from the built-in default payload when `scanner_configs` is empty. The gate verdict SHALL be persisted via the `overfitting` verdict store for every run — pass or fail.

#### Scenario: Evidence folds are chronological and lookahead-free

- **WHEN** an optimizer run builds evidence over a fixture with known `ScannedAt` ordering
- **THEN** every fold's `TestStart` is at or after its `TrainEnd` and the out-of-sample window starts after every fold's windows

#### Scenario: A failing verdict is still persisted

- **WHEN** an optimizer run's evidence fails the gate (for example, too few decided samples)
- **THEN** an `overfitting_verdicts` row with `passed` false is persisted and referenced by the proposal row

### Requirement: Proposals persisted for operator review, never auto-applied

The system SHALL persist every optimizer run's proposal to a new table `scanner_config_proposals` via a GORM model with fields: `id` (UUID primary key), `created_at`, `optimizer_run_id` (text), `proposed_config_json` (JSONB, the full payload), `evidence_json` (JSONB), `verdict_id` (UUID foreign key referencing `overfitting_verdicts.id`), and `status` (text, constrained by Go validation and a database CHECK constraint to `{pending_review, rejected_by_gate, promoted}`). Status at insert SHALL be `pending_review` when the gate verdict passed and `rejected_by_gate` when it failed. The system SHALL provide a migration entry point `MigrateScannerOptimizer(db *gorm.DB) error` that is idempotent, creates only this table, and SHALL NOT create, alter, or drop any `trading-stack-schema`, `overfitting_verdicts`, or playground table. No code path in the optimizer other than the operator promote command SHALL insert, update, or delete rows in `scanner_configs`.

#### Scenario: A gate-passing run lands in the review queue without touching the active config

- **WHEN** an optimizer run completes with a passing verdict against a database whose `scanner_configs` table has N rows
- **THEN** one `scanner_config_proposals` row exists with status `pending_review`
- **AND** `scanner_configs` still has exactly N rows

#### Scenario: A gate-failing run is persisted as rejected

- **WHEN** an optimizer run completes with a failing verdict
- **THEN** one `scanner_config_proposals` row exists with status `rejected_by_gate` referencing the failing verdict

#### Scenario: Migration creates exactly one table and is idempotent

- **WHEN** `MigrateScannerOptimizer` runs twice against a database that already has the `trading-stack-schema` tables and `overfitting_verdicts`
- **THEN** the table `scanner_config_proposals` exists, the second run returns a nil error, and no other table is created, altered, or dropped by either call

### Requirement: Operator promotion command with refusal rules

The system SHALL provide a promote operation, invoked only via the operator CLI subcommand, that: refuses (with a non-nil error and no writes) any proposal whose status is not `pending_review`; otherwise, in a single transaction, inserts a new `scanner_configs` row whose `config_json` equals the proposal's `proposed_config_json` verbatim and whose `optimizer_run_id` is carried from the proposal, and updates the proposal's status to `promoted`. Rolling back a bad promotion uses the existing `scanner_configs` rollback-by-id mechanism; this change SHALL add no other apply path.

#### Scenario: Promoting a pending proposal creates the config version and flips status

- **WHEN** the operator promotes a `pending_review` proposal
- **THEN** a new `scanner_configs` row exists whose `config_json` equals the proposal payload verbatim
- **AND** the proposal's status is `promoted`

#### Scenario: A gate-rejected proposal cannot be promoted

- **WHEN** the operator attempts to promote a proposal with status `rejected_by_gate`
- **THEN** the promote operation returns a non-nil error
- **AND** no `scanner_configs` row is written and the proposal's status is unchanged

#### Scenario: A proposal cannot be promoted twice

- **WHEN** the operator attempts to promote a proposal that already has status `promoted`
- **THEN** the promote operation returns a non-nil error and writes nothing

### Requirement: Operator commands and Taskfile targets

The system SHALL provide a command `cmd/scanner-optimizer/main.go` with three subcommands: `run` (executes an optimizer cycle; `--synthetic` mode, default true, runs against built-in fixtures with no database and prints the proposed payload, per-check gate results, and the would-be status; DB mode requires an explicit flag), `list` (prints persisted proposals with id, created_at, status, verdict pass/fail, and an evidence summary), and `promote --id <uuid>` (invokes the promote operation). The command SHALL exit non-zero only on an internal error — never merely because the gate rejected a proposal. The system SHALL provide Taskfile targets `task scanner:optimize`, `task scanner:proposals`, `task scanner:promote`, and `task test:scanner-optimizer` (runs `go test` for the `scannercfg` and `scanneropt` packages).

#### Scenario: Synthetic self-check runs with no external input

- **WHEN** an operator runs `task scanner:optimize` with no database available
- **THEN** the command runs in `--synthetic` mode, prints the proposed payload and gate check results, and exits zero

#### Scenario: Test target runs the optimizer suites

- **WHEN** an operator runs `task test:scanner-optimizer`
- **THEN** the `scannercfg` and `scanneropt` package tests execute and the command exits zero when all pass, non-zero when any fails
