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
- `halt_store_file.go` — file-backed JSON implementation. Default path is `<projectDir>/.safety/halt-state.json`, overridable via `KILL_SWITCH_STATE_PATH`. It lives **outside** `.cache/` deliberately: `.cache/` is wiped by convention (and by `task` cache-clearing targets), and a wiped halt file would silently boot a halted server back to *clear* — the single most dangerous failure for a kill switch. The store also exposes `Exists()` so startup can emit a loud `Warn` when the state file is missing/empty (i.e. the halt state is being bootstrapped from nothing, which a wipe would also produce).
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
2. **[DEFERRED — `wire-anomaly-guard-feeds`]** Order outcomes (rejected / filled / trade count) → guard observation sinks (`Observe` / `ObserveFill` / `ObserveTrade`) → guards evaluate on their rolling windows → on trip, `EngageAuto` + set cooldown-ack-required. The guards and registry exist and are unit-tested, but this live wiring (feeding real order outcomes into the sinks and constructing the registry at startup) has no production caller in this change and is deferred to `wire-anomaly-guard-feeds`.
3. **[DEFERRED — `wire-anomaly-guard-feeds`]** Staleness: `feed-health-staleness` last-Tick-age signal → `FeedStalenessGuard`; absent signal ⇒ guard inactive. The guard degrades to inactive when no signal source is wired; wiring the actual `feed-health-staleness` source is part of the same deferred follow-up.
4. **[DEFERRED — `wire-companion-stops`]** Live entry fill → companion stop constructed at configured distance → placed via Broker seam. `PlaceCompanionStop` is delivered as a tested library (exercised against MockBroker), but nothing in the live fill pipeline (`DrainTradierOrderQueue`) invokes it yet; that invocation is deferred to `wire-companion-stops` because wiring it overnight would place unverifiable real broker orders. When wired, the companion stop is risk-reducing and bypasses an engaged halt (see "Companion stops bypass the halt" below).
5. Operator: `GET /kill-switch/status` shows state; auto-halt requires `POST /acknowledge` then `POST /release`; manual halt releases directly. A manual `Engage` over an active auto-halt does NOT weaken it — it retains the ack requirement, automatic source, and auto reason (mirror of `EngageAuto` escalating over a manual halt).
6. Every state change is written through `HaltStateStore.Save`; on startup the controller is constructed from `HaltStateStore.Load`.

## Halt-seam design decisions

**The seam gates risk-increasing submission, not risk-reducing closes.** The halt
gate is consulted in `Playground.PlaceOrder` (via `CheckOrderGate`), the seam
that new order submission crosses. Forced maintenance-margin liquidations in
Simulation Mode (`performLiquidations`) commit position-closing orders directly
through the Simulated fill engine (`CommitOrderQueue`) and never call
`PlaceOrder`, so they are *not* gated by the halt. This is intentional: a kill
switch exists to stop the system from opening or adding to exposure, and blocking
the de-risking closes that keep a Simulation account above its maintenance margin
would strand it in a worse state. This exemption is documented as a scenario in
the `kill-switch` spec delta.

**Companion stops bypass the halt (when wired).** Protective companion stops are
risk-reducing: they attach a broker-held exit to an already-open position. When
the live wiring lands (`wire-companion-stops`), companion-stop placement SHALL
bypass an engaged halt — the kill switch must never strand an open position
without its protective exit. A halt stops new/adding exposure; it must not block
the stop that caps the loss on exposure the system already took on. Concretely,
the wired call SHALL place the companion stop through the Broker seam directly
rather than routing it through the halt-gated submission path. (In this change
`PlaceCompanionStop` is a library with no live caller, so no halt interaction
exists yet; this decision governs the deferred wiring.)

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
- **Deferred — live observation feeds (`wire-anomaly-guard-feeds`):** the guards and `NewGuardRegistry` ship as tested components but have no production caller. Wiring the live feeds is a named follow-up:
  - order outcomes (rejected/accepted) → `RejectionRateGuard.Observe`
  - fills (expected vs. actual price) → `FillDeviationGuard.ObserveFill`
  - executed trades → `TradesPerHourGuard.ObserveTrade`
  - the `feed-health-staleness` last-Tick-age signal source → `FeedStalenessGuard`
  - constructing the `GuardRegistry` at startup and evaluating it on a ticker
- **Deferred — companion-stop live invocation (`wire-companion-stops`):** `PlaceCompanionStop` ships as a tested library (MockBroker) but has no production caller. Wiring the live fill pipeline (`DrainTradierOrderQueue`) to invoke it on each real fill — and to bypass an engaged halt per the decision above — is a named follow-up (may be folded into `wire-anomaly-guard-feeds` if the feeds land together).
- **Lands overnight:** the full halt controller (including the manual-engage-must-not-weaken-an-auto-halt rule), all four guards, the cooldown state machine, persisted halt state, the REST endpoints + Taskfile targets, and companion-stop placement against MockBroker — all unit-tested deterministically.

## Verification gates

- **Unit tests — every trigger + cooldown path:** rejection-rate trip/no-trip, fill-deviation trip/no-trip, trades-per-hour 2σ trip/no-trip, feed-staleness trip / no-trip / absent-signal-inactive, cooldown release-blocked-until-ack, cooldown never-self-clears, persistence restart-stays-halted, and MockBroker companion-stop side/qty/price assertions.
- **G1:** `go build ./src/go/... ./cmd/...` green.
- **G2:** `task test` green.
- **G6:** mandatory Fable adversarial review — this is safety-critical code and gets G6 unconditionally per the overnight plan's gate policy.
