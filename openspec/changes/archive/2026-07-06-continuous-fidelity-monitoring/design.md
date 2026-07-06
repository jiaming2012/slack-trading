# Design — continuous-fidelity-monitoring

## Context

`fidelity-checker` (archived 2026-07-05) delivered a pure comparison engine (`src/go/tradingstack/fidelity/`): pair → compare → score → aggregate → gate, plus `task fidelity:check` with a synthetic dataset. Two things were explicitly deferred and two nits were recorded in review:

- No scheduler: nothing runs the check during live operation.
- `Persist(db, results)` exists (`checker.go`) but nothing calls it and no test covers it.
- `ComparePair` computes `FillDelta = (entryDelta + exitDelta) / 2` (`compare.go`); `Score` takes `|FillDelta|`, so a pair whose entry leg drifts +0.50 and exit leg drifts −0.50 contributes **zero** fill drift to the composite — real drift, invisible score.
- The gate's alert contract references "the existing logging/OTel path"; ADR-0005 has since removed OTel and delivered the internal telemetry module (`src/go/telemetry`): registry (Counter/Gauge → Postgres snapshots), heartbeat tracker, and an `AlertEngine` that evaluates rules over in-memory state, persists `telemetry_alerts` rows, posts to Slack, and re-notifies until acked.

The architecture doc's fidelity flow is weekly-batch ("Live trades (week) → re-run simulator → compare → drift score per strategy → within tolerance? proceed : pause + alert") and specifies the `simulator_fidelity` data model, which `trading-stack-schema` already implements. This change makes that flow continuous and observable; it does not change the comparison math except for the FillDelta fix.

## Goals / Non-Goals

**Goals**

1. Scheduled fidelity evaluation inside the trading server (no operator memory required, no new infra).
2. Fidelity scores persisted over time in `simulator_fidelity`, idempotent per period (wires + tests `Persist`).
3. Degradation alerts through the telemetry AlertEngine: persisted alert rows + Slack + ack + re-notify — the existing pattern, not a new mechanism.
4. Liveness of the monitor itself is observable (job heartbeat → existing stale-heartbeat rule).
5. Fix the FillDelta signed-leg cancellation.

**Non-Goals**

- Real live-trade ingestion and batch simulator re-run over a live period (later change; see D2).
- Automatically pausing optimizers (gate signal + alert only, unchanged contract).
- Any ESDB involvement (ADR-0004 undecided — Postgres only).
- Cron/Temporal/K8s scheduling infrastructure (arch doc lists Temporal for a later orchestration phase).
- Tuning weights, normalization constants, or the tolerance threshold against live data.
- Grafana/dashboard work (the observability stack is being decommissioned per ADR-0005; `task fidelity:status` and `task telemetry:status` are the surfaces).

## Decisions

### D1 — In-process scheduler goroutine, not external scheduling

The monitor is a goroutine started from `cmd/main.go` (same lifecycle pattern as `telemetry.AlertEngine.Start`): tick every `FIDELITY_CHECK_INTERVAL` (default 24h), evaluate the trailing `FIDELITY_PERIOD` (default 168h) ending at the tick time, guarded by `FIDELITY_MONITOR_ENABLED` (default true). Env-var knobs follow the telemetry `config.go` pattern (hardcoded defaults + env override, options-config YAML stays trading-only).

- *Alternative — cron on the droplet*: rejected — new infra, invisible to the server's telemetry, violates the no-new-infra constraint.
- *Alternative — Temporal workflow (arch doc's eventual orchestration)*: rejected for now — a whole new dependency for one ticker; revisit when `temporal-scan-orchestration` lands.
- *Interval/period defaults*: the arch doc compares "live trades (week)"; evaluating the trailing 7 days **daily** keeps the weekly comparison window but detects degradation up to 6 days earlier. Overridable.

### D2 — Pluggable trade source; production wires a no-live-trades source for now (FLAGGED for sign-off)

The monitor fetches its input through a `TradeSource` interface (`FetchTradeSet(ctx, period) (TradeSet, error)`), mirroring the `evtracker.TradeOutcomeRepository` precedent. This change ships: an in-memory/synthetic source (tests, demo) and a `NoLiveTradesSource` that production wiring uses, making every scheduled run a clean "no live trades" outcome until real input exists.

Why not a real source now: a real run needs (1) live-trade capture shaped as `fidelity.Trade` — entry/exit fills, realized PnL, hold duration, and an exit *reason*, which `trade_records` does not record — and (2) a batch simulator re-run over the same period, which does not exist. Building either overnight would be speculative and untestable against reality.

- *Alternative — derive live trades from `trade_records` directly*: rejected — no exit-reason field, no reliable entry/exit pairing per strategy, and no sim re-run to compare against; would produce authoritative-looking garbage.
- *Alternative — block this change until ingestion exists*: rejected — the scheduling/persistence/alerting/heartbeat plumbing is independently valuable and testable; swapping the source later is one constructor call at the `cmd/main.go` wiring point.
- Consequence (spelled out for the operator): after deploy, the monitor heartbeats and logs "no live trades" runs; **no fidelity rows or drift alerts occur until the live-trade source change lands.** The alert and persistence paths are proven by tests and by the synthetic path.

### D3 — Alerting: monitor reports into the AlertEngine; new `fidelity_drift` rule

The monitor calls a new `AlertEngine.ReportFidelity(results)` that stores the latest per-strategy fidelity snapshot in the engine's memory. `Evaluate` (the existing 30s cycle) adds one desired alert per strategy whose last-known result breaches tolerance, keyed `fidelity_drift|strategy/<id>`. Everything downstream is the existing lifecycle for free: persisted `telemetry_alerts` row, Slack delivery with the `ack <id>` instructions, re-notify until acked, resolution message when the condition clears (i.e., a later run reports the strategy within tolerance).

- *Alternative — drive the rule from registry gauges* (`within_tolerance` gauge == 0): rejected — the registry has no series-deletion, so a strategy that stops appearing in runs would pin a stale gauge forever with no way to distinguish "still breaching" from "no longer evaluated"; an explicit snapshot replaced per run has clear semantics.
- *Alternative — monitor posts to Slack directly*: rejected — bypasses persistence, ack, re-notify, and resolution; exactly the "new alerting mechanism" this change must not create.
- **No-data semantics**: a `no_data` run does *not* clear the snapshot — the last known state stands (a strategy known to be drifting stays alerted until contradicted by data, and the operator has acked it anyway). A run that produces results replaces the whole snapshot.
- Gauges are still recorded (`grodt.fidelity.drift_score{strategy_id}`, `grodt.fidelity.within_tolerance{strategy_id}`) for `task telemetry:status` visibility and snapshot history — they are observability, not the alert trigger.

### D4 — Persistence idempotency: unique index + upsert (wires review nit a)

`Persist` becomes an upsert keyed on `(strategy_id, period_start, period_end)` (GORM `ON CONFLICT` update of `computed_at`, drift fields, and `within_tolerance`). A new in-package additive migration `MigrateFidelityMonitoring(db)` creates the unique index — precedent: `MigrateNetEvCostModel` adding columns without touching `MigrateTradingStack` or any playground table. Without this, a restarted server re-evaluating the same trailing period would duplicate rows and double-count periods in the optimizer-validation fidelity gate.

- *Alternative — delete-then-insert per period*: rejected — not atomic, loses row identity.
- *Alternative — only ever insert, dedupe at read time*: rejected — pushes the problem into every consumer (`optvalidation.FidelityGate` reads these rows).
- Note: the unique-index columns are nullable in the schema; the monitor always populates them, and Postgres treating NULLs as distinct is acceptable for hand-inserted rows.
- Tested with testcontainers Postgres (round-trip + re-run-updates-not-duplicates), closing the "unwired/untested" nit.

### D5 — FillDelta fix (review nit b): per-leg absolute mean feeds the score; signed mean is kept for reporting

`PairDelta` gains `FillDeltaAbs = (|simEntry−liveEntry| + |simExit−liveExit|) / 2`; `Score` normalizes the mean of `FillDeltaAbs` (replacing mean `|FillDelta|`). `FillDelta` (mean of *signed* leg deltas) remains, and continues to feed the persisted `drift_fill` (the arch doc's "avg fill price delta", whose sign says whether sim fills flatter live).

- *Alternative — sum of absolute leg deltas*: rejected — doubles the scale, silently re-tuning `FillNormK`.
- *Alternative — score entry and exit legs as separate dimensions*: rejected — changes the composite's shape and weight vocabulary for no operator-visible benefit.
- Effect: scores where legs had offsetting drift can only increase (monotonicity preserved; a genuinely zero-drift pair still scores 0.0). Synthetic fixtures gain an offsetting-legs case.

### D6 — Monitor heartbeat: new source kind `job`

The monitor beats `telemetry.Heartbeats.Beat("job", "fidelity-monitor", …)` after every completed run (any outcome). `ValidSourceKind` accepts `job` so persistence/status surfaces treat it uniformly; the existing `stale_heartbeat` rule then covers a dead monitor with zero new alert code. Staleness threshold: the global `TELEMETRY_HEARTBEAT_STALE_AFTER` default (90s) is far shorter than the run interval, so the monitor also beats on a fast keepalive tick (every 60s) while idle — the run-completion beat carries `meta` (last run status/time); the keepalive proves the goroutine is alive.

- *Alternative — reuse kind `datasource` or `strategy`*: rejected — semantically wrong and pollutes the glossary's meaning of both (CONTEXT.md defines those kinds).
- *Alternative — per-source stale thresholds*: rejected as scope creep; the keepalive beat sidesteps it.

### D7 — Operator surface: `task fidelity:status` (psql, same shape as `telemetry:status`)

Read-only queries against the playground DB: latest `simulator_fidelity` row per strategy (score, verdict, period), last N history rows, and the `job/fidelity-monitor` heartbeat + latest fidelity run metrics. Exits zero even when breaches are shown (a breach is a reportable outcome, not a command failure — same contract as `fidelity:check`).

## Risks → Mitigations

- **Score shift from the FillDelta fix invalidates any prior intuition about scores** → no live data exists yet and no `simulator_fidelity` rows are in production; the change lands before any real history accrues. Synthetic fixtures pin the new expected values.
- **Alert noise once real data flows** (every run re-breaching) → the AlertEngine already dedupes by key and re-notifies on its own interval; ack silences; resolution only on a within-tolerance run.
- **Monitor failure loops (DB down, source error)** → run outcome `error` is counted and logged at Warn (never feeding `grodt.errors.total` into the error-rate rule from telemetry's own machinery — same rationale as the AlertEngine's Warn discipline), the loop continues, and the heartbeat keeps proving liveness; a *dead* loop is caught by stale-heartbeat.
- **Upsert races if a manual `fidelity:check --persist`-style run overlaps the monitor** → single-row atomic upserts keyed by period; last writer wins with identical inputs.
- **`replace-otel-with-internal-telemetry` is implemented but not yet archived** → this change builds on its shipped code (already on `dev`), not on its archive state; noted in tasks as an ordering precondition for validation only.

## Migration / Rollback

- **Migration**: `MigrateFidelityMonitoring(db)` — additive unique index on `simulator_fidelity (strategy_id, period_start, period_end)`; idempotent; called from the same startup path that runs `MigrateTradingStack`. No column changes, no playground-table contact, no data backfill (table is empty in every environment today).
- **Rollback**: set `FIDELITY_MONITOR_ENABLED=false` (kills the loop, everything else intact) or revert the commits; dropping the unique index is a single `DROP INDEX`. Alert rows and fidelity rows are plain data — no cleanup required. The FillDelta fix has no persisted-data migration implications (no rows exist).
