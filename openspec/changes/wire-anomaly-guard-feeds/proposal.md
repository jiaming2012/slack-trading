# Proposal — wire-anomaly-guard-feeds

## Why

The anomaly halt guards (`kill-switch-and-broker-side-stops`, archived 2026-07-05) were delivered as tested components with their live observation feeds deliberately unwired: the guards, the `GuardRegistry`, and the `FeedHealthStalenessSignal` adapter exist and pass unit tests, but nothing in production constructs them or feeds them real order outcomes, fills, trades, or Tick ages. Until this change lands, no anomaly can trip the kill switch — the automatic half of the safety story is inert. The same review cycle flagged three non-blocking nits in the already-shipped kill-switch surface that belong in this wiring pass: the `/kill-switch/*` REST endpoints are unauthenticated, the halt-state file write is not crash-durable (no fsync before rename), and the trades-per-hour guard trips on the very first trade when it has zero σ history.

## What Changes

- Construct the `GuardRegistry` at server startup in `cmd/main.go`, bound to the existing shared `HaltController`, with guard thresholds sourced from environment configuration (documented safe defaults; per-guard disable).
- Feed the guards from real runtime signals, live (Paper/Margin) paths only:
  - broker order rejections and fills → `RejectionRateGuard.Observe` (via the live order-update pipeline in `src/go/backtester/services/order_queue.go`)
  - each live fill's actual price vs. the order's requested price → `FillDeviationGuard.ObserveFill`
  - each new live trade → `TradesPerHourGuard.ObserveTrade`
  - construct a `feedhealth.HeartbeatMonitor`, observe Ticks where live candles are ingested (`TradierApiWorker.updateLiveRepos`), and wire it into the `FeedStalenessGuard` through the existing adapter; evaluate the staleness guard on a ticker, suppressed outside market hours and when no live Playground is running (so a closed market can never auto-halt the server).
- Review nit (a): add a shared-secret token guard to the mutating `/kill-switch/*` REST endpoints (engage/release/acknowledge), with fail-closed behavior for release/acknowledge when no token is configured; the Taskfile wrappers pass the token.
- Review nit (b): make halt-state persistence crash-safe — fsync the temp file before the rename to `.safety/halt-state.json` and fsync the parent directory after it.
- Review nit (c): the trades-per-hour guard requires a configurable minimum trade sample before arming and treats degenerate history (mean and σ both zero) as unarmed, so it can no longer trip on the first observed trade.
- Record guard activity in Telemetry (internal registry — counters for observations and trips, a gauge for halt engaged) and push an Alert to the operator when any guard trips.

## Capabilities

### New Capabilities

None — this change wires and hardens existing capabilities; no new capability spec is introduced.

### Modified Capabilities

- `anomaly-halt-guards`: every requirement currently ends with "feeding … is deferred to `wire-anomaly-guard-feeds`" — those deferrals are discharged: guards are constructed at startup and fed from the live order/fill/trade/Tick paths. The trades-per-hour guard gains a minimum-sample arming requirement (nit c). New requirements cover market-hours gating of the staleness evaluation and Telemetry/Alert observability of guard activity.
- `kill-switch`: the REST requirement gains shared-secret authentication on mutating endpoints (nit a); the persistence requirement gains crash-durability (fsync before rename, nit b); the Taskfile wrapper requirement gains token passing.

## Impact

- **Code**: `cmd/main.go` (registry + monitor construction, staleness ticker), `src/go/backtester/safety/` (trades-per-hour arming, guard config from env), `src/go/backtester/services/order_queue.go` (rejection/fill/trade observation hooks), `src/go/workers/tradier_api_worker.go` (Tick heartbeat observation), `src/go/api/killswitchapi/` (auth middleware), `src/go/backtester/safety/halt_store_file.go` (fsync), `taskfile.yml` (token passing on existing `kill-switch:*` targets).
- **Config**: new env vars — `KILL_SWITCH_TOKEN` plus `GUARD_*` threshold overrides with safe defaults. No new dependencies, no schema changes, no new infrastructure.
- **Telemetry**: new counters/gauge in the internal telemetry registry (`telemetry.Default`) and an AlertEngine rule; per ADR-0005 no OpenTelemetry is referenced or reintroduced.
- **Behavioral risk**: once wired, a real anomaly (or a mis-tuned threshold) halts live order submission until an operator acknowledges and releases. Defaults are chosen conservative and every trip is Alerted, logged, and reversible via the documented cooldown protocol.
- **Autonomous-run boundaries respected**: all verification is unit/simulation/MockBroker-based; no live or Paper broker orders, no production database, no new infra. Live-fire drills are explicit operator-only tasks.
