# shadow-config-deployment Specification (delta)

## ADDED Requirements

### Requirement: Payload-driven decision engine over identical observations

The system SHALL provide, in package `src/go/tradingstack/shadowdeploy`, a pure function `EvaluateConfig(payload scannercfg.Payload, obs []Observation) []Decision` where `Observation` is a per-ticker snapshot lifted from a persisted `scan_results` row (at minimum: scan result id, ticker, scanned_at, regime tag, and nullable `volume_ratio`, `rsi_14`, `atr_pct`, `scanner_score`). For each observation the engine SHALL compute: **admission** per the payload's regime `hard_filter_overrides` (`volume_ratio >= volume_ratio_floor` and `atr_pct <= atr_pct_ceiling`, where an absent override or a nil feature value does not reject); a **score** equal to the weighted sum of the regime's `feature_weights` (excluding `drop_features`) over features min-max normalized to [0, 1] per feature across the observation batch, with nil features contributing 0; and **selection** — admitted, score at or above the regime's `score_threshold` (a model without a threshold selects all admitted), then global top-N by score per `global.top_n_candidates` with ties broken by ticker ascending. An observation whose regime has no `regime_models` entry SHALL be marked `no_model`: admitted by default, unscored, and never selected. The engine SHALL perform no I/O and SHALL be deterministic.

#### Scenario: A fixture batch produces the hand-computed decisions

- **WHEN** `EvaluateConfig` runs over a fixture payload and observation batch with hand-computed normalized scores, admissions, and top-N cutoff
- **THEN** each observation's admitted, score, and selected values equal the hand-computed expectations within a small numeric tolerance

#### Scenario: Hard-filter overrides reject an observation below the floor

- **WHEN** an observation's `volume_ratio` is below its regime's `volume_ratio_floor`
- **THEN** its decision is not admitted and not selected, regardless of its score

#### Scenario: An observation with no regime model is visible but never selected

- **WHEN** an observation's regime tag has no entry in the payload's `regime_models`
- **THEN** its decision is marked `no_model` and it is not selected
- **AND** it still appears in the decision output rather than being dropped

#### Scenario: Evaluation is deterministic including top-N ties

- **WHEN** `EvaluateConfig` runs twice over the same payload and observations, including two tickers with identical scores at the top-N boundary
- **THEN** both runs produce identical decisions, with the tie broken by ticker ascending

### Requirement: Same-inputs parallel evaluation of active and shadow configs

A shadow run SHALL evaluate exactly two payloads over the same observation slice: the **active** payload — parsed from the `scanner_configs` row with the latest `created_at`, or the built-in `scannercfg.DefaultPayload()` when the table is empty — and the **shadow** payload parsed from a designated `scanner_config_proposals` row. Observations SHALL be loaded once from persisted `scan_results` rows in a half-open `scanned_at` window `[from, to)` and passed to both evaluations unchanged, so both sides see byte-identical inputs. A window containing no observations SHALL cause the run to return a sentinel error and persist nothing.

#### Scenario: Both sides see identical inputs

- **WHEN** a shadow run executes over a seeded window
- **THEN** the active and shadow decision vectors cover exactly the same tickers in the same order

#### Scenario: Empty scanner_configs falls back to the default baseline

- **WHEN** a shadow run executes against a database whose `scanner_configs` table is empty
- **THEN** the active side is evaluated with the built-in default payload and the persisted run records a null active config id

#### Scenario: An empty window persists nothing

- **WHEN** a shadow run executes over a window containing no `scan_results` rows
- **THEN** the run returns a non-nil sentinel error and no `shadow_runs` row is written

### Requirement: Deterministic divergence report

The system SHALL provide a pure function `CompareDecisions(active, shadow []Decision) DivergenceReport` computing, over selected-set membership: total observations, count selected by the active config, count selected by the shadow config, count selected by both, the lists of shadow-only and active-only tickers each carrying both sides' scores, and `DivergencePct = |symmetric difference of the selected sets| / max(1, |union of the selected sets|) × 100`. Score differences on commonly-selected tickers SHALL NOT count as divergence. Output ordering SHALL be deterministic (tickers ascending).

#### Scenario: A known fixture divergence percentage

- **WHEN** `CompareDecisions` runs over fixture decisions where the active config selects {A, B, C}, and the shadow config selects {B, C, D}
- **THEN** the report has 1 shadow-only ticker (D), 1 active-only ticker (A), 2 selected by both, and `DivergencePct` of 50.0 (2 divergent out of a union of 4)

#### Scenario: Identical selections yield zero divergence

- **WHEN** both configs select exactly the same tickers
- **THEN** `DivergencePct` is 0.0 and the shadow-only and active-only lists are empty

#### Scenario: Neither config selecting anything yields zero divergence without dividing by zero

- **WHEN** neither side selects any ticker
- **THEN** `DivergencePct` is 0.0 and the function returns normally

### Requirement: Coverage-explicit outcome comparison

For each side's selected tickers, the system SHALL join to existing `sim_outcomes` rows via the observation's scan result id and compute per side: selection count, selections-with-outcomes count and coverage percentage, decided count (`pnl_pct > 0` win, `pnl_pct < 0` loss, breakeven excluded), win rate over decided, and mean `pnl_pct` over covered selections. Selections without a `sim_outcomes` row SHALL be counted in coverage only — the system SHALL never fabricate, impute, or simulate an outcome for them within this change.

#### Scenario: Outcome summary matches a seeded fixture

- **WHEN** the outcome comparison runs where the shadow side's 4 selections have 3 outcome rows (2 wins, 1 loss)
- **THEN** the shadow side reports coverage 75%, decided 3, win rate 2/3, and the fixture's mean pnl over the 3 covered selections

#### Scenario: Uncovered selections reduce coverage, not results

- **WHEN** a selected ticker has no `sim_outcomes` row in the window
- **THEN** it appears in the selection count but contributes to neither the win rate nor the mean pnl, and coverage reflects its absence

### Requirement: Persisted shadow runs and divergences with additive migration

The system SHALL persist each shadow run via GORM models: `ShadowRun` mapped to a new table `shadow_runs` (`id` UUID primary key, `created_at`, `active_config_id` UUID nullable, `proposal_id` UUID foreign key referencing `scanner_config_proposals.id`, `window_start`, `window_end`, `total_observations`, `selected_active`, `selected_shadow`, `selected_both`, `divergence_pct`, `outcome_summary_json` JSONB, `synthetic` boolean) and `ShadowDivergence` mapped to a new table `shadow_divergences` (`id` UUID primary key, `shadow_run_id` UUID foreign key referencing `shadow_runs.id`, `ticker`, `kind` text constrained to `{shadow_only, active_only}` by Go validation and a database CHECK constraint, `active_score` and `shadow_score` nullable numerics, `scan_result_id` UUID). The system SHALL provide a migration entry point `MigrateShadowDeployment(db *gorm.DB) error` that is idempotent, creates only these two tables, and SHALL NOT create, alter, or drop any other table. A `ShadowStore` interface with a GORM implementation and an in-memory fake SHALL make run orchestration testable without a database.

#### Scenario: A shadow run round-trips with its divergences

- **WHEN** a shadow run with 2 divergent tickers is persisted and re-read by id
- **THEN** the read-back run carries the written totals, divergence percentage, and outcome summary JSON, and exactly 2 `shadow_divergences` rows reference it

#### Scenario: An invalid divergence kind is rejected

- **WHEN** a `ShadowDivergence` with `kind` outside `{shadow_only, active_only}` is saved
- **THEN** the save returns a non-nil error and no row is written

#### Scenario: Migration creates exactly the two shadow tables and is idempotent

- **WHEN** `MigrateShadowDeployment` runs twice against a database that already has the `trading-stack-schema` tables, `overfitting_verdicts`, and `scanner_config_proposals`
- **THEN** the tables `shadow_runs` and `shadow_divergences` exist, the second run returns a nil error, and no other table is created, altered, or dropped by either call

### Requirement: Simulation-only and read-only-except-own-tables guard

The shadow run entry point SHALL take the operating mode explicitly and SHALL return a sentinel error, executing nothing and persisting nothing, unless the mode is Simulation (never Paper, never Margin). The package SHALL place no orders through any broker path in any mode, and its only database writes SHALL be to `shadow_runs` and `shadow_divergences` — it SHALL never insert, update, or delete rows in `scanner_configs`, `scanner_config_proposals`, `scan_results`, or `sim_outcomes`, and SHALL never change a proposal's status.

#### Scenario: Paper and Margin modes are refused

- **WHEN** a shadow run is invoked with mode Paper, and again with mode Margin
- **THEN** both invocations return the Simulation-only sentinel error and no `shadow_runs` row is written

#### Scenario: A completed run leaves configs and proposals untouched

- **WHEN** a shadow run completes successfully against a seeded database
- **THEN** the `scanner_configs` and `scanner_config_proposals` tables are byte-identical to their pre-run contents, including the proposal's status

### Requirement: Only gate-validated proposals earn shadow evidence

The shadow run SHALL refuse, with a non-nil error and no persisted run, any shadow candidate proposal whose status is `rejected_by_gate`. Proposals with status `pending_review` (the primary use) and `promoted` (post-promotion monitoring against fresh windows) SHALL be accepted.

#### Scenario: A gate-rejected proposal is refused

- **WHEN** a shadow run is requested for a proposal with status `rejected_by_gate`
- **THEN** the run returns a non-nil error and no `shadow_runs` row is written

#### Scenario: A promoted proposal may be re-shadowed

- **WHEN** a shadow run is requested for a proposal with status `promoted` over a fresh window
- **THEN** the run executes and persists normally

### Requirement: Operator commands and Taskfile targets

The system SHALL provide a command `cmd/shadow-scan/main.go` with two subcommands: `run` (executes a shadow run for a designated proposal and window; `--synthetic` mode, default true, evaluates built-in fixture payloads and observations with no database, printing the divergence report and outcome comparison; DB mode requires an explicit flag) and `report --run <uuid>` (prints a persisted run's configs compared, window, divergence report, outcome comparison with coverage, and the known limitation that replayed observations cannot surface tickers the active pipeline never persisted). The command SHALL exit non-zero only on an internal error — never merely because divergence was high. The system SHALL provide Taskfile targets `task scanner:shadow`, `task scanner:shadow-report`, and `task test:shadow-deployment` (runs `go test` for the `shadowdeploy` package).

#### Scenario: Synthetic self-check runs with no external input

- **WHEN** an operator runs `task scanner:shadow` with no database available
- **THEN** the command runs in `--synthetic` mode, prints the fixture divergence report and outcome comparison, and exits zero

#### Scenario: High divergence does not fail the command

- **WHEN** a shadow run completes with a divergence percentage of 100
- **THEN** the command prints the report and exits zero

#### Scenario: Test target runs the shadow suite

- **WHEN** an operator runs `task test:shadow-deployment`
- **THEN** the `shadowdeploy` package tests execute and the command exits zero when all pass, non-zero when any fails
