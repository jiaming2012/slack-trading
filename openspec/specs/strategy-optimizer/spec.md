# strategy-optimizer Specification

## Purpose
TBD - created by archiving change strategy-optimizer. Update Purpose after archive.
## Requirements
### Requirement: Optimizer consumes only validation-pipeline-gated outcomes

The system SHALL feed the strategy optimizer exclusively through the `optimizer-validation-pipeline`: a generation run SHALL first run the pipeline orchestration over the candidate `scan_results`/`sim_outcomes` join with the available reference data, and SHALL then use only the sim outcomes whose identifiers appear in the pipeline's clean weighted output, attaching each surviving outcome's `ev_weight`. An outcome excluded by any pipeline stage (timestamp audit, regime-confidence filter, or fidelity gate) SHALL NOT influence any statistic or proposal. When the pipeline returns an error the run SHALL abort with no proposals generated.

#### Scenario: An outcome dropped by the fidelity gate influences nothing

- **WHEN** a generation run's input contains two outcomes for strategy `S1`, one falling inside a `within_tolerance = false` fidelity period and one outside it
- **THEN** the optimizer's statistics for `S1` are computed from the surviving outcome only

#### Scenario: Surviving outcomes carry their pipeline EV weight

- **WHEN** the pipeline joins `ev_weight = 0.3` onto a surviving row for strategy `S1`, regime `trend`
- **THEN** the optimizer's statistics weight that outcome by 0.3

#### Scenario: A pipeline error aborts the run without proposals

- **WHEN** the validation pipeline returns an error for a generation run
- **THEN** the run produces zero proposals and reports the failure

### Requirement: Weighted outcome statistics per strategy and regime

The system SHALL compute, independently per `(strategy_id, regime)` group of gated outcomes, deterministic ev-weight-weighted statistics: weighted EV per the architecture formula (`EV = (win_rate × avg_win) − (loss_rate × avg_loss)`) using weighted win/loss rates over decided outcomes and weighted mean win/loss magnitudes, with breakeven outcomes (`pnl == 0`) excluded from win and loss counts and `avg_loss` expressed as a positive magnitude (matching the `ev-computation` conventions); plus the weighted exit-reason shares among losses and among wins that the generation rules consume. The statistics SHALL be pure functions of the gated rows — no clock, randomness, database, or network.

#### Scenario: Weighted EV matches a hand-computed fixture

- **WHEN** a group has, at uniform weight 1.0, six winning outcomes averaging +2.0 and four losing outcomes averaging −1.0
- **THEN** the group's weighted EV equals 0.8

#### Scenario: EV weights change the statistics

- **WHEN** the same fixture is recomputed with every losing outcome weighted 0.5 and every winning outcome weighted 1.0
- **THEN** the weighted loss rate and weighted EV differ from the uniform-weight run in the hand-computed direction (losses count for less)

#### Scenario: Groups are independent

- **WHEN** outcomes for one `strategy_id` span two regimes
- **THEN** statistics are computed separately per `(strategy_id, regime)` and no outcome from one regime affects the other group

### Requirement: Minimum-sample pre-filter per group

The system SHALL generate no candidate proposals for a `(strategy_id, regime)` group whose count of decided (non-breakeven) gated outcomes is below a configurable minimum, defaulting to 50, and SHALL report such groups as insufficient-data in the run output rather than silently omitting them. This pre-filter is a candidate-generation economy measure only: it SHALL NOT substitute for the overfitting gate, whose verdict remains the final honesty bar for every candidate that is generated.

#### Scenario: A group below the minimum yields no candidates and is reported

- **WHEN** a generation run processes a group with 49 decided outcomes exhibiting extreme stop-churn
- **THEN** no candidate proposal is generated for that group
- **AND** the run output reports the group as insufficient-data with its sample count

#### Scenario: A group at the minimum is eligible

- **WHEN** a generation run processes a group with exactly 50 decided outcomes
- **THEN** the group is evaluated by the generation rules
- **AND** any candidate it produces is still submitted to the overfitting gate before persistence

### Requirement: Deterministic rule-based candidate generation

The system SHALL generate candidate proposals per eligible `(strategy_id, regime)` group via three deterministic rules with configurable thresholds and step magnitudes (defaults stated): **stop-churn** — when the weighted share of losses that exited via `stop` with `hold_days ≤ 2` is at least 0.5, propose increasing `stop_pct` by +20% relative; **timeout-drag** — when the weighted share of losses that exited via `timeout` is at least 0.5, propose decreasing `max_hold_days` by −20% relative; **fast-target** — when the weighted share of wins that exited via `target` with `hold_days ≤ 1` is at least 0.5, propose increasing `target_pct` by +10% relative. Each rule SHALL emit at most one proposal per group per run; each proposal SHALL carry the strategy id, regime, parameter, signed relative adjustment, originating rule, a human-readable rationale, and supporting evidence (the group's weighted statistics, decided-sample count, and the pipeline gating summary). Generation SHALL be deterministic: identical gated input SHALL produce identical proposals in identical order (sorted by strategy id, regime, then parameter).

#### Scenario: Stop-churn fixture produces a widen-stop proposal with evidence

- **WHEN** an eligible group's losses are 70% (weighted) quick stop-outs (`stop`, `hold_days ≤ 2`)
- **THEN** exactly one stop-churn proposal is generated for the group, proposing `stop_pct` +20% relative
- **AND** its evidence carries the 0.7 weighted share, the group's weighted EV, and the decided-sample count

#### Scenario: Below-threshold evidence generates nothing

- **WHEN** an eligible group's weighted quick-stop-out share among losses is 0.4 and no other rule condition is met
- **THEN** the run generates zero proposals for that group

#### Scenario: Repeated runs on identical input are byte-identical

- **WHEN** a generation run executes twice over the same gated fixture and configuration
- **THEN** both runs produce identical proposals in identical order

### Requirement: Overfitting-gate verdict before operator review

The system SHALL submit every generated candidate proposal to the shared `overfitting-countermeasures` gate before it can enter the operator review queue, using `proposal_kind = strategy` (the kind the gate's verdict table reserves for this optimizer). The optimizer SHALL construct the gate's caller-supplied evidence deterministically from the same gated outcome rows the candidate was derived from: chronological walk-forward folds partitioned by `simulated_at` (default 4 folds; each fold's train window precedes its test window by construction, with the fold's rule trigger statistic as its per-fold parameter and the test window's ev-weighted mean `pnl_pct` as its test metric); in-sample performance from all but the most recent fold window and out-of-sample performance from the most recent window (ev-weighted mean, standard deviation, and sample size of `pnl_pct`); parameter step encoded as a multiplier (`BaselineParams` = 1.0, `ProposedParams` = 1.0 + adjustment); and `TrialsCount` = 1 (direct deterministic derivation). The gate SHALL be invoked with a strategy-kind configuration whose defaults are `MinSamples` 50 and `MinOOSSamples` 15 with all other thresholds inherited from the gate library's defaults, every threshold configurable. The gate's verdict SHALL be persisted through the gate's verdict store, and the candidate SHALL be persisted with the verdict's identifier and a status determined solely by the verdict: `pending_review` when the verdict passed, `rejected_by_gate` when it failed. A `rejected_by_gate` proposal SHALL never appear in the operator review queue.

#### Scenario: A gate-passing candidate enters the review queue

- **WHEN** a generated candidate's evidence passes all four gate checks
- **THEN** its verdict is persisted, and the proposal is persisted with status `pending_review` referencing that verdict

#### Scenario: A gate-failing candidate is audit-persisted but never queued

- **WHEN** a generated candidate's evidence fails at least one gate check
- **THEN** its verdict (with the failing checks identified) is persisted, and the proposal is persisted with status `rejected_by_gate` referencing that verdict
- **AND** the proposal does not appear in the default operator review listing

#### Scenario: Evidence derives from the candidate's own gated rows

- **WHEN** the evidence for a candidate of group `(S1, trend)` is constructed
- **THEN** its folds, in-sample, and out-of-sample performance are computed only from the pipeline-gated outcomes of `(S1, trend)`, partitioned chronologically with every fold's test window not preceding its train window

#### Scenario: The multiplier encoding bounds the adjustment via the gate's step check

- **WHEN** a candidate proposing a +20% relative adjustment is submitted with baseline parameter 1.0 and proposed parameter 1.2
- **THEN** the gate's parameter-stability step check evaluates a 20% relative move, within the gate's default 25% bound

### Requirement: Operator run entry point with synthetic mode and Taskfile target

The system SHALL provide a runnable command that executes a generation run and prints, per run: the pipeline gating summary (rows kept and dropped per stage), each group's statistics and eligibility, and each candidate proposal with its rationale and its overfitting-gate verdict (per-check pass/fail and the resulting status) — labeled as a recommendation requiring operator action. The command SHALL support `--synthetic` (built-in fixtures, no database) and `--dry-run` (print without persisting, including running the gate in memory without persisting its verdict), SHALL exit non-zero only on an internal error (generating zero proposals, or having every candidate rejected by the gate, are normal outcomes), and SHALL be wired to a `task optimizer:propose` Taskfile target whose bare invocation defaults to `--synthetic --dry-run`.

#### Scenario: Bare task invocation is a no-DB self-check

- **WHEN** an operator runs `task optimizer:propose` with no database available
- **THEN** the command runs the synthetic fixtures in dry-run mode, prints the gating summary, the generated candidates, and each candidate's gate verdict, persists nothing, and exits zero

#### Scenario: Zero proposals is a clean outcome

- **WHEN** a run completes where no group meets any rule threshold
- **THEN** the command reports zero proposals generated and exits zero

#### Scenario: All candidates gate-rejected is a clean outcome

- **WHEN** a run completes where every generated candidate fails the overfitting gate
- **THEN** the command reports the candidates as `rejected_by_gate` with their failing checks and exits zero

