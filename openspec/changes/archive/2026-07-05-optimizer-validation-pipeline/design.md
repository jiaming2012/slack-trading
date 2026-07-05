# Design — optimizer-validation-pipeline

## Approach

A pure-Go, five-stage pipeline mirroring the architecture doc's diagram exactly, operating on an in-memory `TrainingRow` (a join of `tradingstack.ScanResult` and its `tradingstack.SimOutcome`) so the whole thing is testable with synthetic fixtures and zero database dependency in the core logic. Each stage is a small, independently testable pure function; an orchestration function threads them together in the fixed documented order. Persistence/loading of the joined rows and the three reference tables (`feature_distributions`, `simulator_fidelity`, `strategy_ev_weights`) is a thin, separately-swappable loader — not exercised overnight, since it needs a real database with real history.

Two stages act as **filters** (drop rows): timestamp audit, regime confidence filter, fidelity gate. One stage is **advisory only** (distribution check — flags, does not drop, per the architecture doc's own "Flag drift" language, distinct from the explicit "Drop rows" language used for the regime and fidelity stages). One stage is a **join** (EV weight).

## Why timestamp-audit violations are dropped, not just flagged

`trading-stack-schema` already enforces `data_as_of <= scanned_at` at write time (Go validation + DB CHECK constraint), so in the steady state no violating row should exist. This stage is defense-in-depth: it assumes the invariant can be violated by data that predates the constraint, by bulk-loaded/migrated rows, or by any future write path that bypasses the GORM hook. A `data_as_of > scanned_at` row represents literal lookahead bias (the scanner used data that did not yet exist), which is unacceptable in a training set regardless of cause — so, unlike the distribution check, this stage both flags and excludes.

## Why the distribution check does not drop rows

The architecture doc's own language distinguishes "Flag drift" (distribution check) from "Drop rows" (regime filter, fidelity gate). A feature-level distribution shift is a signal for a human/operator to investigate (is the market regime shifting? is scanner instrumentation broken?) — it is not, by itself, evidence that any individual row is untrustworthy the way a lookahead violation or a known high-drift simulator period is. The stage returns a report; nothing downstream is required to act on it in this change (a future change may wire the report to an alert, mirroring `fidelity-checker`'s gate/alert pattern).

## Package and file layout

New sub-package `src/go/tradingstack/optvalidation/` (package `optvalidation`, import path `github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation`):

- `row.go` — `TrainingRow` (joins `tradingstack.ScanResult` + `tradingstack.SimOutcome`: `StrategyID`, `Ticker`, `ScannedAt`, `DataAsOf`, `RegimeTag`, `RegimeConfidence`, `Price`, `VolumeRatio`, `RSI14`, `ATRPct`, `ScannerScore`) and `WeightedTrainingRow` (`TrainingRow` + `EVWeight float64`). `NewTrainingRow(sr tradingstack.ScanResult, so tradingstack.SimOutcome) TrainingRow` constructor.
- `config.go` — `Config{RegimeConfidenceThreshold float64, DistributionDriftStdDevs float64}`, `DefaultConfig()` (`0.7`, `2.0`).
- `stage1_timestamp_audit.go` — `TimestampAudit(rows []TrainingRow) (kept []TrainingRow, violations []Violation)`.
- `stage2_distribution_check.go` — `DistributionCheck(rows []TrainingRow, baseline []tradingstack.FeatureDistribution, cfg Config) DistributionReport` — computes current mean per named feature the baseline declares (feature name → row-field lookup table), compares against `baseline.Mean`/`baseline.StdDev`.
- `stage3_regime_filter.go` — `RegimeConfidenceFilter(rows []TrainingRow, cfg Config) (kept, dropped []TrainingRow)`.
- `stage4_fidelity_gate.go` — `FidelityGate(rows []TrainingRow, fidelity []tradingstack.SimulatorFidelity) (kept, dropped []TrainingRow)` — indexes high-drift (`within_tolerance = false`) periods by `strategy_id`, drops rows whose `(StrategyID, DataAsOf)` falls in `[period_start, period_end]`.
- `stage5_ev_weight_join.go` — `EVWeightJoin(rows []TrainingRow, weights []tradingstack.StrategyEvWeight) []WeightedTrainingRow` — indexes weights by `(strategy_id, regime)`, keeps the most recent `computed_at` on duplicates, defaults to `1.0` on no match.
- `pipeline.go` — `Input{Rows []TrainingRow, Baseline []tradingstack.FeatureDistribution, Fidelity []tradingstack.SimulatorFidelity, EVWeights []tradingstack.StrategyEvWeight}`, `Result{Clean []WeightedTrainingRow, Violations []Violation, DistributionReport DistributionReport, DroppedByRegime []TrainingRow, DroppedByFidelity []TrainingRow}`, `Run(input Input, cfg Config) (Result, error)` threading stages 1→2→3→4→5 in fixed order.
- `synthetic.go` — `SyntheticInput()` builder producing one row per excluded category (timestamp violation, drifted feature via baseline, low regime confidence, high-drift-period row, no-EV-weight-match row, and one fully-clean row) — shared by tests and the command's `--synthetic` mode, mirroring the `fidelity-checker` pattern.
- `errors.go` — sentinel errors (e.g. `ErrEmptyBatch`).
- `optvalidation_test.go` — table-driven unit tests per stage plus a full-pipeline integration test and a determinism (run-twice) test.

New thin command `cmd/optimizer-validate/main.go` — flag `--synthetic` (default true for a runnable no-input self-check); calls `Run`, prints per-stage counts (kept, timestamp violations, drifted features, regime-dropped, fidelity-dropped) and per-row `ev_weight`; exits non-zero only on a returned error.

## Data flow

```
scan_results ⋈ sim_outcomes (TrainingRow)
        │
        ▼
1. Timestamp Audit          drop data_as_of > scanned_at   ──► Violations
        │ kept
        ▼
2. Distribution Check        current feature means vs feature_distributions baseline
        │ (all rows pass through unchanged)                ──► DistributionReport
        ▼
3. Regime Confidence Filter  drop regime_confidence < 0.7   ──► DroppedByRegime
        │ kept
        ▼
4. Fidelity Gate             drop rows in high-drift periods per simulator_fidelity
        │ kept                                              ──► DroppedByFidelity
        ▼
5. EV Weight Join            attach ev_weight from strategy_ev_weights (default 1.0)
        │
        ▼
Clean weighted training dataset (Result.Clean) → Scanner Optimizer (future change)
```

## Out of scope

- Loading real `scan_results`/`sim_outcomes`/`feature_distributions`/`simulator_fidelity`/`strategy_ev_weights` rows from Postgres — the pipeline core takes in-memory slices; a real GORM-backed loader is a thin, separate concern left for the change that wires this into an actual training cycle.
- Running or validating against a real training cycle (deferred; see below).
- Acting on the distribution-check report (alerting, blocking optimizer entry) — this change returns the report; wiring a gate/alert on it (as `fidelity-checker` does for its own drift signal) is a future decision.
- Any change to `tradingstack` schema models, playground tables, `dbutils`, or the `fidelity-checker`/`ev-tracker` packages themselves.
- A Temporal/cron/scheduled invocation — `task optimizer:validate` is the wired, runnable entry point; scheduling it is out of scope (no new infra overnight).

## Dependency ordering within the batch

Hard dependency: `trading-stack-schema` (imports `tradingstack.ScanResult`, `tradingstack.SimOutcome`, `tradingstack.FeatureDistribution`, `tradingstack.SimulatorFidelity`, `tradingstack.StrategyEvWeight`) — must be authored after that change merges and archives. Soft/data dependency: `fidelity-checker` and `ev-tracker` populate `simulator_fidelity` and `strategy_ev_weights` respectively, but this change consumes those tables through the shared `tradingstack` models only, not through either package's code — it does not import `src/go/tradingstack/fidelity` or the EV tracker package. Per the batch schedule this change is authored in the wave after both have merged; its documented graceful-degradation behavior (Requirement: "Pipeline degrades gracefully when upstream tables are empty") also makes it correct, if conservative, even if run before either has accrued history.

## Verification gates

- **Deterministic unit tests per pipeline stage** — `go test ./src/go/tradingstack/optvalidation/...`: timestamp audit (violation excluded, clean batch passes), distribution check (drift flagged at >2 std dev, not flagged within), regime filter (below/at/above threshold), fidelity gate (inside/outside high-drift period), EV weight join (match found, no-match default), full-pipeline integration (one row per excluded category + one clean row), determinism (run twice, identical `Result`), and the empty-reference-tables graceful-degradation scenario.
- **G1** — `go build ./src/go/... ./cmd/...` green (new package and `cmd/optimizer-validate` compile, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected).
- **G6** — Fable adversarial review of the diff before commit (drop-vs-flag semantics per stage are correct per this design's reasoning, graceful-degradation defaults do not silently zero out or drop data, deterministic ordering, no leakage into playground tables).

## Deferred validation

Per the overnight run plan (Tier B), the following is explicitly deferred and NOT attempted overnight:

- **A real optimizer training cycle** run against the pipeline's output. Overnight verification is limited to synthetic `TrainingRow` fixtures with violations, drift, low confidence, and high-drift periods known by construction; there is no real `scan_results`/`sim_outcomes` volume or real `feature_distributions`/`simulator_fidelity`/`strategy_ev_weights` history to validate against yet (the same accrual gap the architecture doc's own Build Order table calls out for the optimizers downstream of this gate).
- A real GORM-backed loader translating Postgres rows into `Input` is not built or tested here (out of scope above); when it is added, it should be validated with a testcontainers round-trip, not synthetic fixtures.
- Wiring the distribution-check report into an alert or a hard gate is deferred to a future change.
