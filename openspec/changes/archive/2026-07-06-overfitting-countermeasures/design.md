# Design — overfitting-countermeasures

## Context

The full-trading-stack architecture doc gives the optimizers real teeth (Bayesian search over strategy parameters, XGBoost retraining for the scanner) but says almost nothing about how to keep them honest beyond `min_labeled_samples: 500` in the scanner-config payload and the optimizer-validation-pipeline's *data*-quality gates. Data-quality gating (already shipped in `optvalidation`) protects what the optimizers train **on**; nothing yet protects what they train **out** — a proposal that overfits clean data is still garbage. Both optimizers are being drafted now with the shared contract "proposals are recommendations, persisted for operator review, never auto-applied"; this library is the shared quality bar in front of that review queue.

## Goals

- One gate, two consumers: `scanner-optimizer` and `strategy-optimizer` submit the same `Evidence` shape and get the same `Verdict` shape, so the operator learns one vocabulary.
- Pure and deterministic: every check is a function of `(Evidence, Config)` only — no clock, no DB, no randomness — so overnight verification is synthetic-fixture unit tests, mirroring the `optvalidation` precedent.
- Every verdict persisted, including failures: a rejected proposal with its per-check numbers is audit evidence, not noise.
- Additive-only persistence: one new table, its own migration entry point, nothing existing touched (the `MigrateCrowdingDetection` / `MigrateTradingStack` pattern).

## Non-Goals

- Choosing or constraining the optimizers' search methods — this gate judges the evidence a search produces, not how the search runs.
- Blocking promotion mechanically — promotion is an operator action defined by the optimizer changes; this library only supplies the verdict those flows consult.
- Alerting on failed verdicts. No new metrics or alerts in this change; if added later they MUST use the internal telemetry registry (`src/go/telemetry` Counter/Gauge, AlertEngine) per ADR-0005 — never OpenTelemetry, which has been removed.
- ESDB event publication (ADR-0004 undecided) — Postgres only.
- A DB loader for evidence — evidence is constructed in memory by the calling optimizer.

## Decisions

### D1 — Evidence is a caller-supplied struct, not a query

The gate takes an `Evidence` value: proposal identity (`ProposalID uuid.UUID`, `ProposalKind` ∈ {`scanner`, `strategy`}), `SampleSize` (decided labeled samples used), `TrialsCount` (candidate parameterizations evaluated during the search; a direct deterministic derivation counts as 1), `InSample` and `OutOfSample` `Performance{WindowStart, WindowEnd, Mean, StdDev, SampleSize}` (Mean/StdDev of the per-sample performance measure, e.g. weighted pnl_pct), `Folds []FoldResult{Index, TrainStart, TrainEnd, TestStart, TestEnd, Params map[string]float64, TestMetric float64}`, `ProposedParams` and `BaselineParams` (`map[string]float64` of the comparable numeric knobs).

*Alternative considered:* have the gate query `scan_results`/`sim_outcomes` itself and recompute evidence. Rejected — it would duplicate each optimizer's evaluation semantics inside the gate, couple the gate to both optimizers' internals, and make it untestable without a database. The trade-off (a lying optimizer could fabricate evidence) is acceptable: both optimizers are our own deterministic code, and their own specs require the evidence to be derived from the same rows they trained on.

### D2 — Deflated performance = multiple-testing-adjusted t-statistic, not full Bailey–Prado DSR

Check: `tStat = OutOfSample.Mean / (OutOfSample.StdDev / sqrt(OutOfSample.SampleSize))` must satisfy `tStat >= max(cfg.MinTStat, sqrt(2·ln(max(TrialsCount, 1))))`. `sqrt(2·ln(N))` is the standard asymptotic expected maximum of N i.i.d. standard normals — the bar an N-way search "wins" by luck alone — so clearing it deflates for selection bias; `MinTStat` (default 2.0) keeps a floor of plain statistical significance when `TrialsCount` is small (ln(1) = 0). `StdDev <= 0` or `SampleSize < 2` fails the check (no honest dispersion evidence).

*Alternative considered:* the full Bailey & López de Prado Deflated Sharpe Ratio with skewness/kurtosis correction. Rejected for now — it requires per-sample return moments the evidence contract doesn't carry and adds real numerical-implementation risk for marginal benefit at our sample sizes. The `Performance` struct can grow `Skew`/`Kurtosis` fields later without breaking the contract; recorded as future work.

### D3 — Walk-forward check validates fold geometry and OOS retention

Requirements on the folds themselves: at least `cfg.MinFolds` (default 3); strictly increasing `Index`; each fold's `TestStart >= TrainEnd` (no lookahead within a fold); test windows in chronological order across folds. Performance requirements: `OutOfSample.Mean > 0` and `OutOfSample.Mean >= cfg.OOSRetentionFraction × InSample.Mean` (default 0.5). A non-positive `InSample.Mean` fails the check outright — a proposal that cannot even fit its training window has no business being proposed, and a retention ratio against a non-positive base is meaningless.

*Alternative considered:* requiring every individual fold's `TestMetric` to be positive. Rejected as too brittle (a single bad fold in a small count would veto structurally sound proposals); per-fold values are still recorded in the verdict detail for the operator to eyeball.

### D4 — Parameter stability = bounded step from baseline + bounded cross-fold variance

(a) For every parameter present in **both** `ProposedParams` and `BaselineParams` with a non-zero baseline: `|proposed − baseline| <= cfg.MaxRelativeStep × |baseline|` (default 0.25). A zero-valued baseline parameter is skipped by the relative test (no meaningful reference) and noted in the check detail. Parameters present in only one map are skipped and noted — new/removed knobs are a review topic, not an automatic failure.
(b) For each parameter appearing in the folds' `Params`: coefficient of variation across folds `stddev/|mean| <= cfg.MaxFoldParamCV` (default 0.5); skipped when the cross-fold mean is 0. A parameter whose fitted value swings wildly fold-to-fold is fitting noise.

*Alternative considered:* absolute per-parameter bounds tables. Rejected — they'd hard-code knowledge of each optimizer's parameter names into the shared library; relative bounds are parameter-name-agnostic.

### D5 — Verdict is all-checks-must-pass, and failures are persisted too

`RunGate(evidence, cfg)` runs the four checks in fixed order (minimum-sample → walk-forward → parameter-stability → deflated) and returns `Verdict{ProposalID, ProposalKind, Passed, Checks []CheckResult{Name, Passed, Observed, Threshold, Detail}}` with `Passed = AND` of all checks. All four checks always run (no short-circuit) so a failing proposal's verdict shows every deficiency at once. Persistence writes one `overfitting_verdicts` row per gate run — pass or fail — with the per-check results serialized to a JSONB `checks_json` column.

### D6 — Loose coupling to proposal tables: `proposal_id` is not a foreign key

`overfitting_verdicts.proposal_id` is a plain UUID column, not an FK: the two proposal tables (`scanner_config_proposals`, and the strategy-optimizer's equivalent) live in *later* changes, are two different tables, and Postgres cannot FK one column into either-of-two parents. The consuming optimizer changes hold the FK in the other direction (their proposal rows reference `overfitting_verdicts.id`). Documented deviation, mirrored in the spec.

### D7 — Config over constants

All thresholds live in a `Config` struct with `DefaultConfig()` (`MinSamples 500`, `MinOOSSamples 50`, `MinFolds 3`, `OOSRetentionFraction 0.5`, `MaxRelativeStep 0.25`, `MaxFoldParamCV 0.5`, `MinTStat 2.0`). The 500 comes from the architecture doc's `min_labeled_samples`; every other default is a conservative starting point chosen here — flagged in the proposal for operator sign-off, tunable without code change by callers constructing a non-default `Config`.

## Package and file layout

`src/go/tradingstack/overfitting/` (import path `github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting`):

- `evidence.go` — `Evidence`, `Performance`, `FoldResult`, `ProposalKind` constants.
- `config.go` — `Config`, `DefaultConfig()`.
- `check_min_samples.go`, `check_walk_forward.go`, `check_param_stability.go`, `check_deflated.go` — one pure check per file, each returning `CheckResult`.
- `gate.go` — `RunGate(evidence Evidence, cfg Config) Verdict`.
- `verdict.go` — `Verdict`, `CheckResult`; GORM model `OverfittingVerdict` (table `overfitting_verdicts`); `MigrateOverfittingCountermeasures(db *gorm.DB) error`.
- `store.go` — `VerdictStore` interface (`Persist`, `FetchLatestByProposal`), GORM impl, in-memory fake.
- `synthetic.go` — `SyntheticEvidencePass()` / `SyntheticEvidenceFail()` fixture builders shared by tests and the command.
- `errors.go`, `overfitting_test.go`.

`cmd/overfitting-check/main.go` — `--synthetic` (default true): runs the gate over the built-in passing and failing fixtures, prints per-check name/observed/threshold/pass, exits non-zero only on internal error (a failing fixture verdict is the expected demonstration, not an error).

## Risks → mitigations

- **Gate too strict at low data volume** (nothing passes early on) → every threshold is config; verdicts persist the exact observed-vs-threshold numbers so the operator can see how far off a proposal was; the minimum-sample gate failing early is the *correct* behavior per the architecture doc's build-order note (~500 rows before optimizing).
- **Evidence semantics drift between the two optimizers** (e.g. different meaning of `Mean`) → the evidence contract documents the expected measure (per-sample performance, e.g. weighted pnl_pct); each optimizer's own spec pins how it builds evidence; the shared type lives here so there is exactly one definition.
- **Zero/degenerate inputs** (StdDev 0, empty folds, empty params) → every check specifies its degenerate-input behavior (fail closed for dispersion/fold geometry, skip-and-note for unmatched params) and has a fixture test.
- **Schema creep** → migration creates exactly one table; test asserts no other table is created or altered.

## Migration / rollback

- Forward: `MigrateOverfittingCountermeasures(db)` — additive, idempotent (AutoMigrate + DO-block constraint pattern from `migrate.go`), creates only `overfitting_verdicts`. Not wired into `dbutils.InitPostgres`; callers invoke explicitly, matching the tradingstack lifecycle convention.
- Rollback: drop `overfitting_verdicts`; no other object depends on it within this change (consuming optimizer changes add their own FK toward it and must roll back first).
