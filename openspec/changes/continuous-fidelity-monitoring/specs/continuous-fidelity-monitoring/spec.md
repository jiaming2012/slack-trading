# continuous-fidelity-monitoring — delta (new capability)

## ADDED Requirements

### Requirement: Pluggable trade source for scheduled runs

The system SHALL obtain each scheduled run's sim-vs-live trade sets through a `TradeSource` interface (`FetchTradeSet(ctx, period)` returning a `fidelity.TradeSet`), injected at wiring time, so an in-memory fixture source can drive the full monitor path in tests and a real live-trade-backed source can be supplied later without changing the monitor. The system SHALL ship a no-live-trades source (returns an empty live set) as the production wiring until live-trade ingestion exists, and the synthetic dataset source for tests and demos. A run whose source yields no live trades SHALL complete as a clean `no_data` outcome: no fidelity rows persisted, no fidelity alert state changed, no error raised.

#### Scenario: An in-memory source drives a scheduled run end to end

- **WHEN** the monitor runs one evaluation with an in-memory trade source returning a synthetic trade set with a known breaching strategy
- **THEN** the run completes without error and produces per-strategy fidelity results derived from exactly that source's trades

#### Scenario: No live trades yields a clean no-data run

- **WHEN** the monitor runs one evaluation with the no-live-trades source
- **THEN** the run completes with outcome `no_data`
- **AND** no `simulator_fidelity` row is written and no fidelity alert state is changed

### Requirement: Scheduled fidelity evaluation during live operation

The system SHALL run the fidelity checker on a repeating schedule inside the trading server: every `FIDELITY_CHECK_INTERVAL` (default 24h) it SHALL evaluate the trailing `FIDELITY_PERIOD` (default 168h) window ending at the evaluation time, using the existing `RunFidelityCheck` engine and the injected trade source. The monitor SHALL be enabled by default and disabled when `FIDELITY_MONITOR_ENABLED=false`. Each run SHALL classify its outcome as exactly one of `ok` (results produced, all within tolerance), `breach` (results produced, at least one strategy out of tolerance), `no_data`, or `error`. A run failure SHALL be logged at Warn severity and SHALL NOT stop the loop or crash the server. The monitor SHALL be read-only with respect to trading: it SHALL NOT place, modify, or cancel any order, and SHALL NOT mutate any playground state.

#### Scenario: Each tick evaluates the trailing period

- **WHEN** the monitor ticks at time T with a period of 168h
- **THEN** it invokes the fidelity engine over the window [T − 168h, T] with the trades the source returns for that window

#### Scenario: Disabled monitor never runs

- **WHEN** the server starts with `FIDELITY_MONITOR_ENABLED=false`
- **THEN** no scheduled fidelity evaluation runs and no fidelity monitor heartbeat is emitted

#### Scenario: A failing run does not stop the loop

- **WHEN** a run's trade source returns an error
- **THEN** that run completes with outcome `error`, logged at Warn severity
- **AND** the next scheduled tick still evaluates normally

### Requirement: Fidelity scores persisted over time with idempotent re-runs

The system SHALL persist every scheduled run that produces results by writing one `simulator_fidelity` row per strategy through the fidelity `Persist` path, so fidelity history accumulates over time in Postgres. `Persist` SHALL upsert keyed on `(strategy_id, period_start, period_end)`: re-persisting results for the same strategy and period SHALL update the existing row's `computed_at`, drift fields, and `within_tolerance` rather than inserting a duplicate. The system SHALL provide an idempotent additive migration `MigrateFidelityMonitoring` that creates the supporting unique index on `simulator_fidelity` without creating, altering, or dropping any other table or any playground table. The persistence path SHALL be covered by tests against a real PostgreSQL instance.

#### Scenario: Persisted rows round-trip against Postgres

- **WHEN** a run's results for two strategies are persisted against a testcontainers PostgreSQL
- **THEN** exactly two `simulator_fidelity` rows exist, and each row's `strategy_id`, `period_start`, `period_end`, `drift_pnl`, `drift_fill`, `drift_score`, and `within_tolerance` equal the corresponding result values

#### Scenario: Re-running the same period updates instead of duplicating

- **WHEN** results for strategy `S1` over period P are persisted, and then results for `S1` over the same period P with a different `drift_score` are persisted
- **THEN** exactly one `simulator_fidelity` row exists for `S1` and period P
- **AND** its `drift_score` equals the second persist's value

#### Scenario: History accumulates across distinct periods

- **WHEN** results for strategy `S1` are persisted for two non-identical periods
- **THEN** two `simulator_fidelity` rows exist for `S1`, one per period

#### Scenario: Migration touches only the fidelity index

- **WHEN** `MigrateFidelityMonitoring` runs twice against a database prepared by `MigrateTradingStack`
- **THEN** both calls return a nil error, the unique index on `simulator_fidelity (strategy_id, period_start, period_end)` exists, and no playground table or other trading-stack table is created, altered, or dropped

### Requirement: Telemetry instrumentation and monitor heartbeat

The system SHALL record each run through the internal telemetry registry (never OTel, per ADR-0005): a counter `grodt.fidelity.runs.total` labeled by run outcome, and per-strategy gauges `grodt.fidelity.drift_score` and `grodt.fidelity.within_tolerance` (1.0 when within tolerance, 0.0 when breaching) updated after each run that produces results. The monitor SHALL emit a heartbeat as a new source kind `job` with name `fidelity-monitor` — accepted by the telemetry heartbeat validation alongside `strategy` and `datasource` — beating on run completion (any outcome, carrying the last run's outcome in the heartbeat meta) and on a fast idle keepalive tick, so a dead monitor is caught by the existing stale-heartbeat alert rule with no new alerting mechanism.

#### Scenario: A completed run updates run counter and drift gauges

- **WHEN** a scheduled run completes with results for strategy `S1` with drift score 0.35 and `within_tolerance` false
- **THEN** the `grodt.fidelity.runs.total` counter for outcome `breach` increments
- **AND** the registry snapshot contains `grodt.fidelity.drift_score` = 0.35 and `grodt.fidelity.within_tolerance` = 0.0 labeled with strategy `S1`

#### Scenario: The job heartbeat kind is accepted and tracked

- **WHEN** the monitor beats source kind `job`, name `fidelity-monitor`
- **THEN** the heartbeat is accepted as a valid source kind and appears in the tracked heartbeat sources

#### Scenario: A silent monitor trips the existing stale-heartbeat rule

- **WHEN** the `job/fidelity-monitor` heartbeat stops beating for longer than the configured staleness threshold
- **THEN** the telemetry stale-heartbeat evaluation reports `job/fidelity-monitor` as stale, firing the existing stale-heartbeat alert path

### Requirement: Degradation alerts ride the telemetry AlertEngine

The system SHALL deliver fidelity degradation alerts through the existing telemetry AlertEngine lifecycle — persisted `telemetry_alerts` row, Slack notification, acknowledgement via the existing ack channels, and re-notification until acked — under a new rule name `fidelity_drift`, and SHALL NOT introduce any separate alerting mechanism. The monitor SHALL report each run's per-strategy results to the engine, replacing the engine's fidelity snapshot wholesale on every run that produces results and leaving it unchanged on `no_data` and `error` runs (last known state stands). The engine SHALL desire one firing `fidelity_drift` alert per strategy whose last-known result has `within_tolerance` false, carrying the strategy id and drift score, and SHALL resolve that alert when a later reported run shows the strategy within tolerance.

#### Scenario: A breaching strategy fires one persisted, notified alert

- **WHEN** the monitor reports results where strategy `S1` has `within_tolerance` false and the alert engine evaluates
- **THEN** exactly one `fidelity_drift` alert for subject `strategy/S1` fires, a `telemetry_alerts` row is persisted for it, and one operator notification is delivered

#### Scenario: A recovered strategy resolves the alert

- **WHEN** a `fidelity_drift` alert for `S1` is firing and the monitor reports a later run where `S1` is within tolerance
- **THEN** the next evaluation resolves the alert, stamping its resolution and delivering a resolution notification

#### Scenario: Acknowledgement silences re-notification while the breach persists

- **WHEN** a firing `fidelity_drift` alert is acknowledged and subsequent runs keep reporting the same strategy out of tolerance
- **THEN** the alert remains firing but delivers no further re-notifications

#### Scenario: No-data runs preserve the last known alert state

- **WHEN** a `fidelity_drift` alert for `S1` is firing and the next scheduled run completes as `no_data`
- **THEN** the alert remains firing (the snapshot is unchanged) rather than resolving on absence of data

### Requirement: Operator status surface and Taskfile target

The system SHALL provide a `task fidelity:status` Taskfile target that reads the playground database and prints: the latest `simulator_fidelity` verdict per strategy (drift score, within-tolerance, period), recent fidelity history rows, and the fidelity monitor's liveness (its `job/fidelity-monitor` heartbeat and latest run telemetry). The command SHALL exit zero when it completes, including when breaches are displayed (a breach is a reportable outcome, not a command failure), and SHALL print a clear "no fidelity history" message when the `simulator_fidelity` table has no rows.

#### Scenario: Status shows latest per-strategy verdicts and monitor liveness

- **WHEN** an operator runs `task fidelity:status` with fidelity history present, including a breaching strategy
- **THEN** the output shows each strategy's latest drift score, within-tolerance verdict, and period, plus recent history and the monitor heartbeat state, and the command exits zero

#### Scenario: Empty history reports cleanly

- **WHEN** an operator runs `task fidelity:status` against a database with no `simulator_fidelity` rows
- **THEN** the command prints a "no fidelity history" message and exits zero
