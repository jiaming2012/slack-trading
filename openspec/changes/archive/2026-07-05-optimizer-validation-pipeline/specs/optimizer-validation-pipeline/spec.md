# optimizer-validation-pipeline

## ADDED Requirements

### Requirement: Joined training row as the pipeline's unit of work

The system SHALL define a `TrainingRow` type that joins a `tradingstack.ScanResult` with its associated `tradingstack.SimOutcome` (via `scan_result_id`), carrying at minimum `StrategyID`, `Ticker`, `ScannedAt`, `DataAsOf`, `RegimeTag`, `RegimeConfidence`, and the numeric scanner features (`Price`, `VolumeRatio`, `RSI14`, `ATRPct`, `ScannerScore`) needed by later stages. Every pipeline stage SHALL operate on slices of `TrainingRow` (or the stage's own output type) and SHALL NOT perform database I/O itself.

#### Scenario: A joined scan_result/sim_outcome pair produces one TrainingRow

- **WHEN** a `ScanResult` and its linked `SimOutcome` are converted into a `TrainingRow`
- **THEN** the resulting `TrainingRow` carries the strategy id from the `SimOutcome` and the scanner fields from the `ScanResult`

### Requirement: Timestamp audit excludes lookahead-tainted rows

The system SHALL provide a timestamp-audit stage that, given a set of `TrainingRow`s, flags every row where `DataAsOf` is strictly after `ScannedAt` as a violation and excludes it from the rows passed to later stages. The stage SHALL return both the surviving rows and the list of excluded violations (with enough identifying information to audit later), and SHALL be deterministic and free of side effects.

#### Scenario: A row with data_as_of after scanned_at is excluded and reported

- **WHEN** the timestamp-audit stage runs over a batch containing one row whose `DataAsOf` is after its `ScannedAt` and one row where it is not
- **THEN** the surviving rows contain only the second row
- **AND** the violations list contains exactly the first row's identifying information

#### Scenario: A clean batch passes through unchanged

- **WHEN** the timestamp-audit stage runs over a batch where every row's `DataAsOf` is less than or equal to its `ScannedAt`
- **THEN** every row survives and the violations list is empty

### Requirement: Distribution check flags drift without dropping rows

The system SHALL provide a distribution-check stage that computes, for each named feature present in a supplied training-window baseline (`tradingstack.FeatureDistribution` rows), the mean of that feature across the current candidate batch and flags the feature as drifted when the absolute difference between the current mean and the baseline mean exceeds two times the baseline's `std_dev`. This stage SHALL be advisory only: it SHALL NOT remove or alter any `TrainingRow`, and SHALL return a report listing drifted and non-drifted features.

#### Scenario: A feature drifting more than 2 standard deviations is flagged

- **WHEN** the distribution-check stage runs with a baseline feature whose mean is `M` and `std_dev` is `S`, and the current batch's mean for that feature is `M + 3*S`
- **THEN** the report flags that feature as drifted
- **AND** no row is removed from the batch as a result of this stage

#### Scenario: A feature within 2 standard deviations is not flagged

- **WHEN** the distribution-check stage runs with a current batch mean within `2*S` of the baseline mean for a feature
- **THEN** the report does not flag that feature as drifted

### Requirement: Regime confidence filter drops low-confidence rows

The system SHALL provide a regime-confidence-filter stage that drops every `TrainingRow` whose `RegimeConfidence` is strictly less than a configurable threshold, defaulting to `0.7`, and returns the surviving rows and the dropped rows separately.

#### Scenario: Rows below the confidence threshold are dropped

- **WHEN** the regime-confidence-filter stage runs with the default threshold over a batch containing a row with `RegimeConfidence` of `0.65` and a row with `RegimeConfidence` of `0.9`
- **THEN** the surviving rows contain only the row with `RegimeConfidence` of `0.9`
- **AND** the dropped rows contain the row with `RegimeConfidence` of `0.65`

#### Scenario: A row exactly at the threshold survives

- **WHEN** the regime-confidence-filter stage runs with the default threshold over a row with `RegimeConfidence` exactly `0.7`
- **THEN** that row survives (the filter drops values strictly less than the threshold, not equal to it)

### Requirement: Fidelity gate drops rows from high-drift periods

The system SHALL provide a fidelity-gate stage that, given a set of `tradingstack.SimulatorFidelity` records, builds the set of high-drift periods per strategy (records where `within_tolerance` is false, using `period_start`/`period_end`) and drops every `TrainingRow` whose strategy id matches a high-drift period and whose `DataAsOf` falls within that period's bounds (inclusive). Rows for strategies with no `SimulatorFidelity` record, or whose `DataAsOf` falls outside every high-drift period, SHALL survive.

#### Scenario: A row inside a high-drift period for its strategy is dropped

- **WHEN** the fidelity-gate stage runs with a `SimulatorFidelity` record for strategy `S1` marked `within_tolerance = false` covering `period_start`..`period_end`, and a `TrainingRow` for strategy `S1` whose `DataAsOf` falls inside that range
- **THEN** that row is dropped

#### Scenario: A row outside all high-drift periods survives

- **WHEN** the fidelity-gate stage runs with a `TrainingRow` for strategy `S1` whose `DataAsOf` falls outside every `within_tolerance = false` period recorded for `S1`
- **THEN** that row survives

### Requirement: EV weight join with a neutral default

The system SHALL provide an EV-weight-join stage that, given a set of `tradingstack.StrategyEvWeight` records, attaches the most recently computed matching `ev_weight` (matched by strategy id and regime) to each surviving `TrainingRow`, producing a `WeightedTrainingRow`. When no matching `StrategyEvWeight` record exists for a row's strategy and regime, the stage SHALL assign a neutral default weight of `1.0` rather than dropping the row or assigning a zero weight.

#### Scenario: A row with a matching EV weight is joined

- **WHEN** the EV-weight-join stage runs with a `StrategyEvWeight` record for strategy `S1`, regime `trend`, `ev_weight = 0.3`, and a `TrainingRow` for strategy `S1` and regime `trend`
- **THEN** the resulting `WeightedTrainingRow` carries `ev_weight = 0.3`

#### Scenario: A row with no matching EV weight gets the neutral default

- **WHEN** the EV-weight-join stage runs with no `StrategyEvWeight` record matching a row's strategy and regime
- **THEN** the resulting `WeightedTrainingRow` carries `ev_weight = 1.0`
- **AND** the row is not dropped

### Requirement: Pipeline degrades gracefully when upstream tables are empty

The system SHALL run the full five-stage pipeline successfully (returning a non-error result) when `feature_distributions`, `simulator_fidelity`, and `strategy_ev_weights` are all empty — the expected state before `fidelity-checker` and `ev-tracker` have accrued any history. In that state, the distribution check SHALL report zero drifted features (nothing to compare against) rather than failing, the fidelity gate SHALL drop zero rows, and the EV weight join SHALL assign the neutral default weight `1.0` to every surviving row.

#### Scenario: Full pipeline runs end-to-end with all reference tables empty

- **WHEN** the pipeline runs over a batch of valid `TrainingRow`s with an empty `feature_distributions` baseline, an empty `simulator_fidelity` set, and an empty `strategy_ev_weights` set
- **THEN** the pipeline returns a result with no error
- **AND** every row that passed the timestamp audit and regime confidence filter appears in the output with `ev_weight = 1.0`

### Requirement: Deterministic, ordered pipeline orchestration

The system SHALL provide an orchestration function that runs the five stages in the fixed order timestamp audit → distribution check → regime confidence filter → fidelity gate → EV weight join, threading each stage's surviving rows into the next, and returns a single `Result` containing the clean weighted training dataset (`[]WeightedTrainingRow`), the timestamp-audit violations, the distribution report, the rows dropped by the regime filter, and the rows dropped by the fidelity gate. Running the orchestration twice on the same inputs SHALL produce identical output.

#### Scenario: Orchestration output reflects all four filtering/reporting stages

- **WHEN** the orchestration runs over a batch containing one timestamp violation, one low-confidence row, one row in a high-drift period, and one clean row with no matching EV weight
- **THEN** the `Result`'s clean weighted training dataset contains only the clean row, with `ev_weight = 1.0`
- **AND** the violations, regime-dropped, and fidelity-dropped lists each contain exactly the corresponding excluded row

#### Scenario: Running the pipeline twice on the same input is deterministic

- **WHEN** the orchestration runs twice over the same batch and reference data
- **THEN** both runs produce byte-for-byte identical `Result` values

### Requirement: Operator entry point and Taskfile target

The system SHALL provide a runnable command-line entry point that invokes the pipeline orchestration and prints, per run, the count of rows kept, the count and reason for each excluded/dropped row category, and the drifted-feature report, exiting non-zero only on an internal error (never merely because rows were dropped or drift was flagged). The command SHALL support a `--synthetic` mode that runs against built-in fixtures with no external input required. The system SHALL provide a `task optimizer:validate` Taskfile target that invokes this command.

#### Scenario: Synthetic self-check runs with no external input

- **WHEN** an operator runs `task optimizer:validate` with no database or live data available
- **THEN** the command runs in `--synthetic` mode, prints the per-stage counts, and exits zero

#### Scenario: Command exits non-zero only on internal error

- **WHEN** the command runs against a synthetic batch containing violations, drift, low-confidence rows, and high-drift-period rows, and the pipeline completes without an internal error
- **THEN** the command exits zero, even though rows were dropped and drift was flagged
