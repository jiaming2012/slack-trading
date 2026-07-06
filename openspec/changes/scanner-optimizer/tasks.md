# Tasks — scanner-optimizer

> Sequencing: requires `overfitting-countermeasures` to be implemented first (this change imports its gate and FKs its verdict table). `trading-stack-schema` and `optimizer-validation-pipeline` are already archived.

## 1. Payload package (`scannercfg`)

- [x] 1.1 Create `src/go/tradingstack/scannercfg/` — `Payload`, `RegimeModel`, `HardFilterOverrides`, `GlobalConfig` types mirroring the architecture-doc JSON; `Parse`, `Marshal` (semantic round-trip), `Validate` (non-negative finite weights, `score_threshold` in [0,1]); `DefaultPayload()` built-in baseline used when `scanner_configs` is empty.
- [x] 1.2 Payload tests: architecture-doc example round-trips key-for-key; invalid weight and out-of-range threshold rejected; default payload validates.

## 2. TrainingRow outcome fields (modified `optimizer-validation-pipeline`)

- [x] 2.1 Extend `optvalidation.TrainingRow` with `PnlPct float64` and `OutcomeLabel string`; map them in `NewTrainingRow` (nil → 0 / empty string). No stage reads the new fields.
- [x] 2.2 Tests: outcome fields carried from the `SimOutcome`; pipeline stage survival identical with outcome fields zeroed vs populated.

## 3. Tuner (`scanneropt`)

- [ ] 3.1 Create `src/go/tradingstack/scanneropt/` with `errors.go` and `weighted_stats.go` (EV-weighted mean, pooled std dev, weighted percentile — deterministic, fixture-tested).
- [ ] 3.2 `tuner.go` — per-regime derivation: win/loss labeling (breakevens excluded), effect-size feature weights over `rsi_14`/`volume_ratio`/`atr_pct` normalized to sum 1 (zero-dispersion → 0; all-zero → carry baseline), winner-percentile overrides (p25 volume_ratio floor, p75 atr_pct ceiling), `score_threshold` carried forward, thin regimes (< `min_labeled_samples` decided) carry the baseline model forward; output is a complete `scannercfg.Payload`.
- [ ] 3.3 Tuner tests: hand-computed fixture weights/overrides match; thin regime carries baseline; empty regime tags excluded; determinism (run twice, identical payload).

## 4. Evidence builder and gate integration

- [ ] 4.1 `evidence.go` — chronological 80/20 in-sample/out-of-sample split; walk-forward folds over the in-sample segment with per-fold re-derivation (fold `Params` = fold's floor/ceiling/feature weights); per-sample measure `EVWeight × PnlPct` over admitted rows; `TrialsCount` 1; `BaselineParams` from the active or default payload.
- [ ] 4.2 `gate.go` — submit evidence to `overfitting.RunGate`, persist the verdict (pass or fail) via the `overfitting.VerdictStore`.
- [ ] 4.3 Tests: fold geometry chronological and lookahead-free on a fixture; failing evidence still yields a persisted verdict (via the in-memory fake store).

## 5. Proposal persistence and promotion

- [ ] 5.1 `proposal.go` — GORM model `ScannerConfigProposal` (table `scanner_config_proposals`; status CHECK + Go validation over `{pending_review, rejected_by_gate, promoted}`; `verdict_id` FK → `overfitting_verdicts.id`); `MigrateScannerOptimizer(db)` additive/idempotent, erroring clearly if the verdict table is missing.
- [ ] 5.2 `store.go` — `ProposalStore` interface (`Persist`, `List`, `FetchByID`, `Promote`), GORM implementation (Promote: refuse non-`pending_review`; transactionally insert `scanner_configs` row verbatim + flip status), in-memory fake.
- [ ] 5.3 `synthetic.go` — synthetic weighted-row fixtures (one healthy regime, one thin regime) shared by tests and the CLI `--synthetic` mode.
- [ ] 5.4 Testcontainers tests: passing run → `pending_review` row and `scanner_configs` row count unchanged; failing run → `rejected_by_gate`; promote pending → new `scanner_configs` row verbatim + status `promoted`; promote rejected/promoted → error, no writes; migration creates exactly `scanner_config_proposals` and is idempotent.

## 6. Loader (first real consumer of the validation pipeline)

- [ ] 6.1 `loader.go` — `LoadInput(db, from, to) (optvalidation.Input, error)`: inner-join `scan_results` ⋈ `sim_outcomes` in the half-open `scanned_at` window into `TrainingRow`s; load the three reference tables wholesale.
- [ ] 6.2 Testcontainers tests: outcome-less scan result excluded; out-of-window pair excluded; DB-backed run's tuner input matches the pipeline's documented filtering (timestamp violation and low-confidence rows excluded).

## 7. Operator entry point and Taskfile targets

- [ ] 7.1 `cmd/scanner-optimizer/main.go` — subcommands `run` (`--synthetic` default true; DB mode behind an explicit flag), `list`, `promote --id`; exits non-zero only on internal error; `list` notes that promoted configs take effect only after the future `scanner-config-hot-swap` change.
- [ ] 7.2 Add `scanner:optimize`, `scanner:proposals`, `scanner:promote`, and `test:scanner-optimizer` targets to `taskfile.yml`.

## 8. Verification gates

- [ ] 8.1 `go build ./src/go/... ./cmd/...` green.
- [ ] 8.2 `task test` green (existing backtester suite unaffected).
- [ ] 8.3 `task test:trading-stack` green (tradingstack suite incl. `optvalidation` after the row extension; requires Docker).
- [ ] 8.4 `task test:scanner-optimizer` green; `task scanner:optimize` (synthetic) and `task optimizer:validate` both run green locally.

## 9. Operator-only follow-ups

- [ ] 9.1 Operator runs the first DB-backed optimizer cycle against accrued Simulation data (`task scanner:optimize` in DB mode), reviews the proposal via `task scanner:proposals`, and decides whether to promote — promotion is never performed autonomously.
- [ ] 9.2 Operator decides when to schedule the monthly cadence (no scheduler is added by this change) and when to green-light the named future changes `scanner-config-hot-swap` and `scanner-optimizer-ml-ranking`.
