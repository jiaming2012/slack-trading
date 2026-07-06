# Tasks — shadow-config-deployment

> Sequencing: last of the three-change batch — requires `overfitting-countermeasures` and `scanner-optimizer` to be implemented first (imports `scannercfg.Payload`, reads and FKs `scanner_config_proposals`).

## 1. Package scaffold and decision engine

- [x] 1.1 Create `src/go/tradingstack/shadowdeploy/` with `errors.go` (sentinels incl. `ErrNotSimulation`, `ErrNoObservations`, `ErrProposalRejectedByGate`) and `observation.go` (`Observation` lifted from a persisted `scan_results` row; nullable features as pointers).
- [x] 1.2 `engine.go` — `EvaluateConfig(payload, obs) []Decision`: admission via regime `hard_filter_overrides` (absent override / nil feature does not reject), per-feature batch min-max normalization, weighted-sum scoring excluding `drop_features` (nil contributes 0, noted), threshold + top-N selection with ticker-ascending ties, `no_model` marking. Pure, deterministic.
- [x] 1.3 Engine tests: hand-computed fixture decisions; floor rejection; `no_model` visible but never selected; determinism incl. top-N tie-break; nil-feature handling.

## 2. Divergence and outcome comparison

- [x] 2.1 `divergence.go` — `CompareDecisions(active, shadow) DivergenceReport`: selected-set membership counts, shadow-only/active-only ticker lists with both scores, `DivergencePct` over the union with the max(1, …) guard; deterministic ordering.
- [x] 2.2 `outcomes.go` — coverage-explicit per-side outcome comparison (selection count, coverage count/pct, decided count, win rate, mean pnl over covered), breakeven exclusion per ev-computation conventions; never imputes outcomes.
- [x] 2.3 Tests: {A,B,C} vs {B,C,D} → 50% divergence fixture; identical selections → 0%; empty selections → 0% without division by zero; seeded outcome summary (75% coverage, 2/3 win rate); uncovered selection affects coverage only.

## 3. Run orchestration, guards, and baseline resolution

- [x] 3.1 `run.go` — `RunShadow(mode, db/store, proposalID, from, to)`: Simulation-only guard first; refuse `rejected_by_gate` proposals (accept `pending_review` and `promoted`); load observations once for the half-open window (`ErrNoObservations` on empty, persist nothing); resolve active payload (latest `scanner_configs` by `created_at`, else `scannercfg.DefaultPayload()` with null active id); evaluate both sides over the identical slice; compare; persist.
- [x] 3.2 Tests: Paper and Margin refused with nothing persisted; rejected-by-gate proposal refused; promoted proposal accepted; empty-window sentinel; empty `scanner_configs` falls back to default baseline with null active id; both decision vectors cover identical tickers.

## 4. Persistence

- [x] 4.1 `models.go` — GORM models `ShadowRun` (`shadow_runs`) and `ShadowDivergence` (`shadow_divergences`, `kind` CHECK + Go validation); `MigrateShadowDeployment(db)` additive/idempotent (DO-block pattern), clear wrapped error if `scanner_config_proposals` is absent.
- [x] 4.2 `store.go` — `ShadowStore` interface (`PersistRun`, `FetchRun`, `ListRuns`), GORM implementation, in-memory fake.
- [x] 4.3 `synthetic.go` — fixture payload pair (active + divergent shadow) and observation batch shared by tests and the CLI `--synthetic` mode.
- [x] 4.4 Testcontainers tests: run + divergences round-trip; invalid `kind` rejected; migration creates exactly the two tables, idempotent, alters nothing else; completed run leaves `scanner_configs`/`scanner_config_proposals` (incl. proposal status) untouched.

## 5. Operator entry point and Taskfile targets

- [x] 5.1 `cmd/shadow-scan/main.go` — subcommands `run` (`--synthetic` default true; DB mode behind an explicit flag with proposal id + window) and `report --run <uuid>` (divergence + coverage-explicit outcome evidence + the replayed-observations limitation note); exits non-zero only on internal error, never on high divergence.
- [x] 5.2 Add `scanner:shadow`, `scanner:shadow-report`, and `test:shadow-deployment` targets to `taskfile.yml`.

## 6. Verification gates

- [x] 6.1 `go build ./src/go/... ./cmd/...` green.
- [x] 6.2 `task test` green (existing backtester suite unaffected).
- [x] 6.3 `task test:trading-stack` green (tradingstack suite incl. this package's testcontainers tests; requires Docker).
- [x] 6.4 `task test:shadow-deployment` green; `task scanner:shadow` (synthetic) runs green locally.

## 7. Operator-only follow-ups

- [ ] 7.1 Operator runs the first real shadow cycle in Simulation against a pending proposal (`task scanner:shadow` in DB mode), reads `task scanner:shadow-report`, and decides whether to promote via `task scanner:promote` — promotion is never performed autonomously and is not part of this change.
- [ ] 7.2 Operator decides when to green-light the named future changes `scanner-config-hot-swap` (live config consumption; removes the replayed-observations limitation) and `shadow-outcome-backfill` (queue shadow-only selections for simulation so outcome coverage becomes complete).
