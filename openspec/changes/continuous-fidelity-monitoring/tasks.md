# Tasks — continuous-fidelity-monitoring

## 1. FillDelta aggregation fix (review nit b)

- [x] 1.1 `src/go/tradingstack/fidelity/compare.go` — add `FillDeltaAbs` to `PairDelta` (mean of the absolute per-leg deltas, entry and exit); keep `FillDelta` as the mean of the signed leg deltas; update doc comments to state which form feeds which output.
- [x] 1.2 `score.go` — `Score` normalizes the mean of `FillDeltaAbs` (replacing mean `|FillDelta|`) for the composite; persisted/reported `DriftFill` stays the signed mean. Verify monotonicity and clamping guarantees still hold by construction.
- [x] 1.3 `synthetic.go` — add an offsetting-legs synthetic case (entry +0.50 / exit −0.50) with hand-computed expected magnitude 0.50 and signed delta 0.0.
- [x] 1.4 `fidelity_test.go` — tests: opposite-signed legs yield non-zero fill-drift magnitude and non-zero composite contribution; zero-drift pair still scores 0.0; existing scoring fixtures re-pinned where the fix changes them (each re-pin hand-computed, not snapshotted).

## 2. Persistence wiring and idempotency (review nit a)

- [x] 2.1 `src/go/tradingstack/fidelity/` — `MigrateFidelityMonitoring(db)` creating the unique index on `simulator_fidelity (strategy_id, period_start, period_end)`; idempotent; touches nothing else. Call it from the startup path that runs `MigrateTradingStack`. *(Note: no production startup path ran `MigrateTradingStack` before this change — `cmd/main.go`'s fidelity-monitor block now runs both, gated on `FIDELITY_MONITOR_ENABLED`.)*
- [x] 2.2 `checker.go` — convert `Persist` to an upsert on the unique key (`ON CONFLICT` update of `computed_at`, drift fields, `within_tolerance`).
- [x] 2.3 Testcontainers tests: round-trip of persisted rows (field-for-field), same-period re-persist updates not duplicates, distinct periods accumulate history, migration idempotency and no contact with playground or other trading-stack tables.

## 3. Monitor loop and trade source

- [x] 3.1 `src/go/tradingstack/fidelity/source.go` — `TradeSource` interface (`FetchTradeSet(ctx, period) (TradeSet, error)`), `NoLiveTradesSource`, and an in-memory fixture source; synthetic dataset exposed as a source for demo/tests.
- [x] 3.2 `src/go/tradingstack/fidelity/monitor.go` — `Monitor` with `Start(ctx)`: tick every `FIDELITY_CHECK_INTERVAL`, evaluate trailing `FIDELITY_PERIOD` ending at tick time via `RunFidelityCheck`, classify outcome `ok|breach|no_data|error`, persist results (task 2) on result-producing runs, Warn-log-and-continue on errors, injectable clock/ticker for tests. Strictly read-only against trading state.
- [x] 3.3 `src/go/tradingstack/fidelity/config.go` (or extend existing config) — env knobs `FIDELITY_MONITOR_ENABLED` (default true), `FIDELITY_CHECK_INTERVAL` (default 24h), `FIDELITY_PERIOD` (default 168h), following the telemetry `envDuration` pattern.
- [x] 3.4 Unit tests: tick evaluates the trailing window; `no_data` run persists nothing and changes no alert state; source error → outcome `error`, loop continues; disabled flag → no runs.

## 4. Telemetry integration (registry, heartbeat, alert rule)

- [x] 4.1 `src/go/telemetry/heartbeat_tracker.go` — add `SourceKindJob = "job"` to the valid source kinds; monitor beats `job/fidelity-monitor` on run completion (meta carries last outcome) plus a 60s idle keepalive.
- [x] 4.2 Monitor metrics via the internal registry: `grodt.fidelity.runs.total{status}` counter; `grodt.fidelity.drift_score{strategy_id}` and `grodt.fidelity.within_tolerance{strategy_id}` gauges updated on result-producing runs.
- [x] 4.3 `src/go/telemetry/alerts.go` — `RuleFidelityDrift = "fidelity_drift"`; `AlertEngine.ReportFidelity(...)` storing the latest per-strategy fidelity snapshot (replaced wholesale per result-producing run); `Evaluate` desires one `fidelity_drift|strategy/<id>` alert per breaching strategy, message carrying strategy id and drift score. Define the report payload as a small telemetry-owned struct so `telemetry` does not import `fidelity` (no import cycle; the monitor maps results into it).
- [x] 4.4 `alerts_test.go` — breach fires exactly one persisted+notified alert; recovery resolves it; ack silences re-notify while breach persists; `no_data` (no report) preserves last state; existing stale-heartbeat rule picks up a silent `job/fidelity-monitor` source.

## 5. Server wiring

- [x] 5.1 `cmd/main.go` — construct the monitor (NoLiveTradesSource for production wiring, `DefaultConfig()` scoring, AlertEngine report hook) and start it alongside the alert engine, gated by `FIDELITY_MONITOR_ENABLED`.
- [x] 5.2 Boot log line stating monitor state (enabled/disabled, interval, period, source type) so `no live trades` operation is self-explaining.

## 6. Operator surface

- [x] 6.1 `taskfile.yml` — `fidelity:status` target (psql, same shape as `telemetry:status`): latest verdict per strategy, recent history rows, `job/fidelity-monitor` heartbeat and latest run metrics; clean "no fidelity history" path; exits zero including when breaches are displayed.

## 7. Gates

- [ ] 7.1 `go build ./...` green.
- [ ] 7.2 `task test` green (backtester suite unaffected).
- [ ] 7.3 `task test:trading-stack` green (includes the new fidelity persistence/monitor tests; Docker required).
- [ ] 7.4 `go test -count=1 ./src/go/telemetry/... ./src/go/tradingstack/fidelity/...` green.

## 8. Operator-only follow-ups

- [ ] 8.1 Operator: run the dev server, confirm `task fidelity:status` and `task telemetry:status` show the monitor heartbeating with `no_data` runs; force a synthetic breach (test hook or fixture source) and verify the `fidelity_drift` alert lands in Slack and acks via both channels.
- [ ] 8.2 Operator: decide production enablement at next deploy (default is enabled with the no-live-trades source — heartbeat + no_data runs only until live-trade ingestion lands).
- [ ] 8.3 Flip this change's card(s) in `usm/roadmap.txt` to green `#C5E1A5` and refresh ROADMAP.md (done at archive time).
