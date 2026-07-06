# Design — strategy-optimizer

## Context

The architecture doc defines the Strategy Optimizer as: input `sim_outcomes`, tunes stop % / target % / hold days / sizing / entry triggers, method "Bayesian optimization over strategy parameter space", output "updated strategy config → Simulator", weekly cadence. Upstream pieces exist and are archived: `trading-stack-schema` (the `sim_outcomes` data model), `optimizer-validation-pipeline` (`src/go/tradingstack/optvalidation` — the five-stage trust gate producing `Result{Clean []WeightedTrainingRow, …}`), `ev-computation`/`ev-persistence` (`src/go/evtracker`), and the fidelity gate rows arriving via `continuous-fidelity-monitoring`. Nothing consumes the gated training data yet.

A sibling change being drafted in the same batch, `overfitting-countermeasures`, delivers the shared **proposal honesty gate** (`src/go/tradingstack/overfitting`): a deterministic four-check library (minimum samples, walk-forward/OOS retention, parameter stability, deflated t-statistic) over caller-supplied `Evidence`, with every verdict persisted to `overfitting_verdicts` keyed by `proposal_id` + `proposal_kind` — and `proposal_kind = strategy` reserved for this optimizer. Its design is explicit that data-quality gating (`optvalidation`) protects what optimizers train *on* while the honesty gate protects what they train *out*, and that both optimizer changes submit evidence to it before their proposals reach operator review. This change is a consumer of that contract (the dependency points one way; the gate imports nothing from us) and **must land after it**.

Hard constraints shaping this design: optimizer output is **recommendations only** (operator-actioned, never auto-applied to live trading), Postgres-only persistence (ADR-0004 pending), internal telemetry not OTel (ADR-0005), autonomous-run prohibitions (no live/paper orders, no prod DB, no new infra), and every operator command gets a Taskfile target.

## Goals / Non-Goals

**Goals**

1. Turn accumulated sim-outcome history into concrete, evidenced, per-`(strategy_id, regime)` parameter proposals.
2. Consume only validation-pipeline-gated data, carrying `ev_weight` into every statistic.
3. Every proposal obtains an `overfitting-countermeasures` verdict before it can enter operator review; gate-rejected proposals are audit-persisted (`rejected_by_gate`) and never queued.
4. Persist proposals as recommendations with full provenance (evidence + gate verdict); operator lists, inspects, and records decisions on the `pending_review` queue.
5. Hard invariant: no code path applies a proposal to any strategy configuration or trading behavior.
6. Deterministic and fixture-testable end to end (synthetic mode, testcontainers for persistence).

**Non-Goals**

- Bayesian/search optimization over re-simulated parameter candidates (see D1).
- Applying accepted proposals anywhere (see D2) — including writing to `options-config.yaml`, `scanner_configs`, or any playground table.
- The Scanner Optimizer (separate component and card), XGBoost/SHAP, Python involvement.
- Scheduling optimizer runs (operator-invoked via task target; a scheduler can ride later once cadence is proven).
- Position sizing and entry-trigger rules (arch doc lists them; v1 rules cover stop/target/hold where `sim_outcomes` carries direct evidence — sizing needs capacity/portfolio context owned by other capabilities).

## Decisions

### D1 — Deterministic rule-based generation v1, not Bayesian optimization (FLAGGED for sign-off)

The arch doc says "Bayesian optimization over strategy parameter space". Real Bayesian tuning requires evaluating candidate parameter sets — i.e., **re-simulating** each candidate against history. Batch re-simulation does not exist (the same gap deferred by the fidelity checker), so a Bayesian loop today would score candidates against nothing. v1 instead derives directional proposals straight from the outcome evidence the arch doc itself narrates ("If most losses are `stop` hits within 1–2 days → entry timing is off…", "If losses are `timeout` → catalyst isn't materializing fast enough"):

- **R1 stop-churn → widen stop**: among losses, when the ev-weighted share exited via `stop` with `hold_days ≤ 2` is ≥ 0.5 → propose `stop_pct` +20% (relative).
- **R2 timeout-drag → shorten hold**: among losses, when the ev-weighted share exited via `timeout` is ≥ 0.5 → propose `max_hold_days` −20%.
- **R3 fast-target → raise target**: among wins, when the ev-weighted share exited via `target` with `hold_days ≤ 1` is ≥ 0.5 → propose `target_pct` +10%.

All thresholds, hold-day cutoffs, and step magnitudes are config with the stated defaults. One proposal max per rule per group per run; steps are fixed (no compounding within a run). Pure functions over gated rows → byte-identical output on identical input.

- *Alternative — Bayesian via scikit-optimize (Python)*: rejected for v1 — needs re-simulation to score candidates, adds a Python/gRPC surface, and its stochastic search defeats deterministic verification. Becomes viable after a `simulator-batch-rerun` capability exists; the proposal store is method-agnostic (proposals carry `rule`/method provenance), so a Bayesian generator later persists into the same table.
- *Alternative — grid search over historical outcomes*: rejected — you cannot evaluate a *different* stop/target against recorded outcomes without per-trade price paths, which `sim_outcomes` does not store.
- Consequence for the operator: v1 proposals are directional nudges with evidence, not optimal points. Honest, inspectable, and safe.

### D2 — Proposals are relative adjustments; no binding to a config store

A proposal says "widen `stop_pct` by 20%", not "set stop to 4.2%". There is no canonical Go-side strategy-config store to read current parameter values from (`options-config.yaml` is the Python covered-call strategy's config; `scanner_configs` is scanner-side). Inventing one now would smuggle in the apply path this change must not have.

- *Alternative — absolute proposed values*: rejected — nothing authoritative to compute them from.
- *Alternative — build a strategy-config registry in this change*: rejected — scope creep and it creates the very auto-apply surface the constraints prohibit. A future `strategy-config-registry` change can bind accepted proposals to concrete configs.

### D3 — Never-auto-applied is a spec invariant, not just a convention

The only writes the optimizer performs are inserts into `strategy_proposals` and status updates on decision. `accepted` is a recorded operator judgment; no watcher, hook, or startup path reads proposal status to alter behavior. This is spec'd (see `optimizer-proposals`) so any future "apply" feature must arrive as its own signed-off change that modifies the requirement.

### D4 — Gating integration: join `optvalidation.Result` back to outcomes by `SimOutcomeID`

`WeightedTrainingRow` carries `SimOutcomeID` and `EVWeight` but not the outcome fields the rules need (`pnl_pct`, `exit_reason`, `hold_days`). The optimizer takes the pipeline `Result` plus the full `[]tradingstack.SimOutcome` batch and keeps exactly the outcomes whose IDs appear in `Result.Clean`, attaching each row's `ev_weight`. A pipeline error aborts the run with no proposals.

- *Alternative — extend `TrainingRow` with outcome fields*: rejected — modifies the shipped `optimizer-validation-pipeline` capability (delta spec, re-verification) for what a join by ID gives us for free.
- *Alternative — re-implement the gates inside the optimizer*: rejected — two divergent trust gates is exactly the failure mode the pipeline exists to prevent.

### D5 — Statistics are ev-weight-weighted

Every per-group statistic uses `ev_weight` as the row weight: weighted EV per the arch formula (`EV = (win_rate × avg_win) − (loss_rate × avg_loss)` with weighted rates and weighted means, breakevens excluded per the `ev-computation` convention), and weighted exit-reason shares for the rules. A decaying strategy's stale rows (weight 0.3) thus influence proposals less, which is the entire point of the EV-weight join stage. Computed locally in `stratopt` (the shapes differ from `evtracker`'s unweighted-trade windows); `evtracker` conventions (breakeven handling, avg_loss as positive magnitude) are reused, documented, and fixture-pinned.

- *Alternative — unweighted stats, weights only reported*: rejected — silently discards the pipeline's trust signal at the moment of decision.

### D6 — Minimum-sample **pre-filter**: 50 decided outcomes per group (default)

Groups with fewer than 50 decided (non-breakeven) gated outcomes generate no candidates and are reported as insufficient-data. This is the candidate-generation *pre-filter* — cheap noise suppression before any evidence is built — not the final honesty verdict, which belongs to the shared overfitting gate (D7): the gate's own minimum-sample check re-verifies sample sufficiency on the submitted evidence, and its verdict is what admits a proposal to review. The arch doc's "~500 rows minimum" is guidance for the optimizer *phase* across all groups; per-group 50 keeps early regimes from wasting gate submissions while not requiring months of history per regime. Configurable; flagged as a judgment call.

### D7 — Overfitting-gate integration: evidence mapping and a strategy-kind gate config (FLAGGED for sign-off)

Every candidate that survives the pre-filter is submitted to `overfitting.RunGate` as `proposal_kind = strategy` before persistence; the verdict is persisted through the gate's `VerdictStore` and the proposal row records `verdict_id` and the status the verdict dictates (`pending_review` on pass, `rejected_by_gate` on fail). The gate's evidence is caller-supplied by design, so this change pins how the strategy optimizer builds it from the *same gated outcome rows the proposal was derived from*:

- **Walk-forward folds**: the group's gated outcomes are partitioned chronologically (by `simulated_at`) into K equal time buckets (default 4, satisfying the gate's ≥3-fold default); fold *i* trains on bucket *i* and tests on bucket *i+1*, so `TestStart ≥ TrainEnd` holds by construction. `FoldResult.TestMetric` = the test bucket's ev-weighted mean `pnl_pct`; `FoldResult.Params` carries the rule's trigger statistic recomputed on that fold's train window (e.g. `stop_churn_share`), so the gate's cross-fold CV check verifies the *condition* driving the proposal is temporally stable, not an artifact of one period.
- **In-sample / out-of-sample performance**: in-sample = ev-weighted mean/stddev of `pnl_pct` over all but the most recent bucket; out-of-sample = the most recent bucket. The gate's retention and deflated-t checks therefore verify the group's measured edge is real, positive, and holds up out of sample.
- **Parameter-step encoding**: proposals are relative adjustments (D2), so `BaselineParams = {parameter: 1.0}` and `ProposedParams = {parameter: 1.0 + adjustment}` (multiplier encoding). The gate's default 25% relative-step bound then bounds the adjustment magnitude directly — and all three v1 rule steps (+20%, −20%, +10%) sit inside it by construction.
- **TrialsCount = 1**: v1 generation is a direct deterministic derivation, not a search, which the gate's contract explicitly scores as 1 trial (deflated bar = the plain `MinTStat` floor).
- **Strategy-kind gate config**: gate thresholds are configurable per proposal kind, and strategy groups are far thinner than scanner training data — the gate library's scanner-scale defaults (500 samples, 50 OOS) would reject every early strategy group mechanically. The strategy-kind defaults supplied by this optimizer: `MinSamples 50` (aligned with the D6 pre-filter so the gate re-verifies the same bar on the evidence itself), `MinOOSSamples 15`, all other thresholds inherited from the gate's `DefaultConfig()` (`MinFolds 3`, `OOSRetentionFraction 0.5`, `MaxRelativeStep 0.25`, `MaxFoldParamCV 0.5`, `MinTStat 2.0`). Conservative starting points, operator-tunable, **flagged for sign-off**.

Known consequence, stated honestly: the gate fails evidence whose in-sample mean is non-positive, so v1 proposals on negative-edge groups (exactly where a stop-churn fix might help most) will land as `rejected_by_gate` — visible in the audit trail with the retention check named, never silently dropped. Refining the evidence mapping for turnaround proposals is deferred until re-simulation can measure a candidate's own performance rather than the group's.

- *Alternative — treat the D6 pre-filter as sufficient and skip the gate*: rejected — the cross-batch contract is that the shared gate is the single honesty bar in front of every optimizer review queue; two optimizers with private bars is the divergence the gate exists to prevent.
- *Alternative — submit only rule trigger statistics without performance evidence*: rejected — it would hollow out the gate's retention and deflated-t checks for strategy proposals; if the group's edge is not demonstrably real, tuning its execution is noise.
- *Alternative — persist gate-rejected candidates nowhere*: rejected — the gate's own design persists every verdict for audit; a proposal row in `rejected_by_gate` keeps proposal and verdict navigable together.

### D8 — Status vocabulary aligned with scanner-optimizer: `rejected_by_gate | pending_review | accepted | dismissed`

The two optimizers share the status core: `rejected_by_gate` (failed the overfitting gate; never queued for review) and `pending_review` (passed the gate; awaiting the operator). Terminal states deliberately differ: `scanner-optimizer` uses `promoted | dismissed` because a promoted scanner proposal has a real apply target (`scanner_configs` hot-swap per the architecture doc); this optimizer uses `accepted | dismissed` because there is **no apply target** (D2/D3) — `accepted` records the operator's judgment and nothing more. Keeping the shared core identical means the operator learns one gate vocabulary; keeping the terminal states distinct means the words never overpromise ("promoted" implies deployment; "accepted" does not).

- *Alternative — fully identical vocabularies (use `promoted` here too)*: rejected — `promoted` would imply an apply step this change is prohibited from having.
- *Alternative — keep the original `pending` name*: rejected — `pending_review` is the cross-batch shared term and makes "passed the gate" explicit in the name.

### D9 — New `strategy_proposals` table via in-package additive migration

Model `StrategyProposal` → table `strategy_proposals`: `id` (UUID PK), `created_at`, `run_id` (UUID grouping one generation run), `strategy_id`, `regime`, `parameter` (e.g. `stop_pct`), `adjustment_pct` (signed relative step), `rule` (provenance, e.g. `stop_churn`), `rationale` (one-line human text), `evidence_json` (JSONB: weighted EV, exit-reason shares, decided-sample count, pipeline gating summary — rows kept/dropped per stage), `verdict_id` (UUID FK → `overfitting_verdicts.id`, not null — every persisted proposal has a verdict, pass or fail; this is the FK direction the gate's own design specifies), `status` (`rejected_by_gate|pending_review|accepted|dismissed`, CHECK-constrained), `decided_at`, `decided_via`. `MigrateStrategyOptimizer(db)` creates only this table (precedent: `MigrateNetEvCostModel`); `MigrateTradingStack`, `MigrateOverfittingCountermeasures`, and playground migrations untouched — but the strategy-optimizer migration requires `overfitting_verdicts` to exist first (sequencing).

- *Alternative — reuse `scanner_configs`*: rejected — that table versions *applied* scanner configs (different component, different lifecycle); proposals are pending recommendations with a decision workflow.
- Decisions are one-way: only `pending_review` rows can be decided; deciding a `rejected_by_gate` or already-terminal row is an error (an operator who changes their mind gets a fresh proposal from the next run — keeps the audit trail honest).

### D10 — Operator surface via the Go command wrapped by task targets

`cmd/strategy-optimizer` modes: generate (default; `--synthetic` fixtures, `--dry-run` print-only), `--list [--status=…]` (default `pending_review` — the review queue; `rejected_by_gate` rows appear only under an explicit `--status=rejected_by_gate` filter), `--show <id>` (evidence + per-check gate verdict), `--decide <id> --decision accepted|dismissed` (stamps `decided_via=cli`; `pending_review` only). Task wrappers: `optimizer:propose` (defaults `--synthetic --dry-run` so the bare invocation is a no-DB self-check, matching `fidelity:check`/`optimizer:validate` precedent), `optimizer:proposals`, `optimizer:proposal` (ID=), `optimizer:proposal:decide` (ID=, DECISION=). Exit non-zero only on internal error — "no proposals generated" and "all candidates gate-rejected" are normal reportable outcomes.

- *Alternative — psql-based list/show like `telemetry:status`*: rejected — list/show/decide share store logic that must be Go-tested anyway (testcontainers); one implementation, task-wrapped.

## Risks → Mitigations

- **Rule thresholds produce silly proposals on thin/skewed data** → minimum-sample pre-filter, ev-weighting, bounded single-step magnitudes, the shared overfitting gate's four checks, and the operator decision gate; every proposal carries its evidence and gate verdict so a bad one is inspectable and dismissible.
- **Operators treat proposals as authoritative** → output labeling states "recommendation — requires operator action"; rationale text names the rule and the evidence; nothing changes until a human acts *outside* this system.
- **The strategy-kind gate config is too strict or too loose at early data volumes** → every threshold is config (D7); verdicts persist observed-vs-threshold numbers per check, so the operator can see exactly how far off rejected proposals were before tuning; the negative-edge consequence is documented in D7 rather than papered over.
- **Proposal spam across repeated runs** → one proposal per rule/group/run; re-runs create new rows only for a new `run_id` — the list surface shows the `pending_review` queue grouped by run, gate-rejected rows stay out of the queue, and dismissing is one command. (Dedup-against-open-proposals considered and deferred: silently suppressing regenerated proposals hides that the condition persists.)
- **Determinism drift via map iteration** → groups and proposals sorted by `(strategy_id, regime, parameter)`; determinism test pins byte-identical repeated runs.
- **Empty reference tables early on** → the pipeline already degrades gracefully (neutral weight 1.0, zero fidelity drops); optimizer inherits that behavior and the synthetic mode never needs a DB.

## Migration / Rollback

- **Migration**: `MigrateStrategyOptimizer(db)` — creates `strategy_proposals` (+ status CHECK + `verdict_id` FK to `overfitting_verdicts`); idempotent; additive only; wired into the same startup path as `MigrateTradingStack`, ordered after `MigrateOverfittingCountermeasures` (the FK's parent table must exist — the sequencing dependency in code form). No backfill (new table).
- **Rollback**: revert commits; `DROP TABLE strategy_proposals` removes all state (drop it before any rollback of `overfitting_verdicts`, per the gate design's rollback note). Nothing else references the table (guaranteed by D3), so rollback cannot strand behavior; orphaned verdict rows are harmless audit data.
