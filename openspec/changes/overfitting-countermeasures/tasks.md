# Tasks — overfitting-countermeasures

## 1. Package scaffold and contracts

- [x] 1.1 Create `src/go/tradingstack/overfitting/` with `errors.go` (sentinel errors incl. `ErrUnknownProposalKind`) and `evidence.go` (`Evidence`, `Performance`, `FoldResult`, `ProposalKind` constants `scanner`/`strategy`).
- [x] 1.2 `config.go` — `Config{MinSamples, MinOOSSamples, MinFolds, OOSRetentionFraction, MaxRelativeStep, MaxFoldParamCV, MinTStat}` with `DefaultConfig()` (500, 50, 3, 0.5, 0.25, 0.5, 2.0).

## 2. Checks (pure functions, one file each)

- [x] 2.1 `check_min_samples.go` — total decided samples and OOS samples vs thresholds; at-threshold passes; result reports both observed counts.
- [x] 2.2 `check_walk_forward.go` — fold count, strictly increasing indices, no-lookahead (`TestStart >= TrainEnd`), chronological test windows, `InSample.Mean > 0`, `OutOfSample.Mean > 0`, OOS retention fraction.
- [x] 2.3 `check_param_stability.go` — step bound vs baseline (skip-and-note zero baselines and unmatched params) plus cross-fold coefficient of variation (skip zero-mean params).
- [x] 2.4 `check_deflated.go` — OOS t-statistic vs `max(MinTStat, sqrt(2·ln(max(TrialsCount,1))))`; fail closed on `StdDev <= 0` or `SampleSize < 2`; result reports observed t and effective bar.

## 3. Gate orchestration and verdict types

- [x] 3.1 `gate.go` — `RunGate(evidence, cfg) (Verdict, error)`: validates `ProposalKind`, runs all four checks in fixed order without short-circuit, `Passed = AND` of checks; deterministic.
- [x] 3.2 `verdict.go` — `Verdict`/`CheckResult` types; GORM model `OverfittingVerdict` (table `overfitting_verdicts`, `proposal_kind` CHECK + Go validation, `checks_json` JSONB); `MigrateOverfittingCountermeasures(db)` — additive, idempotent, creates only this table.
- [x] 3.3 `store.go` — `VerdictStore` interface (`Persist`, `FetchLatestByProposal`), GORM implementation, in-memory fake.
- [x] 3.4 `synthetic.go` — `SyntheticEvidencePass()` and `SyntheticEvidenceFail()` fixture builders shared by tests and the command.

## 4. Operator entry point and Taskfile targets

- [x] 4.1 `cmd/overfitting-check/main.go` — `--synthetic` (default true); runs the gate over both fixtures, prints per-check name/observed/threshold/pass and overall verdicts; exits non-zero only on internal error.
- [x] 4.2 Add `optimizer:overfitting-check` and `test:overfitting` targets to `taskfile.yml`.

## 5. Tests

- [x] 5.1 Minimum-sample gate: at-threshold passes; total-sample failure and OOS-sample failure each identified.
- [x] 5.2 Walk-forward: healthy folds pass; retention collapse fails; lookahead fold fails; too-few folds fail; non-positive in-sample mean fails outright.
- [x] 5.3 Parameter stability: within-bounds passes; oversized step names the parameter; fold-unstable parameter names the parameter; proposal-only parameter is noted, not failed; zero-baseline parameter skipped.
- [x] 5.4 Deflated: trials=1 with t 2.5 passes; trials=100 with t 2.5 fails the raised bar; zero StdDev fails closed.
- [x] 5.5 Gate: one failing check fails the verdict while all four results are present; unknown proposal kind errors; determinism (run twice, identical verdicts).
- [x] 5.6 Persistence (testcontainers): verdict row round-trips including `checks_json`; invalid `proposal_kind` rejected; migration creates exactly `overfitting_verdicts`, is idempotent, and touches no `trading-stack-schema` or playground table.
- [x] 5.7 Store: in-memory fake returns the latest verdict per proposal.

## 6. Verification gates

- [ ] 6.1 `go build ./src/go/... ./cmd/...` green. *(Blocked externally at implementation time: concurrent uncommitted safety-remediation work in `src/go/backtester/models/` does not compile; `go build ./src/go/tradingstack/... ./cmd/overfitting-check/` is green. Re-run once that work lands.)*
- [ ] 6.2 `task test` green (existing backtester suite unaffected). *(Blocked by the same external in-flight `backtester/models` compile break — no test in that suite exercises this change.)*
- [ ] 6.3 `task test:trading-stack` green (tradingstack package suite, incl. this package's testcontainers tests; requires Docker). *(All tradingstack packages pass — including `overfitting` — except `riskoverlay`, which fails to build only because it imports the externally broken `backtester/models`. Re-run once that work lands.)*
- [x] 6.4 `task test:overfitting` and `task optimizer:overfitting-check` both run green locally.

## 7. Operator-only follow-ups

- [ ] 7.1 Operator reviews the default thresholds (500/50 samples, 3 folds, 0.5 retention, 25% step, CV 0.5, t 2.0) after the first real optimizer proposals produce verdicts, and decides whether to tune the gate `Config` defaults in a follow-up change.
