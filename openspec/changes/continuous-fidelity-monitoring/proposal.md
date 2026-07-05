# Continuous Fidelity Monitoring (scheduled simulator-vs-reality checking)

## Why

The fidelity checker shipped as a batch component: a pure comparison engine plus a `task fidelity:check` command the operator must remember to run. During live operation nothing evaluates simulator fidelity on a schedule, nothing accumulates fidelity history in the database (the checker's `Persist` function was declared deferred and is unwired and untested), and a fidelity degradation is only visible if the operator happens to run the command. The whole point of the fidelity gate — "optimizers pause when the simulator drifts" — needs the check to run continuously, its scores to persist over time, and a breach to reach the operator as an alert.

This change also folds in two review nits recorded in HANDOFF.md: (a) `fidelity.Persist` is unwired/untested — wire it into the scheduled path and test it against a real Postgres; (b) `FillDelta` averages the signed entry and exit leg deltas, so opposite-signed legs cancel and a pair with real fill drift can score as zero-drift — fix the aggregation.

## What Changes

- Add a **fidelity monitor**: an in-process scheduler inside the trading server that, on a configurable interval (default daily), runs the existing fidelity checker over a trailing period (default the last 7 days) and records the outcome. Read-only with respect to trading: the monitor never places orders or mutates any playground state.
- **Wire and test `Persist`** (review nit a): every scheduled run that produces results writes one `simulator_fidelity` row per strategy, so fidelity scores accumulate over time. Persistence becomes idempotent per `(strategy_id, period_start, period_end)` — a re-run of the same period updates the existing row instead of duplicating it — via a small additive migration (unique index) following the `MigrateNetEvCostModel` in-package precedent.
- **Fix the `FillDelta` aggregation** (review nit b): the per-pair fill-drift magnitude used by the composite score becomes the mean of the *absolute* per-leg deltas, so an entry leg drifting +0.50 and an exit leg drifting −0.50 no longer cancel to zero. The signed average is retained for the reported/persisted `drift_fill` (its sign is meaningful: whether the simulator fills better or worse than live).
- **Degradation alerts ride the internal telemetry AlertEngine** (ADR-0005 — no OTel, no new alerting mechanism): the monitor reports each run's per-strategy results to the alert engine, which gains a `fidelity_drift` rule. A tolerance breach produces a persisted `telemetry_alerts` row and a Slack notification with the existing ack + re-notify-until-acked lifecycle, and resolves when a later run comes back within tolerance.
- **Telemetry instrumentation** through the internal registry: per-strategy gauges (`grodt.fidelity.drift_score`, `grodt.fidelity.within_tolerance`), a run counter labeled by outcome, and a monitor heartbeat (new source kind `job`) so a silently dead monitor trips the existing stale-heartbeat alert.
- **Pluggable trade source**: the monitor reads its sim-vs-live trade sets through an injected interface. Until live-trade ingestion and batch sim re-run exist (a later change), production wires a no-live-trades source, so scheduled runs report a clean "no live trades" status — the scheduling, persistence, alerting, and heartbeat plumbing are all real and verified, and a real source later is a one-constructor swap. **Flagged for sign-off** — see design.md D2.
- New operator surface: `task fidelity:status` showing the latest per-strategy fidelity verdicts, recent history, and the monitor's last-run/heartbeat state.
- Update the fidelity-checker spec's stale reference to "the existing logging/OTel path" (OTel was removed by ADR-0005). Not BREAKING: the checker's public engine API is extended, not changed; the score of pairs with offsetting leg drift changes by design (that was the bug).

## Capabilities

### New Capabilities

- `continuous-fidelity-monitoring`: the scheduled evaluation loop, fidelity-score persistence over time (wired `Persist` + idempotent re-runs), telemetry instrumentation and job heartbeat, the `fidelity_drift` alert rule on the telemetry AlertEngine, and the `task fidelity:status` operator surface.

### Modified Capabilities

- `fidelity-checker`:
  - "Per-pair comparison on the four drift dimensions" — fill-drift aggregation must not let opposite-signed entry/exit legs cancel (magnitude = mean of absolute per-leg deltas; signed mean retained for reporting).
  - "Fidelity gate signal and alert on breach" — the alert path is the internal telemetry/logging path, not OTel (wording follows ADR-0005; behavior contract unchanged).

## Impact

- **Modified packages**: `src/go/tradingstack/fidelity/` (compare/score fix, `Persist` upsert + migration `MigrateFidelityMonitoring`, monitor loop, trade-source interface, config), `src/go/telemetry/` (new `fidelity_drift` rule + `ReportFidelity` input on `AlertEngine`, new heartbeat source kind `job`, new metric names), `cmd/main.go` (start the monitor), `taskfile.yml` (`fidelity:status`).
- **Database**: one additive unique index on the existing `simulator_fidelity` table; new rows accumulate there (subject to the existing trading-stack retention machinery). New alert rows use the existing `telemetry_alerts` table. Postgres only — no ESDB (ADR-0004 pending), no new infrastructure, no cron/Temporal.
- **Config**: new env knobs following the telemetry pattern — `FIDELITY_MONITOR_ENABLED` (default true), `FIDELITY_CHECK_INTERVAL` (default 24h), `FIDELITY_PERIOD` (default 168h).
- **Dependencies**: none new; rides `fidelity-checker`, `trading-stack-schema`, and the internal telemetry delivered by `replace-otel-with-internal-telemetry` (that change is implemented on `dev`; this change must build after it).
- **Downstream**: `optimizer-validation-pipeline`'s fidelity-gate stage starts receiving real `simulator_fidelity` history instead of an always-empty table; the `strategy-optimizer` change consumes that gating.
- **Not covered yet (explicit non-goals)**: ingesting real live trades and re-running the simulator over a live period (the monitor runs against the pluggable source's output; production reports "no live trades" until that later change); pausing optimizers automatically (the gate emits signals and alerts only); tuning drift weights/tolerance against live data.
