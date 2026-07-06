# optimizer-validation-pipeline Specification (delta)

## MODIFIED Requirements

### Requirement: Joined training row as the pipeline's unit of work

The system SHALL define a `TrainingRow` type that joins a `tradingstack.ScanResult` with its associated `tradingstack.SimOutcome` (via `scan_result_id`), carrying at minimum `StrategyID`, `Ticker`, `ScannedAt`, `DataAsOf`, `RegimeTag`, `RegimeConfidence`, the numeric scanner features (`Price`, `VolumeRatio`, `RSI14`, `ATRPct`, `ScannerScore`) needed by later stages, and the outcome fields `PnlPct` and `OutcomeLabel` from the joined `SimOutcome` (needed by the scanner optimizer to label wins and losses; a nil source `PnlPct` maps to `0`, and a nil source `OutcomeLabel` maps to the empty string). Every pipeline stage SHALL operate on slices of `TrainingRow` (or the stage's own output type) and SHALL NOT perform database I/O itself; no pipeline stage SHALL read or alter the two outcome fields.

#### Scenario: A joined scan_result/sim_outcome pair produces one TrainingRow

- **WHEN** a `ScanResult` and its linked `SimOutcome` are converted into a `TrainingRow`
- **THEN** the resulting `TrainingRow` carries the strategy id from the `SimOutcome` and the scanner fields from the `ScanResult`

#### Scenario: Outcome fields are carried from the SimOutcome

- **WHEN** a `ScanResult` and its linked `SimOutcome` with `pnl_pct` 0.034 and `outcome_label` `win` are converted into a `TrainingRow`
- **THEN** the resulting `TrainingRow` carries `PnlPct` 0.034 and `OutcomeLabel` `win`

#### Scenario: Pipeline stage behavior is unchanged by the outcome fields

- **WHEN** the full five-stage pipeline runs over the same batch twice, once with outcome fields zeroed and once with them populated
- **THEN** the same rows survive each stage in both runs, differing only in the carried outcome field values
