# overfitting-countermeasures Specification (delta)

## ADDED Requirements

### Requirement: Shared evidence contract for optimizer proposals

The system SHALL define, in package `src/go/tradingstack/overfitting`, an `Evidence` type that an optimizer supplies when submitting a proposal to the gate, carrying at minimum: `ProposalID` (UUID), `ProposalKind` (exactly one of `scanner` or `strategy`), `SampleSize` (count of decided labeled samples the proposal was derived from), `TrialsCount` (count of candidate parameterizations evaluated during the search; a direct deterministic derivation counts as 1), an in-sample and an out-of-sample `Performance` record (each with `WindowStart`, `WindowEnd`, `Mean`, `StdDev`, and `SampleSize` of the per-sample performance measure), a slice of walk-forward `FoldResult` records (each with `Index`, `TrainStart`, `TrainEnd`, `TestStart`, `TestEnd`, per-fold fitted `Params` as `map[string]float64`, and `TestMetric`), and `ProposedParams` plus `BaselineParams` as `map[string]float64`. The gate SHALL operate only on this in-memory value and SHALL perform no database, network, or clock I/O.

#### Scenario: Evidence with an unknown proposal kind is rejected

- **WHEN** the gate runs over an `Evidence` whose `ProposalKind` is neither `scanner` nor `strategy`
- **THEN** the gate returns an error and produces no verdict

#### Scenario: A well-formed evidence value produces a verdict without I/O

- **WHEN** the gate runs over a well-formed synthetic `Evidence` fixture with no database connection available
- **THEN** it returns a `Verdict` containing exactly four check results

### Requirement: Minimum-sample gate

The system SHALL provide a minimum-sample check that passes only when `Evidence.SampleSize` is greater than or equal to a configurable minimum (default 500, per the architecture doc's `min_labeled_samples`) AND the out-of-sample `Performance.SampleSize` is greater than or equal to a configurable minimum (default 50). Values exactly at a threshold SHALL pass. The check result SHALL report both observed counts and both thresholds.

#### Scenario: Sufficient samples pass

- **WHEN** the check runs with defaults over evidence with `SampleSize` 500 and out-of-sample `SampleSize` 50
- **THEN** the check passes (values exactly at the thresholds are sufficient)

#### Scenario: Too few labeled samples fail

- **WHEN** the check runs with defaults over evidence with `SampleSize` 499 and out-of-sample `SampleSize` 200
- **THEN** the check fails and its result identifies the total-sample condition

#### Scenario: Too few out-of-sample rows fail

- **WHEN** the check runs with defaults over evidence with `SampleSize` 2000 and out-of-sample `SampleSize` 49
- **THEN** the check fails and its result identifies the out-of-sample condition

### Requirement: Walk-forward out-of-sample validation check

The system SHALL provide a walk-forward check that passes only when ALL of the following hold: the evidence carries at least a configurable minimum number of folds (default 3); the folds' `Index` values are strictly increasing; every fold's `TestStart` is greater than or equal to its `TrainEnd` (no lookahead); the folds' test windows are in chronological order; `InSample.Mean` is strictly positive; `OutOfSample.Mean` is strictly positive; and `OutOfSample.Mean >= retentionFraction × InSample.Mean` for a configurable retention fraction (default 0.5). A failing geometry condition and a failing performance condition SHALL each be identified in the check result.

#### Scenario: Healthy folds with retained out-of-sample performance pass

- **WHEN** the check runs with defaults over evidence with 4 chronologically ordered, non-lookahead folds, `InSample.Mean` 0.02 and `OutOfSample.Mean` 0.015
- **THEN** the check passes (0.015 ≥ 0.5 × 0.02)

#### Scenario: Out-of-sample collapse fails retention

- **WHEN** the check runs with defaults over evidence with valid fold geometry, `InSample.Mean` 0.02 and `OutOfSample.Mean` 0.004
- **THEN** the check fails and its result identifies the retention condition

#### Scenario: A lookahead fold fails geometry

- **WHEN** the check runs over evidence containing a fold whose `TestStart` is strictly before its `TrainEnd`
- **THEN** the check fails and its result identifies the lookahead condition

#### Scenario: Too few folds fail

- **WHEN** the check runs with defaults over evidence carrying only 2 folds
- **THEN** the check fails and its result identifies the fold-count condition

#### Scenario: Non-positive in-sample performance fails outright

- **WHEN** the check runs over evidence with `InSample.Mean` of 0 or below
- **THEN** the check fails regardless of the out-of-sample values

### Requirement: Parameter-stability check

The system SHALL provide a parameter-stability check with two conditions. (a) Step bound: for every parameter name present in both `ProposedParams` and `BaselineParams` with a non-zero baseline value, `|proposed − baseline|` SHALL NOT exceed a configurable relative step (default 0.25) times `|baseline|`; a parameter with a zero baseline, or present in only one of the two maps, SHALL be skipped by this condition and noted in the check detail rather than failing it. (b) Cross-fold stability: for every parameter name appearing in the folds' `Params`, the coefficient of variation of its per-fold values (`stddev / |mean|`) SHALL NOT exceed a configurable maximum (default 0.5); a parameter whose cross-fold mean is zero SHALL be skipped by this condition. The check passes only when no evaluated parameter violates either condition.

#### Scenario: A proposal within the step bound and with stable folds passes

- **WHEN** the check runs with defaults over evidence whose every shared parameter moves at most 25% from baseline and whose per-fold parameter values have coefficient of variation at most 0.5
- **THEN** the check passes

#### Scenario: An oversized parameter step fails

- **WHEN** the check runs with defaults over evidence where a shared parameter has baseline 1.2 and proposed value 1.8 (a 50% move)
- **THEN** the check fails and its result names the offending parameter and the step condition

#### Scenario: A fold-unstable parameter fails

- **WHEN** the check runs with defaults over evidence where a parameter's per-fold values vary with coefficient of variation greater than 0.5 while all step bounds hold
- **THEN** the check fails and its result names the offending parameter and the cross-fold condition

#### Scenario: A parameter present only in the proposal is noted, not failed

- **WHEN** the check runs over evidence where `ProposedParams` contains a parameter name absent from `BaselineParams`, and all shared parameters satisfy both conditions
- **THEN** the check passes and its detail notes the unmatched parameter

### Requirement: Deflated performance check

The system SHALL provide a deflated-performance check that computes the out-of-sample t-statistic `t = OutOfSample.Mean / (OutOfSample.StdDev / sqrt(OutOfSample.SampleSize))` and passes only when `t >= max(minTStat, sqrt(2 × ln(max(TrialsCount, 1))))`, where `minTStat` is configurable (default 2.0). The bar therefore rises with the number of parameterizations the optimizer searched over, deflating selection bias. The check SHALL fail when `OutOfSample.StdDev` is zero or negative or `OutOfSample.SampleSize` is less than 2 (no honest dispersion evidence), and its result SHALL report the observed t-statistic and the effective bar.

#### Scenario: A strong result from a small search passes

- **WHEN** the check runs with defaults over evidence with `TrialsCount` 1 and an out-of-sample t-statistic of 2.5
- **THEN** the check passes (bar is max(2.0, 0) = 2.0)

#### Scenario: The same result from a large search fails the raised bar

- **WHEN** the check runs with defaults over evidence with the same out-of-sample t-statistic of 2.5 but `TrialsCount` 100
- **THEN** the check fails (bar is max(2.0, sqrt(2·ln 100)) ≈ 3.03)

#### Scenario: Zero dispersion fails closed

- **WHEN** the check runs over evidence whose `OutOfSample.StdDev` is 0
- **THEN** the check fails and its result identifies the dispersion condition

### Requirement: Deterministic all-checks gate verdict

The system SHALL provide a `RunGate(evidence, config)` function that runs the four checks in the fixed order minimum-sample → walk-forward → parameter-stability → deflated-performance, without short-circuiting (all four always run, so a failing proposal's verdict shows every deficiency), and returns a `Verdict` carrying the proposal identity, `Passed` (true only when every check passed), and the four `CheckResult`s each with name, pass/fail, observed value(s), threshold(s), and detail. Running `RunGate` twice on the same evidence and config SHALL produce identical verdicts.

#### Scenario: One failing check fails the verdict but all checks are reported

- **WHEN** `RunGate` runs over evidence that satisfies three checks and violates the minimum-sample gate
- **THEN** the verdict's `Passed` is false
- **AND** the verdict still contains all four check results, three passing and one failing

#### Scenario: Gate output is deterministic

- **WHEN** `RunGate` runs twice over the same evidence and config
- **THEN** both runs produce identical `Verdict` values

### Requirement: Persisted verdicts with additive migration

The system SHALL provide a GORM model `OverfittingVerdict` mapped to a new table `overfitting_verdicts` with fields: `id` (UUID primary key), `proposal_id` (UUID, deliberately NOT a foreign key — the proposal tables live in later optimizer changes which reference verdicts in the other direction), `proposal_kind` (text, constrained to `{scanner, strategy}` by Go validation and a database CHECK constraint), `computed_at` (not null), `passed` (boolean, not null), `checks_json` (JSONB serialization of the per-check results), and `library_version` (text). The system SHALL provide a migration entry point `MigrateOverfittingCountermeasures(db *gorm.DB) error` that is idempotent, creates only this table and its constraints, and SHALL NOT create, alter, or drop any `trading-stack-schema` table or any existing playground table.

#### Scenario: A verdict row round-trips including per-check JSON

- **WHEN** a failed verdict with four check results is persisted and re-read by id
- **THEN** the read-back row has `passed` false and its `checks_json` deserializes to the same four check results

#### Scenario: An invalid proposal_kind is rejected

- **WHEN** an `OverfittingVerdict` with `proposal_kind` outside `{scanner, strategy}` is saved
- **THEN** the save returns a non-nil error and no row is written

#### Scenario: Migration creates exactly one table and is idempotent

- **WHEN** `MigrateOverfittingCountermeasures` runs twice against a database that already has the six `trading-stack-schema` tables
- **THEN** the table `overfitting_verdicts` exists, the second run returns a nil error, and none of the `trading-stack-schema` or playground tables is created, altered, or dropped by either call

### Requirement: VerdictStore interface with an in-memory fake

The system SHALL define a `VerdictStore` interface with methods `Persist(v OverfittingVerdict) error` and `FetchLatestByProposal(proposalID uuid.UUID) (*OverfittingVerdict, error)`, a GORM-backed production implementation, and an in-memory fake used by unit tests, so gate-adjacent orchestration in the consuming optimizers is testable without a live database.

#### Scenario: The fake records and returns what was persisted

- **WHEN** two verdicts for the same proposal are persisted through the in-memory fake and `FetchLatestByProposal` is called
- **THEN** the fake returns the verdict with the later `computed_at`

### Requirement: Operator self-check command and Taskfile targets

The system SHALL provide a runnable command `cmd/overfitting-check/main.go` supporting a `--synthetic` mode (default) that runs the gate over built-in passing and failing fixture evidence with no external input, prints each check's name, observed value, threshold, and pass/fail plus the overall verdict, and exits non-zero only on an internal error — never merely because a fixture verdict failed. The system SHALL provide Taskfile targets `task optimizer:overfitting-check` (invokes the command) and `task test:overfitting` (runs `go test` for the `overfitting` package).

#### Scenario: Synthetic self-check runs with no external input

- **WHEN** an operator runs `task optimizer:overfitting-check` with no database available
- **THEN** the command prints per-check results for both fixtures, shows the failing fixture's verdict as failed, and exits zero

#### Scenario: Test target runs the package suite

- **WHEN** an operator runs `task test:overfitting`
- **THEN** the `overfitting` package's unit tests execute and the command exits zero when all pass, non-zero when any fails
