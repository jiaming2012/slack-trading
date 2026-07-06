# optimizer-proposals Specification

## Purpose
TBD - created by archiving change strategy-optimizer. Update Purpose after archive.
## Requirements
### Requirement: Proposal persistence model and additive migration

The system SHALL provide a `StrategyProposal` model mapped to a new table `strategy_proposals` with fields: `id` (UUID primary key), `created_at`, `run_id` (UUID grouping the proposals of one generation run), `strategy_id`, `regime`, `parameter`, `adjustment_pct` (signed relative adjustment), `rule` (originating rule name), `rationale` (human-readable text), `evidence_json` (JSONB carrying the weighted statistics, decided-sample count, and pipeline gating summary), `verdict_id` (UUID, not null, foreign key referencing the `overfitting-countermeasures` capability's `overfitting_verdicts.id` — every persisted proposal carries its gate verdict, pass or fail), `status` constrained to exactly `{rejected_by_gate, pending_review, accepted, dismissed}` (database CHECK plus Go validation; the persisted status of a new proposal is `pending_review` when its gate verdict passed and `rejected_by_gate` when it failed — the shared status core with `scanner-optimizer`, whose terminal states differ because it has a real apply target), `decided_at`, and `decided_via`. The system SHALL provide an idempotent migration `MigrateStrategyOptimizer` that creates only this table and its constraints, SHALL NOT create, alter, or drop any of the six trading-stack tables, the `overfitting_verdicts` table, or any playground table, and SHALL require `overfitting_verdicts` to already exist (the foreign key's parent — the sequencing dependency on `overfitting-countermeasures`). Persisting a generation run SHALL write one row per generated proposal, all sharing the run's `run_id`.

#### Scenario: A persisted proposal round-trips field-for-field

- **WHEN** a proposal with evidence JSON, a signed negative `adjustment_pct`, a `verdict_id` referencing a persisted verdict, and status `pending_review` is persisted against a testcontainers PostgreSQL and re-read by id
- **THEN** every field, including the deserialized `evidence_json` keys and values and the `verdict_id`, equals what was written

#### Scenario: An invalid status is rejected

- **WHEN** a proposal row with status outside `{rejected_by_gate, pending_review, accepted, dismissed}` is saved
- **THEN** the save returns a non-nil error and no row is written

#### Scenario: A proposal without a persisted verdict is rejected

- **WHEN** a proposal row whose `verdict_id` references no existing `overfitting_verdicts` row is saved
- **THEN** the save returns a foreign-key violation error and no row is written

#### Scenario: Migration creates only strategy_proposals and is idempotent

- **WHEN** `MigrateStrategyOptimizer` runs twice against a database prepared by `MigrateTradingStack` and `MigrateOverfittingCountermeasures`
- **THEN** both calls return a nil error, `strategy_proposals` exists, and no other table is created, altered, or dropped

### Requirement: Proposals are recommendations only and are never auto-applied

Optimizer proposals SHALL be recommendations requiring operator action: no code path SHALL read `strategy_proposals` (in any status) to modify any strategy configuration, scanner configuration, playground, order flow, or live/paper/simulation trading behavior. Recording a decision SHALL change only the proposal row itself (`status`, `decided_at`, `decided_via`); the system SHALL contain no automatic apply, no watcher, and no startup hook keyed on proposal status. Any future capability that applies accepted proposals SHALL arrive as its own spec change modifying this requirement.

#### Scenario: Accepting a proposal changes only the proposal row

- **WHEN** a `pending_review` proposal is decided `accepted`
- **THEN** the proposal row's `status`, `decided_at`, and `decided_via` are updated
- **AND** no other table, configuration store, or trading behavior is modified

#### Scenario: A full generate-and-persist run writes only proposal and verdict rows

- **WHEN** a generation run persists its proposals against a database containing trading-stack and playground tables
- **THEN** the only rows written by the run are `strategy_proposals` rows and their `overfitting_verdicts` rows (written through the gate's own verdict store)

### Requirement: Operator surface to list, inspect, and decide proposals

The system SHALL provide operator commands, each wrapped by a Taskfile target, to: list proposals (`task optimizer:proposals`) showing id, created time, strategy, regime, parameter, adjustment, rule, and status, defaulting to the `pending_review` review queue with an optional status filter (`rejected_by_gate` rows are visible only via that explicit filter — they never appear in the default queue); inspect one proposal (`task optimizer:proposal ID=<id>`) printing its full rationale, evidence, and its overfitting-gate verdict with per-check results; and record a decision (`task optimizer:proposal:decide ID=<id> DECISION=accepted|dismissed`) stamping `decided_at` and `decided_via`. A decision SHALL be permitted only on a `pending_review` proposal — deciding an unknown id, a `rejected_by_gate` proposal, or an already-decided proposal SHALL fail with a clear error and no side effects. The commands SHALL exit non-zero only on an error (an empty list is a normal outcome reported cleanly).

#### Scenario: Listing defaults to the pending_review queue

- **WHEN** the store holds two `pending_review`, one `rejected_by_gate`, and one dismissed proposal and the operator runs `task optimizer:proposals`
- **THEN** only the two `pending_review` proposals are listed with their id, strategy, regime, parameter, adjustment, rule, and status, and the command exits zero

#### Scenario: Gate-rejected proposals are visible only via an explicit filter

- **WHEN** the operator runs the list command with the status filter set to `rejected_by_gate`
- **THEN** the gate-rejected proposals are listed for audit
- **AND** they are absent from the default listing

#### Scenario: Inspecting a proposal shows its evidence and gate verdict

- **WHEN** the operator runs `task optimizer:proposal ID=<id>` for an existing proposal
- **THEN** the output includes the proposal's rationale, its evidence (weighted statistics, sample count, and pipeline gating summary), and its gate verdict's per-check name, observed value, threshold, and pass/fail

#### Scenario: Deciding a pending_review proposal records the decision once

- **WHEN** the operator decides a `pending_review` proposal `accepted`
- **THEN** the row's status becomes `accepted` with `decided_at` set and `decided_via` recording the channel
- **AND** a second decide attempt on the same proposal fails with an error and changes nothing

#### Scenario: A gate-rejected proposal cannot be decided

- **WHEN** the operator attempts to decide a `rejected_by_gate` proposal
- **THEN** the command fails with a clear error and the row is unchanged

#### Scenario: An empty proposal list is a clean outcome

- **WHEN** the operator runs `task optimizer:proposals` against an empty `strategy_proposals` table
- **THEN** the command reports that no proposals exist and exits zero

