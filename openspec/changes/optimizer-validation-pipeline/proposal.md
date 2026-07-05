# Optimizer Validation Pipeline (pre-training data gate)

## Why

Every optimizer training cycle is only as trustworthy as the data it trains on; this change adds the deterministic, five-stage data gate the architecture doc requires to run before that cycle so lookahead-tainted, low-confidence, and untrustworthy-period rows never reach the optimizer.

## What Changes

- Add a new deterministic Go sub-package that runs the five gate stages, in order, over `scan_results` joined with their `sim_outcomes`: **(1) timestamp audit** — exclude rows where `data_as_of > scanned_at` (a lookahead-bias violation); **(2) distribution check** — compute current per-feature distributions over the candidate batch and flag (advisory, non-dropping) any feature whose mean drifts more than 2 standard deviations from the `feature_distributions` training-window baseline; **(3) regime confidence filter** — drop rows with `regime_confidence < 0.7`; **(4) fidelity gate** — drop rows whose strategy and `data_as_of` fall inside a high-drift period recorded in `simulator_fidelity` (`within_tolerance = false`); **(5) EV weight join** — attach `ev_weight` from `strategy_ev_weights` to each surviving row.
- Define a `TrainingRow` type joining `tradingstack.ScanResult` and `tradingstack.SimOutcome` fields (the actual unit the Scanner Optimizer trains on) and a `WeightedTrainingRow` output type carrying the joined `ev_weight`.
- Each stage is a pure function over in-memory rows and reference data (no DB/network/clock inside the stage logic), so the overnight verification runs entirely against synthetic fixtures with known-by-construction violations, drift, low confidence, and high-drift periods.
- Document and implement graceful degradation: when `simulator_fidelity` and/or `strategy_ev_weights` are empty (the normal state before `fidelity-checker` and `ev-tracker` have accrued history), the fidelity gate drops nothing and the EV weight join assigns a neutral default weight (`1.0`) rather than dropping or zero-weighting rows — the pipeline stays runnable end-to-end with no upstream history.
- Add an operator-visible entry point (`cmd/optimizer-validate/main.go`, `--synthetic` self-check mode) and a matching `task optimizer:validate` Taskfile target.
- No changes to any existing playground table, model, or migration, and no changes to the `tradingstack` schema models delivered by `trading-stack-schema`. **Not BREAKING.**

## Capabilities

### New Capabilities

- `optimizer-validation-pipeline`

### Modified Capabilities

(none)

## Impact

- New sub-package: `src/go/tradingstack/optvalidation/` (row types, five stage files, orchestration, config, synthetic fixtures, errors, tests). New thin command `cmd/optimizer-validate/`. Only `taskfile.yml` is edited among existing files (adds `optimizer:validate`).
- **Hard dependency on `trading-stack-schema`**: imports `tradingstack.ScanResult`, `tradingstack.SimOutcome`, `tradingstack.FeatureDistribution`, `tradingstack.SimulatorFidelity`, and `tradingstack.StrategyEvWeight`. Must be authored after that change lands and archives.
- **Data dependency (not a code/package dependency) on `fidelity-checker` and `ev-tracker`**: this change reads the `simulator_fidelity` and `strategy_ev_weights` rows those changes write, through the shared `tradingstack` models, not through their packages. Per the batch schedule it is authored in the wave after both have merged, and its documented graceful-degradation behavior means it is also correct — just conservative — if either has not run yet.
- New dependency surface only within already-vendored modules: `gorm.io/gorm`, `github.com/google/uuid`, `github.com/stretchr/testify`. No new external infrastructure, no scheduler.
- Verification is local-only (deterministic unit tests over synthetic fixtures, no database). No prod DB, no live/paper broker, no infra provisioning. Validation against a real training cycle is explicitly deferred (see design.md).
