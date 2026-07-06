# fidelity-checker — delta for continuous-fidelity-monitoring

## MODIFIED Requirements

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
