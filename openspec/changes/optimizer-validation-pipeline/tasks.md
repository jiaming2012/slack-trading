# Tasks — optimizer-validation-pipeline

## 1. Package scaffold and row types

- [ ] 1.1 Create `src/go/tradingstack/optvalidation/` package with `errors.go` (sentinel errors incl. `ErrEmptyBatch`) and `row.go` (`TrainingRow` joining `tradingstack.ScanResult` + `tradingstack.SimOutcome`; `WeightedTrainingRow`; `NewTrainingRow(sr, so)` constructor).
- [ ] 1.2 `config.go` — `Config{RegimeConfidenceThreshold, DistributionDriftStdDevs}` with `DefaultConfig()` (`0.7`, `2.0`).

## 2. Stage 1 — timestamp audit

- [ ] 2.1 `stage1_timestamp_audit.go` — `TimestampAudit(rows []TrainingRow) (kept []TrainingRow, violations []Violation)`: excludes rows where `DataAsOf > ScannedAt`, pure function, deterministic ordering of output.

## 3. Stage 2 — distribution check (advisory)

- [ ] 3.1 `stage2_distribution_check.go` — `DistributionCheck(rows []TrainingRow, baseline []tradingstack.FeatureDistribution, cfg Config) DistributionReport`: computes current per-feature mean over the batch for each feature named in the baseline, flags drift when `|current_mean - baseline.Mean| > cfg.DistributionDriftStdDevs * baseline.StdDev`; never removes or mutates rows; returns empty/no-drift report when baseline is empty.

## 4. Stage 3 — regime confidence filter

- [ ] 4.1 `stage3_regime_filter.go` — `RegimeConfidenceFilter(rows []TrainingRow, cfg Config) (kept, dropped []TrainingRow)`: drops rows with `RegimeConfidence < cfg.RegimeConfidenceThreshold` (strict less-than; a row exactly at the threshold survives).

## 5. Stage 4 — fidelity gate

- [ ] 5.1 `stage4_fidelity_gate.go` — `FidelityGate(rows []TrainingRow, fidelity []tradingstack.SimulatorFidelity) (kept, dropped []TrainingRow)`: indexes `within_tolerance = false` periods by `strategy_id`, drops rows whose `(StrategyID, DataAsOf)` falls within `[period_start, period_end]` inclusive; rows for strategies with no fidelity record, or outside all high-drift periods, survive; empty `fidelity` input drops nothing.

## 6. Stage 5 — EV weight join

- [ ] 6.1 `stage5_ev_weight_join.go` — `EVWeightJoin(rows []TrainingRow, weights []tradingstack.StrategyEvWeight) []WeightedTrainingRow`: indexes weights by `(strategy_id, regime)` keeping the most recent `computed_at` on duplicates; assigns the matched `ev_weight`, or the neutral default `1.0` when no match exists (never drops a row, never assigns zero on no-match).

## 7. Pipeline orchestration and synthetic fixtures

- [ ] 7.1 `pipeline.go` — `Input` and `Result` types; `Run(input Input, cfg Config) (Result, error)` threading stages in fixed order 1→2→3→4→5; `Result` carries `Clean`, `Violations`, `DistributionReport`, `DroppedByRegime`, `DroppedByFidelity`.
- [ ] 7.2 `synthetic.go` — `SyntheticInput()` builder producing one row per excluded category (timestamp violation, drifted-feature baseline, low regime confidence, high-drift-period row, no-EV-weight-match row) plus one fully-clean row, shared by tests and the command's `--synthetic` mode.

## 8. Operator entry point and Taskfile target

- [ ] 8.1 `cmd/optimizer-validate/main.go` — flag `--synthetic` (default true); calls `Run`, prints per-stage counts (kept, violations, drifted features, regime-dropped, fidelity-dropped) and each surviving row's `ev_weight`; exits non-zero only on a returned error (never merely because rows were dropped or drift was flagged).
- [ ] 8.2 Add `optimizer:validate` target to `taskfile.yml` invoking the command, runnable with no external input.

## 9. Unit tests per stage and full-pipeline integration

- [ ] 9.1 `optvalidation_test.go` — timestamp audit: violation excluded and reported; clean batch passes through unchanged.
- [ ] 9.2 Distribution check: feature drifting > 2 std dev flagged; feature within 2 std dev not flagged; empty baseline yields empty/no-drift report; no row is ever removed by this stage.
- [ ] 9.3 Regime confidence filter: row below threshold dropped; row above threshold kept; row exactly at threshold kept (strict less-than semantics).
- [ ] 9.4 Fidelity gate: row inside a high-drift period for its strategy dropped; row outside all high-drift periods kept; empty fidelity input drops nothing.
- [ ] 9.5 EV weight join: matching record joined correctly; no-match row gets neutral default `1.0` and is not dropped; duplicate weights for the same strategy/regime resolve to the most recent `computed_at`.
- [ ] 9.6 Full-pipeline integration: batch with one row per excluded category plus one clean row — clean weighted output contains only the clean row with `ev_weight = 1.0`; violations/regime-dropped/fidelity-dropped lists each contain exactly the corresponding row.
- [ ] 9.7 Graceful degradation: full pipeline runs with `feature_distributions`, `simulator_fidelity`, and `strategy_ev_weights` all empty — no error, no fidelity drops, `ev_weight = 1.0` on every surviving row.
- [ ] 9.8 Determinism: running `Run` twice on the same `Input` produces byte-for-byte identical `Result` values.

## 10. Verification and closeout

- [ ] 10.1 Deterministic unit tests per pipeline stage pass: `go test -count=1 ./src/go/tradingstack/optvalidation/...` green (all stages, full-pipeline integration, graceful degradation, determinism).
- [ ] 10.2 G1 — `go build ./src/go/... ./cmd/...` green (new package and `cmd/optimizer-validate` compile, nothing else broken).
- [ ] 10.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [ ] 10.4 G6 — Fable adversarial review approves the diff (drop-vs-flag semantics correct per stage, graceful-degradation defaults don't silently zero out or drop data, deterministic ordering, no leakage into playground tables) before commit.
- [ ] 10.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
