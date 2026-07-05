# Design — kill-switch-and-broker-side-stops

## Approach

One central `HaltController` owns a single source of truth for "may orders be submitted right now?". Both the manual kill switch and every anomaly guard converge on this one object, so there is exactly one chokepoint and one status surface. The order-submission path consults the controller immediately before an order crosses the Broker seam — the single seam through which every order in every Mode is placed — so a halt cannot be bypassed by any Mode or code path.

The controller is deliberately small and synchronous: engage / release / status plus an `AllowOrder()` check. State transitions are guarded by a mutex. Persistence is behind a `HaltStateStore` interface with a file-backed default (JSON at a configured path) so that (a) restarts stay halted and (b) unit tests use a temp dir with no database dependency, satisfying the overnight prohibition on prod DB connections.

Guards are pure evaluators over rolling windows. Each guard receives observations (order outcomes, fills, trade counts, or a staleness signal) and returns a trip decision; when it trips it calls `controller.EngageAuto(reason)`. Keeping guards free of I/O makes every trigger deterministically unit-testable with fake clocks and synthetic inputs.

Broker-side stops hook the point where a live entry order transitions to filled: a companion stop order is constructed at the configured distance on the protective side and placed through the same `IBroker.PlaceOrder` seam. Overnight this runs against `MockBroker`, which is extended to record received stop orders for assertions.

## File / package layout

New package `src/go/backtester-api/safety/`:

- `halt_controller.go` — `HaltController`: state (engaged/clear, reason, source manual|auto, cooldown-ack-required flag), `Engage(reason)`, `EngageAuto(reason)`, `Acknowledge()`, `Release()`, `AllowOrder()`, `Status()`.
- `halt_store_interface.go` — `HaltStateStore` interface (`Load()`, `Save(state)`).
- `halt_store_file.go` — file-backed JSON implementation; default path from config/env.
- `guards.go` — `RejectionRateGuard`, `FillDeviationGuard`, `TradesPerHourGuard`, `FeedStalenessGuard`; each with an `Evaluate(...)` returning a trip decision and, on trip, engaging the controller.
- `guards_registry.go` — wires guards to the controller and to their observation sources; the staleness guard's signal source is optional (nil ⇒ inactive).
- `*_test.go` — unit tests per guard trigger and the cooldown path (fake clocks, synthetic observations).

New REST surface (registered in `src/go/backtester-api/router/handler.go` `SetupHandler`, following the existing Gorilla Mux pattern; a small `killswitchapi` handler set):

- `POST /kill-switch/engage`, `POST /kill-switch/release`, `POST /kill-switch/acknowledge`, `GET /kill-switch/status`.

Order path integration (read-mostly edits, no signature changes):

- `src/go/backtester-api/router/grpc.go` `PlaceOrder` and `src/go/backtester-api/services/order_queue.go`: consult `controller.AllowOrder()` before forwarding to the Broker; on halt, return the distinct halt error. Feed order rejections / fills / counts into the guards.

Broker seam:

- `src/go/backtester-api/models/mock_broker.go`: record stop orders (extend the existing `requests`/`orders` capture) so tests can assert side/qty/stop price.
- Companion-stop construction logic lives in `safety/` (or a thin helper in `services/`) and is invoked on live entry fills; it uses the existing `Stop` order type from `backtester_order_type.go`.

Taskfile (`taskfile.yml`): `kill-switch:engage`, `kill-switch:release`, `kill-switch:status` targets that curl the REST endpoints.

## Data flow

1. Order request → `PlaceOrder` / order queue → `controller.AllowOrder()`.
   - Halted → reject with halt error, never reaches Broker.
   - Clear → forward to `IBroker.PlaceOrder`.
2. Order outcomes (rejected / filled / trade count) → guard observation sinks → guards `Evaluate` on their rolling windows → on trip, `EngageAuto` + set cooldown-ack-required.
3. Staleness: `feed-health-staleness` last-Tick-age signal → `FeedStalenessGuard`; absent signal ⇒ guard inactive.
4. Live entry fill → companion stop constructed at configured distance → placed via Broker seam (MockBroker overnight).
5. Operator: `GET /kill-switch/status` shows state; auto-halt requires `POST /acknowledge` then `POST /release`; manual halt releases directly.
6. Every state change is written through `HaltStateStore.Save`; on startup the controller is constructed from `HaltStateStore.Load`.

## Explicitly OUT of scope

- Placing any real order against a live or Paper broker (Tradier). All execution overnight is MockBroker / synthetic.
- Tradier-sandbox validation of broker-side stops (deferred — see below).
- A physical dashboard button / Grafana panel (the gap-analysis mentions a dashboard button; only the REST + Taskfile surface is in scope here).
- Per-regime automatic threshold tuning. Thresholds are static configuration this change; adaptive tuning is future work.
- Portfolio-level exposure/drawdown circuit breaking — that is `portfolio-risk-overlay` (Finding 1), a separate change.
- Modifying the `feed-health-staleness` capability itself; this change only consumes its signal.

## Dependency ordering on other batch changes

- Soft dependency on `feed-health-staleness` (Wave 3a) for the staleness signal. This change is scheduled in Wave 3b per `todo/overnight-run-plan-20260704.md`. Because the staleness guard degrades gracefully to inactive when no signal source is registered, this change builds, tests, and passes G1/G2 independently even if `feed-health-staleness` is not merged.
- No hard dependency on the Phase 2 refactors (`reconcile-models-packages`, `migrate-crossed-enums`); the halt gate and stops are additive to the order path and Broker seam and can rebase onto whichever tree is current.

## Deferred validation

This is a **Tier B** change: code and unit tests land overnight; final validation is deferred.

- **Deferred:** end-to-end verification of broker-side stops against the Tradier sandbox (requires a Paper/sandbox account and places broker orders — prohibited overnight). Also deferred: a live feed-outage drill exercising the staleness guard against a real feed.
- **Lands overnight:** the full halt controller, all four guards, the cooldown state machine, persisted halt state, the REST endpoints + Taskfile targets, and companion-stop placement against MockBroker — all unit-tested deterministically.

## Verification gates

- **Unit tests — every trigger + cooldown path:** rejection-rate trip/no-trip, fill-deviation trip/no-trip, trades-per-hour 2σ trip/no-trip, feed-staleness trip / no-trip / absent-signal-inactive, cooldown release-blocked-until-ack, cooldown never-self-clears, persistence restart-stays-halted, and MockBroker companion-stop side/qty/price assertions.
- **G1:** `go build ./src/go/... ./cmd/...` green.
- **G2:** `task test` green.
- **G6:** mandatory Fable adversarial review — this is safety-critical code and gets G6 unconditionally per the overnight plan's gate policy.
