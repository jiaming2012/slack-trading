# crowding-detection Specification

## Purpose
TBD - created by archiving change crowding-detection. Update Purpose after archive.
## Requirements
### Requirement: Scan-cycle overlap computation

The system SHALL provide a pure function `DetectCrowding(candidates []ScanCandidate, thresholdPct float64) (CrowdingResult, error)` that, given the candidates belonging to a single scan cycle (all sharing one `scanned_at`) and the distinct strategy IDs (from `sim_outcomes.strategy_id`) that produced an outcome for each candidate, computes: the total candidate count, the count of candidates claimed by two or more distinct strategies ("overlapping candidates"), and the overlap percentage (`overlapping_candidates / total_candidates * 100`). The function SHALL NOT query a database — it SHALL operate only on data passed to it, so it is testable with fixtures alone.

#### Scenario: Known overlap percentage matches a fixture

- **WHEN** `DetectCrowding` is called with a fixture of 10 candidates in one scan cycle where exactly 3 candidates each have outcomes from 2 or more distinct strategies and the remaining 7 have at most 1 strategy
- **THEN** the returned `CrowdingResult.TotalCandidates` is 10, `OverlappingCandidates` is 3, and `OverlapPct` is 30.0

#### Scenario: Zero overlap when every candidate has at most one strategy

- **WHEN** `DetectCrowding` is called with a fixture where no candidate has outcomes from more than one distinct strategy
- **THEN** `OverlapPct` is 0.0

#### Scenario: Full overlap when every candidate has multiple strategies

- **WHEN** `DetectCrowding` is called with a fixture where every candidate has outcomes from at least 2 distinct strategies
- **THEN** `OverlapPct` is 100.0

#### Scenario: Candidates with zero simulating strategies count toward the total but never toward overlap

- **WHEN** `DetectCrowding` is called with a fixture that includes candidates with no `sim_outcomes` rows at all
- **THEN** those candidates are included in `TotalCandidates` and excluded from `OverlappingCandidates`

### Requirement: Configurable overlap threshold with strict-greater-than flagging

The system SHALL accept a configurable overlap threshold percentage (0–100, default 30.0) and SHALL set `CrowdingResult.Flagged` to true only when `OverlapPct` is strictly greater than the threshold; a cycle whose `OverlapPct` equals the threshold SHALL NOT be flagged.

#### Scenario: Overlap above threshold is flagged

- **WHEN** `DetectCrowding` is called with a threshold of 25.0 and a fixture producing `OverlapPct` of 30.0
- **THEN** `CrowdingResult.Flagged` is true

#### Scenario: Overlap exactly at threshold is not flagged

- **WHEN** `DetectCrowding` is called with a threshold of 30.0 and a fixture producing `OverlapPct` of exactly 30.0
- **THEN** `CrowdingResult.Flagged` is false

#### Scenario: Overlap below threshold is not flagged

- **WHEN** `DetectCrowding` is called with a threshold of 50.0 and a fixture producing `OverlapPct` of 10.0
- **THEN** `CrowdingResult.Flagged` is false

### Requirement: Persisted crowding_metrics and crowding_flagged_candidates models

The system SHALL provide two new GORM models owned by this change: `CrowdingMetric` mapped to table `crowding_metrics` (fields: `id` UUID primary key, `scanned_at`, `computed_at`, `total_candidates`, `overlapping_candidates`, `overlap_pct`, `threshold_pct`, `flagged`) and `CrowdingFlaggedCandidate` mapped to table `crowding_flagged_candidates` (fields: `id` UUID primary key, `crowding_metric_id` foreign key referencing `crowding_metrics.id`, `scan_result_id` foreign key referencing `scan_results.id`, `ticker`, `strategy_ids` stored as a JSON array of strings). The system SHALL provide a dedicated migration entry point `MigrateCrowdingDetection(db *gorm.DB) error` that creates only these two tables and SHALL NOT alter any `trading-stack-schema` table or any existing playground table.

#### Scenario: Migration creates exactly the two crowding tables

- **WHEN** `MigrateCrowdingDetection` runs against a database that already has the six `trading-stack-schema` tables
- **THEN** the tables `crowding_metrics` and `crowding_flagged_candidates` exist with those exact names
- **AND** none of `scan_results`, `sim_outcomes`, `simulator_fidelity`, `strategy_ev_weights`, `scanner_configs`, `feature_distributions`, `playgrounds`, `order_records`, `trade_records`, `equity_plot_records`, `live_accounts`, or `live_account_plots` is created, altered, or dropped by the call

### Requirement: CrowdingStore interface with a fake for deterministic tests

The system SHALL define a `CrowdingStore` interface with a method `Persist(metric CrowdingMetric, flagged []CrowdingFlaggedCandidate) error`, a GORM-backed production implementation, and an in-memory fake implementation used by unit tests, so persistence-adjacent orchestration logic is testable without a live database connection.

#### Scenario: In-memory fake records what was persisted

- **WHEN** a `CrowdingResult` for a scan cycle with 2 flagged candidates is persisted through the in-memory fake `CrowdingStore`
- **THEN** the fake exposes the same `CrowdingMetric` values and exactly 2 `CrowdingFlaggedCandidate` entries that were passed in

### Requirement: Taskfile target for crowding-detection unit tests

The system SHALL provide a `task test:crowding-detection` Taskfile target that runs `go test` for the `src/go/tradingstack/crowding` package. The target SHALL exit non-zero if any fixture overlap, threshold-boundary, or store test fails.

#### Scenario: Task target runs the crowding unit-test suite

- **WHEN** an operator runs `task test:crowding-detection`
- **THEN** the `crowding` package's unit tests execute and the command exits zero when all tests pass, non-zero when any fails

