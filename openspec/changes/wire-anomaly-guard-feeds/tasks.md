# Tasks — wire-anomaly-guard-feeds

## 1. Crash-safe halt persistence (nit b)

- [x] 1.1 In `src/go/backtester/safety/halt_store_file.go` `Save`: `Sync()` the temp file before `Close`, and fsync the parent directory after the `Rename`; surface flush failures as save errors
- [x] 1.2 Unit tests: save still round-trips (Load after Save), flush-failure path returns an error (e.g. directory removed between create and sync), existing atomic-write tests stay green

## 2. Kill-switch REST authentication (nit a)

- [x] 2.1 Add token middleware in `src/go/api/killswitchapi/`: read `KILL_SWITCH_TOKEN` at startup, accept `Authorization: Bearer` or `X-Kill-Switch-Token`, constant-time compare (`crypto/subtle`); release/acknowledge fail-closed when unset; engage open-with-Warn when unset; status always open
- [x] 2.2 Handler tests: valid token succeeds for all three mutating endpoints; missing/invalid token rejected with 401 and state unchanged; release/acknowledge refused with clear error when no token configured; engage succeeds when no token configured; status needs no token
- [x] 2.3 Update `taskfile.yml` `kill-switch:engage`, `kill-switch:release`, `kill-switch:acknowledge` targets to pass `$KILL_SWITCH_TOKEN`; update target `desc` strings to mention the token
- [x] 2.4 Log a loud startup `Warn` in `cmd/main.go` when `KILL_SWITCH_TOKEN` is unset (unauthenticated engage / locked release posture)

## 3. Trades-per-hour guard arming (nit c)

- [x] 3.1 Add `MinSamples` to `TradesPerHourGuard` (constructor param + `GuardConfig.TradesPerHourMinSamples`); guard cannot trip below the minimum window sample count
- [x] 3.2 Add unarmed state for degenerate history (`histMean == 0 && histStdDev == 0`): observes but never trips; one `Warn` at construction
- [x] 3.3 Unit tests: first-trade-with-zero-σ does not trip, below-min-samples does not trip even beyond 2σ, arming threshold reached then trip works, existing trip/no-trip tests updated for the new params

## 4. Guard configuration and startup construction

- [ ] 4.1 Add env-var parsing for `GuardConfig` (window/threshold/min-samples/deviation-pct/mean/stddev/staleness threshold/eval interval) with documented conservative defaults, per-guard `off` sentinel, and `Fatal` on unparseable values; log effective config at Info
- [ ] 4.2 Add a package-level registry hook in `safety` (mirroring `models.SetOrderGate`): `SetGuardRegistry`/accessor, nil-safe so all paths are inert when unwired
- [ ] 4.3 Construct the `GuardRegistry` in `cmd/main.go` immediately after the `HaltController`, bound to that same controller instance, and install it via the hook
- [ ] 4.4 Unit tests: env parsing (defaults, overrides, `off`, invalid ⇒ error), nil-hook inertness

## 5. Live observation feeds

- [ ] 5.1 Rejection outcomes: in `src/go/backtester/services/order_queue.go`, observe `rejected=true` where `DrainTradierOrderQueue` applies a broker rejection and `rejected=false` where `fillPendingOrder` commits a live fill; realtime Modes only
- [ ] 5.2 Fill deviation + trade rate: at the new-live-trade point in `fillPendingOrder`, call `FillDeviationGuard.ObserveFill(order.RequestedPrice, tradePrice)` (skip non-positive requested price) and `TradesPerHourGuard.ObserveTrade()`
- [ ] 5.3 Feed staleness: construct a `feedhealth.HeartbeatMonitor` at startup; call `Observe(assetClass, now)` from `TradierApiWorker.updateLiveRepos` when new bars are appended for a realtime Playground; wire it into `FeedStalenessGuard` via `FeedHealthStalenessSignal`
- [ ] 5.4 Staleness evaluation ticker in `cmd/main.go` (default 30s): call `GuardRegistry.EvaluateFeedStaleness()` only when the market is open (reuse `IsMarketOpen`) and at least one realtime Playground is loaded; calendar-check failure skips the cycle with `Warn`
- [ ] 5.5 Unit tests driving the real pipeline functions with synthetic events/MockBroker: a rejection event reaches the guard and can trip the shared controller; a fill feeds deviation + trade guards; Simulation paths feed nothing; ticker gating (closed market / no realtime playground / calendar error) never evaluates

## 6. Telemetry and Alerts (internal registry, per ADR-0005)

- [ ] 6.1 Add `safety_guard_observations_total{guard}` and `safety_guard_trips_total{guard}` counters and `safety_halt_engaged` gauge on `telemetry.Default`; set the gauge on every controller transition and on startup restore
- [ ] 6.2 Push an Alert (AlertEngine → Slack) when a guard trips / the halt engages automatically, naming the guard and reason
- [ ] 6.3 Unit tests: trip increments counters and sets gauge; release clears gauge; alert emitted on auto-engage

## 7. Verification gates (autonomous — no live/Paper orders, no prod DB, no push, no new infra)

- [ ] 7.1 `go build ./src/go/... ./cmd/...` green
- [ ] 7.2 `task test` green (backtester suite)
- [ ] 7.3 Targeted suites green: `go test ./src/go/backtester/safety/... ./src/go/api/killswitchapi/... ./src/go/backtester/services/... ./src/go/telemetry/... ./src/go/workers/...`
- [ ] 7.4 Simulation-only end-to-end check against a locally run dev server: engage/release/acknowledge via `task kill-switch:*` with a token set, confirm 401 without the token, confirm Simulation ticking is unaffected by guard wiring (no live playgrounds ⇒ no staleness evaluation)

## 8. Operator-only follow-ups (NOT part of the autonomous run)

- [ ] 8.1 Set `KILL_SWITCH_TOKEN` in the production/droplet environment (and the operator shell that runs `task kill-switch:*`); restart and verify release requires it
- [ ] 8.2 Review and tune guard thresholds (`GUARD_*` env vars) against real Paper-session statistics; supply a real trades-per-hour mean/σ so that guard arms
- [ ] 8.3 Live feed-outage drill during market hours in Paper Mode: confirm the staleness guard trips, the Alert arrives in Slack, and acknowledge+release restores submission
- [ ] 8.4 Observe one full Paper session with guards armed; confirm zero spurious trips (or de-tune and re-run)
