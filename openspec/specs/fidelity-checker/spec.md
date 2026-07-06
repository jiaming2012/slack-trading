# fidelity-checker Specification

## Purpose
TBD - created by archiving change fidelity-checker. Update Purpose after archive.
## Requirements
### Requirement: Deterministic trade pairing over a period

The system SHALL provide a deterministic pairing function that, given a set of live trades and a set of simulator trades for the same period, matches each live trade to at most one simulator trade by strategy identifier and a stable pairing key (symbol plus entry timestamp). Pairing SHALL be deterministic: the same inputs SHALL always yield the same set of matched pairs and the same set of unmatched trades, independent of input ordering. A live trade with no corresponding simulator trade (or vice versa) SHALL be reported as unmatched and SHALL NOT contribute to any drift computation.

#### Scenario: Matched pairs are formed deterministically regardless of input order

- **WHEN** the pairing function is given a set of live trades and simulator trades that share strategy id, symbol, and entry timestamp
- **THEN** each live trade is paired with exactly its corresponding simulator trade
- **AND** re-running the pairing with the inputs in a different order produces the identical set of pairs

#### Scenario: Unmatched trades are reported and excluded from drift

- **WHEN** a live trade has no simulator trade with the same strategy id, symbol, and entry timestamp
- **THEN** that live trade is returned in the unmatched set
- **AND** it does not contribute to `drift_pnl`, `drift_fill`, or `drift_score`

### Requirement: Per-pair comparison on the four drift dimensions

The system SHALL compute, for each matched sim-vs-live pair, the per-pair deltas on the four dimensions named in the architecture doc: PnL delta (simulator PnL minus live PnL), fill-price deltas, hold-time delta (simulator hold duration minus live hold duration), and an exit-reason match flag (true when the simulator and live exit reasons are equal). Fill drift SHALL be carried in two forms: a signed fill delta (the mean of the signed per-leg deltas, entry and exit, each simulator minus live) used for reporting and for the persisted `drift_fill`, and a fill-drift magnitude equal to the mean of the *absolute* per-leg deltas, so that opposite-signed entry and exit leg drifts SHALL NOT cancel. The composite drift score SHALL consume the fill-drift magnitude, never the signed fill delta. All per-pair deltas SHALL be pure functions of the pair with no dependence on wall-clock time, randomness, database state, or external services.

#### Scenario: Zero-drift pair yields zero deltas and matching exit reason

- **WHEN** a pair whose simulator and live trades have identical PnL, fill prices, hold time, and exit reason is compared
- **THEN** the PnL delta, signed fill delta, fill-drift magnitude, and hold-time delta are all zero
- **AND** the exit-reason match flag is true

#### Scenario: PnL delta equals simulator PnL minus live PnL

- **WHEN** a pair whose simulator PnL exceeds its live PnL by a known amount is compared
- **THEN** the reported PnL delta equals that known amount (simulator minus live)

#### Scenario: Exit-reason mismatch is flagged

- **WHEN** a pair whose simulator exit reason is `target` and live exit reason is `stop` is compared
- **THEN** the exit-reason match flag is false

#### Scenario: Opposite-signed leg drifts do not cancel

- **WHEN** a pair whose simulator entry fill is 0.50 above the live entry fill and whose simulator exit fill is 0.50 below the live exit fill is compared
- **THEN** the fill-drift magnitude is 0.50 (the mean of the absolute per-leg deltas)
- **AND** the signed fill delta is 0.0 (the mean of the signed per-leg deltas)
- **AND** the pair contributes non-zero fill drift to the composite drift score

### Requirement: Composite drift score bounded to [0.0, 1.0]

The system SHALL reduce the per-pair deltas for a strategy into a composite `drift_score` in the closed interval `[0.0, 1.0]`, where `0.0` means the simulator perfectly mirrors live execution and larger values mean greater drift. The composite SHALL be a deterministic weighted combination of the normalized PnL drift, fill drift, and exit-reason mismatch rate, using fixed configurable weights. The score SHALL be monotonic non-decreasing in the magnitude of each underlying drift dimension and SHALL be clamped so that it never falls below `0.0` nor exceeds `1.0`, even for arbitrarily large input drift.

#### Scenario: Perfect-fidelity set scores zero

- **WHEN** every pair for a strategy has zero PnL delta, zero fill delta, and matching exit reasons
- **THEN** the composite `drift_score` for that strategy is `0.0`

#### Scenario: Score is clamped to at most 1.0 under extreme drift

- **WHEN** the composite is computed for a strategy whose pairs exhibit arbitrarily large PnL and fill deltas and fully mismatched exit reasons
- **THEN** the composite `drift_score` equals `1.0` and never exceeds it

#### Scenario: Score increases monotonically with drift

- **WHEN** two strategies are scored where the second has strictly larger drift on every dimension than the first
- **THEN** the second strategy's `drift_score` is greater than or equal to the first strategy's `drift_score`

### Requirement: Per-strategy fidelity record with tolerance verdict

The system SHALL aggregate matched pairs by strategy identifier and produce one fidelity result per strategy containing `strategy_id`, `period_start`, `period_end`, `drift_pnl` (simulator PnL minus live PnL, aggregated), `drift_fill` (average fill-price delta), `drift_score` (the composite), and `within_tolerance`. `within_tolerance` SHALL be true when and only when `drift_score` is less than or equal to a configurable tolerance threshold (default provided). Each result SHALL map field-for-field onto a `tradingstack.SimulatorFidelity` record so it can be persisted through the schema delivered by `trading-stack-schema`.

#### Scenario: Distinct strategies produce distinct fidelity records

- **WHEN** the checker runs over pairs spanning two strategy ids
- **THEN** exactly two fidelity results are produced, one per strategy id, each carrying that strategy's own `drift_pnl`, `drift_fill`, and `drift_score`

#### Scenario: within_tolerance reflects the threshold

- **WHEN** a strategy's `drift_score` is less than or equal to the configured tolerance threshold
- **THEN** its `within_tolerance` is true
- **AND** when another strategy's `drift_score` exceeds the threshold its `within_tolerance` is false

#### Scenario: Result maps onto the SimulatorFidelity model

- **WHEN** a fidelity result is converted to a `tradingstack.SimulatorFidelity`
- **THEN** the model's `strategy_id`, `period_start`, `period_end`, `drift_pnl`, `drift_fill`, `drift_score`, and `within_tolerance` fields equal the corresponding result values

### Requirement: Fidelity gate signal and alert on breach

The system SHALL derive, per strategy, a gate signal from `within_tolerance`: when `within_tolerance` is true the gate decision SHALL be Proceed (optimizers may consume that strategy's simulator data); when false the gate decision SHALL be Pause (optimizers must not consume that strategy's simulator data) and the system SHALL raise an alert through an injected alerter. The default alerter SHALL be backed by the platform's internal telemetry/logging path (structured logrus, counted by the telemetry error hook per ADR-0005); scheduled operation delivers operator-facing breach alerts through the telemetry AlertEngine per the `continuous-fidelity-monitoring` capability. The gate SHALL only emit the decision and raise the alert; it SHALL NOT itself modify or stop any optimizer. Alert raising SHALL be deterministically observable (an injected alerter is invoked exactly once per breaching strategy).

#### Scenario: Within-tolerance strategy proceeds without alert

- **WHEN** the gate evaluates a fidelity result whose `within_tolerance` is true
- **THEN** the gate decision is Proceed
- **AND** the injected alerter is not invoked for that strategy

#### Scenario: Tolerance breach pauses and raises exactly one alert

- **WHEN** the gate evaluates a fidelity result whose `within_tolerance` is false
- **THEN** the gate decision is Pause
- **AND** the injected alerter is invoked exactly once carrying that strategy's fidelity result

#### Scenario: Gate decisions are independent per strategy

- **WHEN** the gate evaluates a batch containing one within-tolerance strategy and one breaching strategy
- **THEN** the first yields Proceed and the second yields Pause, and exactly one alert is raised in total

### Requirement: Operator entry point and Taskfile target

The system SHALL provide an operator-visible command wired to a `task fidelity:check` Taskfile target that runs the fidelity checker for a period and prints the per-strategy drift results and gate decisions. The command SHALL exit non-zero only on an execution error (never merely because a strategy breached tolerance — a breach is a normal, reportable outcome). For overnight scope the command SHALL support running against a built-in synthetic sim-vs-live dataset so it is runnable with no live-trade input, and SHALL report a clear "no live trades available" status when invoked for a period with no ingested live trades.

#### Scenario: Task target runs the engine and reports drift

- **WHEN** an operator runs `task fidelity:check` against the built-in synthetic dataset
- **THEN** the command prints each strategy's `drift_score`, `within_tolerance`, and gate decision
- **AND** exits zero when the run completes without error, including when some strategies breach tolerance

#### Scenario: No live trades yields a clean no-input status

- **WHEN** the command is invoked for a period that has no ingested live trades
- **THEN** it reports a "no live trades available" status and exits zero without producing spurious fidelity records

