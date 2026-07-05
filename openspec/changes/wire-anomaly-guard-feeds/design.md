# Design — wire-anomaly-guard-feeds

## Context

`kill-switch-and-broker-side-stops` shipped the halt controller, the manual REST/Taskfile surface, the cooldown protocol, and four anomaly guards — but the guards have no production caller. `cmd/main.go` constructs the `HaltController` and installs it as the process-wide order gate; it never constructs a `GuardRegistry`. Likewise `feedhealth.HeartbeatMonitor` exists (with the `FeedHealthStalenessSignal` adapter ready in `src/go/backtester/safety/feed_staleness_adapter.go`) but nothing observes Ticks into it. This change closes that loop, and folds in three review nits from the same cycle (unauthenticated REST, non-durable halt-file write, trades-per-hour cold-start trip).

This is safety-critical wiring: a bug here either (a) fails to halt during a real anomaly, or (b) spuriously halts a healthy live session. Both directions are addressed explicitly below.

## Goals

- Guards observe real runtime signals and can trip the kill switch in anger, using exactly the delivered guard components and the shared `HaltController`.
- No spurious halts from structurally-predictable conditions: closed market, cold start, backtests.
- Mutating kill-switch endpoints require a shared secret; halt-state persistence survives power loss.
- Every guard trip is visible: Telemetry counters, a halt gauge, and a pushed Alert.

## Non-Goals

- No changes to guard math or the cooldown protocol beyond the trades-per-hour arming fix.
- No adaptive/per-regime threshold tuning (still static config, unchanged decision from the parent change).
- No companion-stop wiring (that is `wire-companion-stops`).
- No dashboard/UI; the REST + Taskfile + Telemetry surfaces are the operator interface.
- No Simulation-mode guard feeds (see Decision 2).

## Decisions

### 1. Registry constructed once in `cmd/main.go`, exposed through a package-level safety hook

`NewGuardRegistry` is called in `cmd/main.go` right after the `HaltController` is constructed, and the resulting registry is installed via a small package-level accessor in `safety` (mirroring the `models.SetOrderGate` pattern) so the order-queue and worker code can reach the guards without threading the registry through every constructor.

- *Alternative — thread the registry through `DatabaseService`/worker constructors*: rejected; it touches a dozen constructor signatures for what is deliberately a process-singleton, and the codebase already established the package-hook pattern for exactly this situation (`order_gate.go`), including the "nil hook = feature inert" property that keeps simulation/model-diff paths byte-for-byte unaffected.

### 2. Guards observe live (Paper/Margin) activity only, on wall-clock time

Observation hooks fire only on realtime-Mode paths: the live order-update pipeline (`DrainTradierOrderQueue` / `fillPendingOrder` in `src/go/backtester/services/order_queue.go`) and the live-repo candle ingestion (`TradierApiWorker.updateLiveRepos`). Simulation fills, backfills, and reconciliation adjustments never feed the guards.

- *Why*: a Simulation Playground replays days of history in minutes — its trades-per-hour rate on the guard's wall-clock window would be thousands, tripping the halt instantly and halting *live* trading because of a *backtest*. Rejection/deviation stats from the Simulated fill engine are similarly meaningless as broker-anomaly signals.
- *Alternative — feed all modes and tag observations by mode*: rejected; the guards are single-stream by design and the only mode whose anomalies the kill switch exists for is live execution.

### 3. Observation points

- **Rejection-rate**: `Observe(rejected=true)` where the live pipeline applies a broker rejection (`playground.RejectOrder` branch of `DrainTradierOrderQueue`); `Observe(rejected=false)` where a live fill commits (`fillPendingOrder` success). This gives the guard the real accepted/rejected mix at the Broker.
- **Fill-deviation**: in `fillPendingOrder`, `ObserveFill(expected=order.RequestedPrice, actual=trade price)` when a new live trade is created. Orders with no meaningful requested price (zero/absent) are skipped — the guard already refuses to evaluate `expected <= 0`.
- **Trades-per-hour**: `ObserveTrade()` at the same new-live-trade point.
- **Feed staleness**: `feedhealth.HeartbeatMonitor.Observe(assetClass, candleTimestamp… wall-clock now)` in `TradierApiWorker.updateLiveRepos` whenever new live bars are appended; asset class derived from the repo's instrument (equity vs option). The monitor feeds `FeedStalenessGuard` through the existing `FeedHealthStalenessSignal` adapter. The heartbeat records wall-clock receipt time, not bar timestamp, so delayed-but-arriving data still counts as a live feed (bar-lag detection stays the fidelity-checker's job).

### 4. Staleness evaluation on a ticker, gated by market hours and live-Playground presence

A goroutine in `cmd/main.go` calls `GuardRegistry.EvaluateFeedStaleness()` on a fixed interval (default 30s). Evaluation is suppressed when the market is closed (reusing the existing `TradierApiWorker.IsMarketOpen` calendar check) or when no realtime Playground is loaded.

- *Why*: without gating, every night at close the feed goes quiet, the guard trips, and the operator wakes to an engaged halt + required acknowledgment — a guaranteed false positive that would train the operator to ignore the kill switch.
- *Alternative — bake market-hours awareness into the guard*: rejected; the guard stays a pure threshold evaluator (unit-testable with fake clocks), and the *caller* owns "should we even be evaluating now", same split the parent change established.
- *Failure posture*: if the market-calendar check itself errors, evaluation is skipped for that tick with a `Warn` (fail-quiet for one interval, not fail-halt) — a calendar API blip must not halt trading; persistent calendar failure surfaces through the existing feed-health/alert paths.

### 5. Trades-per-hour guard arming (review nit c)

`TradesPerHourGuard` gains `MinSamples int`: the guard SHALL NOT trip until at least `MinSamples` trades sit in its rolling one-hour window. Additionally, degenerate history — supplied `histMean == 0 && histStdDev == 0`, i.e. "no σ history at all" — leaves the guard **unarmed** (it observes but never trips) and logs one `Warn` at construction. Default `MinSamples`: 5.

- *Why*: today `mean=0, σ=0 ⇒ threshold=0`, so the first live trade (`rate=1 > 0`) trips the halt. That is a cold-start artifact, not an anomaly.
- *Alternative — synthesize a default σ*: rejected; inventing a norm hides that the operator never supplied one. Unarmed + loud log is honest and matches the staleness guard's "absent signal ⇒ inactive" precedent.

### 6. Guard configuration via environment variables with conservative defaults

Thresholds come from env vars (parsed in `cmd/main.go`, `GuardConfig` populated once at startup, effective config logged at Info): `GUARD_REJECTION_WINDOW` (default 10m), `GUARD_REJECTION_THRESHOLD` (default 0.5), `GUARD_REJECTION_MIN_SAMPLES` (default 5), `GUARD_FILL_DEVIATION_PCT` (default 0.05), `GUARD_TRADES_PER_HOUR_MEAN` / `GUARD_TRADES_PER_HOUR_STDDEV` (default 0/0 ⇒ unarmed per Decision 5), `GUARD_TRADES_PER_HOUR_MIN_SAMPLES` (default 5), `GUARD_FEED_STALENESS_THRESHOLD` (default 5m), `GUARD_STALENESS_EVAL_INTERVAL` (default 30s). An explicit `off` value disables an individual guard (logged loudly at startup); unparseable values are a startup `Fatal` — a silently-misconfigured guard is worse than a refused boot.

- *Alternative — a section in `options-config.yaml`*: viable, but the kill-switch surface already established env-based config (`KILL_SWITCH_STATE_PATH`) and env keeps this change free of config-file schema churn. Revisit if guard config grows.

### 7. REST authentication (review nit a): shared secret, asymmetric fail posture

A middleware on the `/kill-switch` subrouter checks a shared secret from `KILL_SWITCH_TOKEN` (env), presented as `Authorization: Bearer <token>` (header `X-Kill-Switch-Token` also accepted for curl ergonomics). Rules:

- `POST /release` and `POST /acknowledge` — the halt-*weakening* operations — always require a valid token. If `KILL_SWITCH_TOKEN` is unset, they are refused (fail-closed) with a clear "token not configured" error.
- `POST /engage` requires the token when one is configured; when no token is configured it remains available, with a loud startup `Warn`. Rationale: stopping trading is the safe direction and must never be blocked by missing configuration; the worst an unauthenticated engage enables is a denial-of-trading, never an un-halt.
- `GET /status` stays unauthenticated (read-only, needed by health tooling).
- Taskfile `kill-switch:engage|release|acknowledge` targets pass `$KILL_SWITCH_TOKEN` from the operator's environment.
- *Alternative — full mTLS or reuse of an existing auth layer*: there is no existing auth layer on the REST router and mTLS is disproportionate for a Tailscale-internal single-operator deployment; a shared secret closes the reviewed gap at matching scale. Constant-time comparison (`crypto/subtle`) to avoid trivial timing leaks.

### 8. Crash-safe halt persistence (review nit b)

`FileHaltStore.Save` gains `File.Sync()` on the temp file before `Close`/`Rename`, and an fsync of the parent directory after the rename. This makes the atomic-rename write actually durable: without the fsyncs, a power loss just after `Save` returns can leave the *old* state (or on some filesystems an empty file) — for an engage that means a halted server silently rebooting clear. Directory-fsync failure is a returned error (the caller already treats persist failure as an error path).

### 9. Telemetry and Alerts (internal registry only — ADR-0005)

New instruments on `telemetry.Default`: counter `safety_guard_observations_total{guard}`, counter `safety_guard_trips_total{guard}`, gauge `safety_halt_engaged` (0/1, set on every controller transition and at startup restore). The AlertEngine gains a rule that pushes a Slack Alert when a guard trips / the halt engages automatically, so the operator hears about an auto-halt without polling status. No OpenTelemetry APIs, packages, or exporters are referenced anywhere in this change.

## Risks & Mitigations

- **Spurious auto-halt of a healthy live session** (mis-tuned defaults, market-closed staleness): conservative defaults; market-hours + live-presence gating; trades-per-hour unarmed without history; every trip Alerted and reversible via acknowledge+release; per-guard `off` switch for emergency de-tuning.
- **Missed anomaly (wiring gap)**: each observation hook gets a unit test that drives the real pipeline function (with MockBroker / synthetic events) and asserts the guard saw the observation and the controller engaged; plus the existing guard unit suites remain the math oracle.
- **Deadlock/latency in hot order path**: guard `Observe*` calls are mutex-guarded pure-memory operations (no I/O); hooks are placed outside DB transactions; `EngageAuto` persists a tiny JSON file — acceptable at trade frequency, and it is idempotent while already engaged (no persistence thrash under a rejection storm).
- **Locked-out operator (auth nit)**: fail-closed applies only to release/acknowledge; engage and status always reachable; token misconfiguration therefore can never prevent *stopping* the system, only resuming it — and resuming has a documented fix (set the token, restart is not required since env is read per-request... read once at startup; the error message states exactly which variable to set).
- **fsync portability/performance**: directory fsync is a no-op-with-error on some exotic filesystems; errors are surfaced, not swallowed. Save frequency is per-halt-transition (rare), so the added fsyncs are performance-irrelevant.

## Migration / Rollback

- **Migration**: none — no schema changes, no data migration. New env vars are optional (defaults apply); existing deployments gain armed guards on next restart. Operators must set `KILL_SWITCH_TOKEN` before they next need `release`/`acknowledge`; this is called out as an operator-only task and in the change's sign-off summary.
- **Rollback**: revert the change commits; the guards return to inert (registry never constructed, hooks compiled out), REST returns to unauthenticated, and the halt-state file remains fully compatible in both directions (same JSON shape). Setting every guard's env to `off` is the no-redeploy kill switch for the kill-switch-wiring itself.
