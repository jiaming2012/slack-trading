# Tasks — strategy-optimizer

> Sequencing precondition: `overfitting-countermeasures` must be implemented first — this change imports `src/go/tradingstack/overfitting` (`Evidence`, `RunGate`, `VerdictStore`) and its `strategy_proposals.verdict_id` FK requires the `overfitting_verdicts` table to exist.

## 1. Proposal schema and store

- [ ] 1.1 `src/go/tradingstack/stratopt/` package scaffold: `errors.go` (sentinels: pipeline failure, gate failure, unknown proposal, not decidable, invalid status/decision), `proposal.go` — `StrategyProposal` GORM model (fields per spec: status CHECK `{rejected_by_gate, pending_review, accepted, dismissed}`, JSONB `evidence_json`, not-null `verdict_id` FK → `overfitting_verdicts.id`), `MigrateStrategyOptimizer(db)` (creates only `strategy_proposals`, idempotent; requires `MigrateOverfittingCountermeasures` to have run).
- [ ] 1.2 Wire `MigrateStrategyOptimizer` into the startup path that runs `MigrateTradingStack`, ordered after `MigrateOverfittingCountermeasures`.
- [ ] 1.3 `store.go` — `PersistRun(db, runID, proposals)` (one row per proposal, status per its gate verdict: `pending_review` on pass, `rejected_by_gate` on fail), `ListProposals(db, statusFilter)` (default `pending_review`), `GetProposal(db, id)`, `DecideProposal(db, id, decision, via, at)` (`pending_review`-only; unknown, `rejected_by_gate`, or already-decided → error, no side effects).
- [ ] 1.4 Testcontainers tests: round-trip field-for-field (incl. evidence JSON, negative adjustment, `verdict_id`), invalid status rejected, orphan `verdict_id` rejected (FK), migration idempotent and additive-only (with `overfitting_verdicts` pre-created), decide happy path + double-decide rejection + decide-on-`rejected_by_gate` rejection + unknown id, list default-`pending_review` and explicit `rejected_by_gate` filter.

## 2. Gated input assembly

- [ ] 2.1 `input.go` — `GatedOutcome` (a `tradingstack.SimOutcome` view + `EVWeight`) and `AssembleGatedOutcomes(pipelineResult optvalidation.Result, outcomes []tradingstack.SimOutcome) []GatedOutcome` keeping exactly the outcomes whose `SimOutcomeID` survives in `Result.Clean`, attaching each row's `ev_weight`; capture the pipeline gating summary (rows kept/dropped per stage) for proposal evidence.
- [ ] 2.2 Unit tests: dropped-by-fidelity-gate outcome excluded; ev_weight attached; pipeline error path aborts with zero proposals.

## 3. Weighted statistics engine

- [ ] 3.1 `stats.go` — per-`(strategy_id, regime)` grouping; weighted EV per the architecture formula (weighted win/loss rates over decided outcomes, weighted mean magnitudes, breakevens excluded, `avg_loss` positive — `ev-computation` conventions); weighted exit-reason shares among losses and among wins (with hold-day cutoffs); decided-sample counts. Pure functions, deterministic ordering.
- [ ] 3.2 Unit tests: hand-computed weighted-EV fixture (uniform weights → 0.8; loss-downweighted variant moves in the hand-computed direction); group independence across regimes; breakeven exclusion.

## 4. Rule-based candidate generation

- [ ] 4.1 `config.go` — `OptimizerConfig` with defaults: `min_group_samples = 50` (pre-filter), rule thresholds 0.5, hold cutoffs (stop-churn ≤ 2 days, fast-target ≤ 1 day), steps (+20% stop, −20% hold, +10% target), fold count 4, and the strategy-kind gate config (`MinSamples 50`, `MinOOSSamples 15`, remainder from `overfitting.DefaultConfig()`).
- [ ] 4.2 `rules.go` — R1 stop-churn → widen `stop_pct`; R2 timeout-drag → shorten `max_hold_days`; R3 fast-target → raise `target_pct`; at most one candidate per rule per group per run; rationale text + evidence (stats, sample count, gating summary); output sorted by `(strategy_id, regime, parameter)`.
- [ ] 4.3 Unit tests: stop-churn fixture yields exactly the +20% candidate with 0.7 share evidence; below-threshold yields none; 49 vs 50 decided-sample pre-filter boundary; repeated runs byte-identical.

## 5. Overfitting-gate evidence and verdict integration

- [ ] 5.1 `gate_evidence.go` — `BuildEvidence(group, candidate, cfg) overfitting.Evidence`: chronological partition of the group's gated outcomes by `simulated_at` into the configured fold count (train window precedes test window by construction); per-fold `Params` = the rule's trigger statistic on the fold's train window; `TestMetric` = test window's ev-weighted mean `pnl_pct`; in-sample = all but the most recent window, out-of-sample = the most recent window (ev-weighted mean/stddev/sample size); `BaselineParams = {param: 1.0}`, `ProposedParams = {param: 1.0 + adjustment}` (multiplier encoding); `TrialsCount = 1`; `ProposalKind = strategy`. Deterministic.
- [ ] 5.2 `generate.go` — run orchestration: assemble gated input → stats → minimum-sample pre-filter (insufficient-data groups reported, not omitted) → rules → per-candidate `BuildEvidence` → `overfitting.RunGate` with the strategy-kind config → persist verdict via the gate's `VerdictStore` → attach `verdict_id` + status (`pending_review`/`rejected_by_gate`) → `run_id` assignment; dry-run mode runs the gate in memory (in-memory `VerdictStore` fake) and persists nothing; deterministic end to end.
- [ ] 5.3 `synthetic.go` — fixtures: gate-passing stop-churn group (candidate → `pending_review`), gate-failing group (e.g. OOS collapse → `rejected_by_gate` with retention check named), below-threshold group (no candidate), insufficient-sample group (reported), multi-regime strategy; shared by tests and `--synthetic`.
- [ ] 5.4 Unit tests (gate library exercised via its in-memory `VerdictStore` fake): evidence fold geometry always satisfies `TestStart ≥ TrainEnd`; multiplier encoding puts all three default steps inside the gate's 25% step bound; gate-passing candidate persists as `pending_review` with `verdict_id` set; gate-failing candidate persists as `rejected_by_gate` and its verdict names the failing checks; negative-edge group's candidate lands `rejected_by_gate` (documented D7 consequence); determinism of evidence construction.

## 6. Command and Taskfile targets

- [ ] 6.1 `cmd/strategy-optimizer/main.go` — generate mode (`--synthetic`, `--dry-run`; real mode loads `scan_results`/`sim_outcomes` + reference tables, runs `optvalidation.Run`, generates, gates, persists via `PersistRun` unless dry-run); prints gating summary, per-group stats/eligibility, and each candidate with its per-check gate verdict and resulting status, labeled "recommendation — requires operator action"; `--list [--status]` (default `pending_review`), `--show <id>` (evidence + gate verdict), `--decide <id> --decision accepted|dismissed` (stamps `decided_via=cli`); exits non-zero only on internal error (zero proposals and all-rejected are clean outcomes).
- [ ] 6.2 `taskfile.yml` — `optimizer:propose` (bare default `--synthetic --dry-run`), `optimizer:proposals`, `optimizer:proposal` (ID=), `optimizer:proposal:decide` (ID=, DECISION=).

## 7. Gates

- [ ] 7.1 `go build ./...` green.
- [ ] 7.2 `task test` green (backtester suite unaffected).
- [ ] 7.3 `task test:trading-stack` green (Docker required; includes the new `stratopt` testcontainers suites — extend the target's package list if it is package-scoped).
- [ ] 7.4 `go test -count=1 ./src/go/tradingstack/stratopt/...` green; `task optimizer:propose` (synthetic dry-run) exits zero printing both the gate-passing and gate-rejected fixture candidates with their verdicts.

## 8. Operator-only follow-ups

- [ ] 8.1 Operator: once real `sim_outcomes` volume exists, run a real (non-synthetic) generation against the dev DB, review the first proposal batch end to end (`optimizer:proposals` → `optimizer:proposal` → `optimizer:proposal:decide`), inspect a `rejected_by_gate` example via the explicit status filter, and judge whether the default rule thresholds/steps and the strategy-kind gate config (design D7) produce sane outcomes before any acceptance is acted on outside the system.
- [ ] 8.2 Operator: decide whether/when to queue the follow-on cards — batch re-simulation (unlocks Bayesian generation per design D1 and a better evidence mapping for negative-edge groups per design D7) and a strategy-config registry (unlocks binding accepted proposals to concrete configs per design D2).
- [ ] 8.3 Flip this change's card(s) in `usm/roadmap.txt` to green `#C5E1A5` and refresh ROADMAP.md (done at archive time).
