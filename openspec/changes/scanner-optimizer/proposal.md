# Scanner Optimizer (gated ScannerConfig proposals for operator review)

## Why

The scanner's filter thresholds and feature emphasis are frozen at their architecture-doc defaults today, so nothing feeds realized simulation outcomes back into what gets scanned; this change closes the medium feedback loop from the architecture doc — `scan_results ⋈ sim_outcomes` (weighted by EV) in, proposed `ScannerConfig` updates out — while keeping every proposal a persisted recommendation the operator reviews, never an auto-applied change.

## What Changes

- Add a Go representation of the architecture doc's **Scanner Config Payload** (`src/go/tradingstack/scannercfg/`): per-regime `feature_weights`, `drop_features`, `hard_filter_overrides` (`volume_ratio_floor`, `atr_pct_ceiling`), `score_threshold`, plus `global` (`top_n_candidates`, `min_labeled_samples`) and `version` — with verbatim JSON round-trip so payloads slot into the existing `scanner_configs` JSONB column, and shared with the upcoming `shadow-config-deployment` change.
- Add the optimizer itself (`src/go/tradingstack/scanneropt/`): a **deterministic, statistics-based tuner** (v1) that consumes ONLY the clean weighted dataset produced by the `optimizer-validation-pipeline` gate and, per regime, proposes normalized feature weights (EV-weighted effect size of each feature between winning and losing outcomes) and hard-filter overrides (EV-weighted percentiles of winners' `volume_ratio` / `atr_pct`). The architecture doc's XGBoost + SHAP method is deliberately deferred to a named future change (`scanner-optimizer-ml-ranking`); `score_threshold` is carried forward unchanged (Layer 3 ML scoring does not exist yet). Regimes with fewer decided samples than `min_labeled_samples` produce no proposal.
- Extend the `optimizer-validation-pipeline`'s `TrainingRow` with the two outcome fields the tuner needs (`PnlPct`, `OutcomeLabel` from the joined `SimOutcome`) — an additive field extension, no stage behavior changes.
- Every optimizer run builds walk-forward evidence (chronological folds over the training window) and submits it to the **`overfitting-countermeasures` gate**; the verdict (pass or fail) is persisted, and the proposal row records it.
- Persist proposals to a new Postgres table `scanner_config_proposals` (proposed payload JSON, evidence JSON, verdict reference, status `pending_review` | `rejected_by_gate` | `promoted`) via a dedicated additive migration. **The optimizer never writes to `scanner_configs`** — the active-config table changes only through the explicit operator promote command.
- Add a DB-backed loader that joins `scan_results` with `sim_outcomes` and the three reference tables, threads them through `optvalidation.Run`, and hands the clean output to the tuner — the first real (non-synthetic) consumer of the validation pipeline.
- Add operator commands (`cmd/scanner-optimizer/main.go` with `run` / `list` / `promote` subcommands; `run --synthetic` self-check needs no database) and Taskfile targets `task scanner:optimize`, `task scanner:proposals`, `task scanner:promote`, `task test:scanner-optimizer`.
- No new metrics or alerts (any future ones must use the internal telemetry registry per ADR-0005); Postgres persistence only (ESDB untouched, ADR-0004 pending); no changes to the `scanner` package's runtime behavior — the scanner does not yet consume configs (hot-swap integration is a named future change, `scanner-config-hot-swap`). **Not BREAKING.**

## Capabilities

### New Capabilities

- `scanner-optimizer`: the scanner-config payload type, the deterministic tuning method, the gate-integrated proposal flow with persisted `scanner_config_proposals`, and the operator review/promote commands.

### Modified Capabilities

- `optimizer-validation-pipeline`: the "Joined training row" requirement is extended so `TrainingRow` also carries `PnlPct` and `OutcomeLabel` from the joined `SimOutcome` — the outcome labels the scanner optimizer trains against. No stage semantics change.

## Impact

- New packages: `src/go/tradingstack/scannercfg/` (payload parse/validate/serialize) and `src/go/tradingstack/scanneropt/` (tuner, evidence builder, loader, proposal model + migration, store, synthetic fixtures, tests). New command `cmd/scanner-optimizer/`. Edited existing files: `src/go/tradingstack/optvalidation/row.go` (two additive fields + constructor mapping) and `taskfile.yml`.
- **Sequencing**: hard dependencies on `trading-stack-schema`, `optimizer-validation-pipeline` (already archived) and on **`overfitting-countermeasures`, which must land before or alongside this change** — the proposal flow imports its gate and persists its verdicts. The sibling `strategy-optimizer` change mirrors the identical "proposals are recommendations; the operator promotes" contract; the two share the gate but no code tables.
- Data dependency (not code) on `ev-tracker` and `fidelity-checker` history via the validation pipeline; with empty reference tables the pipeline's documented graceful degradation applies (neutral EV weights), so the optimizer is runnable — just unweighted — from day one.
- Autonomous-run safe: verification is deterministic unit tests plus testcontainers round-trips; `run --synthetic` needs no database; no live/paper orders, no prod DB, no push, no new infra. Promotion (`task scanner:promote`) is an explicit operator-only action.
- **Conservative scope decisions needing sign-off** (detail in design.md): v1 tuning is deterministic effect-size statistics instead of the architecture doc's XGBoost + SHAP (deferred, named future change); `score_threshold` is not tuned yet; proposals only tighten/adjust `volume_ratio_floor` and `atr_pct_ceiling` among the hard filters; the scanner does not yet read configs at runtime, so a promoted config has no effect until `scanner-config-hot-swap` lands.
