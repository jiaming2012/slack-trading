# Design — scanner-optimizer

## Context

The architecture doc's Scanner Optimizer is the medium (monthly) feedback loop: train on `scan_results JOIN sim_outcomes` weighted by `strategy_ev_weights`, tune feature weights per regime, hard-filter thresholds, and score cutoffs, and emit a versioned scanner-config payload. The data gate in front of it (`optimizer-validation-pipeline`) already exists and produces `[]WeightedTrainingRow`. What does not exist: the config-payload type in Go, any tuning logic, a proposal store, or the operator review path. The doc prescribes XGBoost retraining + SHAP importance; this batch is autonomous-run-constrained (everything verifiable in Simulation/tests, deterministic, no new infra), and Layer 3 (ML ranking) of the scanner does not exist yet either — there is no model to retrain. The honest v1 is therefore a deterministic tuner whose proposals flow through the shared `overfitting-countermeasures` gate and land in a review queue.

## Goals

- Close the loop shape: validated training rows in → gated, versioned `ScannerConfig` proposal out → operator reviews → operator promotes.
- Deterministic and fixture-testable end to end; DB touches confined to a thin loader and the proposal/config stores, each testcontainers-verified.
- Exact payload fidelity with the architecture doc's Scanner Config Payload JSON, so proposals drop into the existing `scanner_configs.config_json` JSONB column unchanged when promoted.
- Mirror the `strategy-optimizer` contract precisely: proposals are recommendations; nothing auto-applies; the gate verdict is persisted either way; promotion is one explicit operator command.

## Non-Goals

- XGBoost + SHAP tuning (deferred to future change `scanner-optimizer-ml-ranking`, which also owns any Python/gRPC bridge).
- Tuning `score_threshold` — the scanner has no Layer 3 score to threshold yet; the value is carried forward from the baseline payload verbatim.
- Making the scanner *consume* configs at runtime (hot-swap polling per the architecture doc) — deferred to future change `scanner-config-hot-swap`. Until it lands, a promoted config is recorded intent, not live behavior; flagged in the proposal for sign-off.
- Scheduling (the doc's "monthly" cadence) — operator/task-invoked only; no cron, no Temporal, no new infra.
- Any new metric or alert. If added later they MUST use the internal telemetry registry (`src/go/telemetry`) per ADR-0005 — never OpenTelemetry (removed). The persisted proposal/verdict rows are the durable record of optimizer activity; a one-shot CLI's in-memory counters would not outlive the process anyway.
- ESDB events (ADR-0004 pending) — Postgres only.

## Decisions

### D1 — Payload type lives in its own small package, `scannercfg`

`src/go/tradingstack/scannercfg/` owns `Payload{Version string, RegimeModels map[string]RegimeModel, Global GlobalConfig}` with `RegimeModel{FeatureWeights map[string]float64, DropFeatures []string, HardFilterOverrides HardFilterOverrides{VolumeRatioFloor, ATRPctCeiling *float64}, ScoreThreshold *float64}` and `GlobalConfig{TopNCandidates, MinLabeledSamples int}`, plus `Parse([]byte) (Payload, error)`, `(Payload).Marshal() ([]byte, error)` (stable key order), and `Validate()` (weights non-negative and finite, thresholds in sane ranges). Why its own package: `shadow-config-deployment` needs the payload without the tuner, and `scanneropt` importing `scannercfg` keeps the dependency one-way. *Alternative:* define it inside `scanneropt` — rejected, would force shadow deployment to import the whole optimizer.

### D2 — v1 tuning method: EV-weighted effect-size statistics (XGBoost/SHAP deferred)

Per regime group (rows grouped by `RegimeTag`; rows with empty regime tags are excluded from tuning):

- **Labels**: a row is a *win* when `PnlPct > 0`, a *loss* when `PnlPct < 0`; `PnlPct == 0` rows are excluded from label-conditioned statistics (mirroring `ev-computation`'s decided-trade convention). All statistics are weighted by the row's `EVWeight`.
- **Feature weights** over the numeric features available on `TrainingRow` (`rsi_14`, `volume_ratio`, `atr_pct`): per feature, effect size `e_f = |weightedMean(f | win) − weightedMean(f | loss)| / pooledWeightedStdDev(f)`; proposed weight `w_f = e_f / Σ e`. Features with zero pooled dispersion get weight 0. If all effect sizes are 0, feature weights are carried forward from the baseline.
- **Hard-filter overrides**: `volume_ratio_floor` = EV-weighted 25th percentile of winners' `volume_ratio`; `atr_pct_ceiling` = EV-weighted 75th percentile of winners' `atr_pct`. Rationale: bound the scan region toward where winners actually lived, conservatively (inner quartiles, not extremes). The `overfitting-countermeasures` parameter-stability check independently bounds how far these may move from the baseline.
- **Skips**: a regime with fewer than `Global.MinLabeledSamples` decided rows produces no `RegimeModel` proposal for that regime (the baseline's model, if any, is carried forward in the payload so a proposal is always a complete payload).
- `TrialsCount` reported to the gate is 1 — this is a direct derivation, not a search; the deflated check then reduces to the plain significance floor, which is honest.

*Alternatives considered:* (a) XGBoost + SHAP per the doc — rejected for v1: no Layer 3 model exists to retrain, it drags in Python/gRPC plumbing and non-deterministic training, and it cannot be verified by deterministic fixtures overnight; named future change. (b) Grid search over threshold candidates scored by would-have-admitted outcome EV — plausible v2 that exercises `TrialsCount > 1`; deferred to keep v1 reviewable.

### D3 — TrainingRow gains outcome fields (modified capability, additive)

The tuner needs win/loss labels, but `optvalidation.TrainingRow` carries no outcome fields. Extend it with `PnlPct float64` and `OutcomeLabel string`, mapped in `NewTrainingRow` from the joined `SimOutcome` (nil `PnlPct` → 0, which lands in the excluded breakeven class — consistent with the existing nil-handling conventions in that constructor). No pipeline stage reads the new fields; determinism and stage semantics are untouched. *Alternative:* a scanneropt-local re-join of outcomes onto `WeightedTrainingRow` — rejected: it would re-query what the pipeline already joined and create two competing join definitions.

### D4 — Evidence construction for the gate

The optimizer evaluates its proposed hard-filter region on data it did not derive it from: rows are ordered by `ScannedAt` and split 80/20 into derivation (in-sample) and holdout (out-of-sample) segments; the in-sample segment is further split into `MinFolds+1` equal chronological segments to produce walk-forward folds (train on prefix, test on next segment), re-deriving the proposal parameters per fold (fold `Params` = the fold's derived `volume_ratio_floor`, `atr_pct_ceiling`, and feature weights — feeding the gate's cross-fold stability check). The per-sample performance measure is `EVWeight × PnlPct` of rows the proposed overrides would have admitted (`volume_ratio >= floor AND atr_pct <= ceiling`); `Performance.Mean/StdDev/SampleSize` are computed over admitted rows in the respective windows. `BaselineParams` come from the current active payload (or the built-in defaults payload when `scanner_configs` is empty).

### D5 — Proposal persistence and status lifecycle

New table `scanner_config_proposals`: `id` UUID PK, `created_at`, `optimizer_run_id` text, `proposed_config_json` JSONB (the full payload), `evidence_json` JSONB, `verdict_id` UUID FK → `overfitting_verdicts.id`, `status` text CHECK ∈ {`pending_review`, `rejected_by_gate`, `promoted`}. Status is set from the gate verdict at insert: passed → `pending_review`, failed → `rejected_by_gate` (persisted anyway — failures are audit evidence). `MigrateScannerOptimizer(db)` creates only this table (additive, idempotent, `migrate.go` DO-block pattern); it requires `MigrateOverfittingCountermeasures` to have run first (FK target), documented in the migration comment.

### D6 — Never auto-applied; promotion is one operator command

No code path in `scanneropt` writes to `scanner_configs` except `Promote(proposalID)`, invoked only by the `promote` CLI subcommand (`task scanner:promote`). Promote refuses proposals whose status is not `pending_review` (so `rejected_by_gate` and already-`promoted` rows can never be applied), then inserts a new `scanner_configs` row (`config_json` = proposal payload verbatim, `optimizer_run_id` carried over, `regime` left NULL — the payload is multi-regime) and flips the proposal's status to `promoted` in the same transaction. Rollback of a bad promotion uses the existing `scanner_configs` rollback-by-id mechanism from `trading-stack-schema` — this change adds no rollback machinery.

### D7 — Loader: first real consumer of the validation pipeline

`scanneropt.LoadInput(db, window) (optvalidation.Input, error)` joins `scan_results` ⋈ `sim_outcomes` (via `scan_result_id`) within a `scanned_at` window into `TrainingRow`s and loads `feature_distributions`, `simulator_fidelity`, and `strategy_ev_weights` wholesale into `optvalidation.Input`. The optimizer run then calls `optvalidation.Run` and tunes only on `Result.Clean` — the pipeline gate is structurally unavoidable, not a convention. Testcontainers-verified (the deferred-loader note in the `optimizer-validation-pipeline` design said exactly this: validate the loader with a round-trip, not fixtures).

## Risks → mitigations

- **v1 statistics propose something silly on thin/skewed data** → the `min_labeled_samples` skip, the overfitting gate's four checks (especially minimum-sample and parameter-stability step bounds), and the human review queue all sit between derivation and promotion; nothing executes automatically.
- **Promoted configs silently do nothing** (scanner doesn't read configs yet) → explicitly flagged as a sign-off item and a named future change; `scanner:proposals` output states the promoted config's effect is pending `scanner-config-hot-swap`.
- **Divergence from the sibling strategy-optimizer contract** → both changes pin the identical lifecycle in their specs (gate verdict persisted always; statuses `pending_review`/`rejected_by_gate`/`promoted`; promote-only-by-operator-command); the shared gate types live in one package (`overfitting`).
- **Loader window mistakes (partial joins)** → loader takes an explicit half-open `[from, to)` window on `scanned_at`, inner-joins only rows with outcomes, and its tests pin counts against seeded fixtures.
- **FK ordering between migrations** → `MigrateScannerOptimizer` returns a wrapped error naming the missing `overfitting_verdicts` dependency if the FK target is absent; test covers running it on a fresh DB after the overfitting migration.

## Migration / rollback

- Forward: run `MigrateOverfittingCountermeasures` then `MigrateScannerOptimizer` (additive, idempotent; creates only `scanner_config_proposals`). Not wired into `dbutils.InitPostgres`.
- Rollback: drop `scanner_config_proposals`; `scanner_configs` rows created by promotions remain valid history and are individually rollback-able by id per `trading-stack-schema`. The two additive `TrainingRow` fields are backward-compatible with all existing `optvalidation` callers.
