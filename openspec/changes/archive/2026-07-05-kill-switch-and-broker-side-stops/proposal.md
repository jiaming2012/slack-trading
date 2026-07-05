## Why

Before any Playground ever trades with real money (Margin mode) or against the broker sandbox (Paper mode), the platform needs an emergency stop it cannot outrun and an exit that survives total infrastructure loss. Today there is no way to instantly halt all order submission, no automatic guard against runaway or anomalous trading, and no broker-held stop that protects open positions when our own process is dead — the two P0 safety gaps (gap-analysis Finding 2 and Finding 3 Tier 1).

## What Changes

- Add a **hard kill switch**: a REST endpoint that halts ALL order submission across every Mode (Simulation, Paper, Margin) instantly, plus a matching operator command. The halt is checked at the single Broker seam through which every order flows, so nothing can leak past it. The halt state is persisted so a server restart comes back up still halted.
- Add **anomaly auto-halt guards** evaluated over a rolling window that trip the same halt automatically when any of these fire: order-rejection rate over threshold, fill price deviating more than a configured percent from the expected price, trades-per-hour exceeding 2σ of the historical norm, or data-feed staleness (consuming the `feed-health-staleness` signal, degrading gracefully to "inactive" if that signal is not present).
- Add a **cooldown protocol**: after ANY auto-halt, the system stays halted and requires an explicit manual acknowledgment before order submission can resume — it never self-clears even if the anomaly subsides.
- Add **broker-side stop losses**: when a live (Paper/Margin) entry order fills, a companion broker-held stop order is placed at a configured stop distance so the exit executes at the broker even if our infrastructure is completely down. Implemented and unit-tested against the existing MockBroker; verification against the Tradier sandbox is deferred (out of scope tonight — no live/paper orders).
- Add operator-facing Taskfile targets (`task kill-switch:engage`, `task kill-switch:release`, `task kill-switch:status`) wrapping the REST endpoints, per the project convention that every operator-visible command ships with a Taskfile target.
- No existing successful order path or RPC signature is removed or changed shape — orders gain a new rejected-because-halted failure mode only. Nothing here is **BREAKING**.

## Capabilities

### New Capabilities

- `kill-switch`: A hard, instantly-effective halt of all order submission across every Mode, engaged manually via REST + Taskfile or automatically by the guards, with halt state persisted across restarts.
- `anomaly-halt-guards`: Rolling-window anomaly detectors (rejection rate, fill-price deviation, trades-per-hour sigma, feed staleness) that trip the kill switch automatically.
- `halt-cooldown-protocol`: The post-auto-halt cooldown state machine requiring explicit manual acknowledgment before order submission resumes.
- `broker-side-stop-losses`: Broker-held stop orders placed at entry for every open live position so exits survive total infrastructure failure.

### Modified Capabilities

(none — no existing specs exist in this repo yet, and no existing requirement's behavior changes)

## Impact

- **New Go package** `src/go/backtester-api/safety/`: halt controller, persisted halt-state store (interface + file-backed default), anomaly guard evaluators, cooldown state machine.
- **New REST handler** (a `killswitchapi` producer or equivalent registered in `src/go/backtester-api/router/handler.go` `SetupHandler`) exposing engage / release / status endpoints on the existing Gorilla Mux router (:8080).
- **Order-submission path** (`src/go/backtester-api/router/grpc.go` `PlaceOrder`, `src/go/backtester-api/services/order_queue.go`, and the Broker seam `IBroker.PlaceOrder`): consult the halt controller before any order reaches the Broker; feed order outcomes (rejections, fills, counts) into the guards.
- **Broker seam** (`src/go/backtester-api/models/broker_interface.go`, `mock_broker.go`, and `services/tradier_broker.go`): companion stop-order placement on live entry fills; MockBroker extended to record stop orders for unit tests.
- **New Taskfile targets** in `taskfile.yml`: `kill-switch:engage`, `kill-switch:release`, `kill-switch:status`.
- **Dependencies on other changes in this batch**: soft dependency on `feed-health-staleness` for the staleness guard's tick-age signal — the staleness guard degrades gracefully to inactive if that change has not landed, so this change builds and passes its gates independently (Wave 3b per `todo/overnight-run-plan-20260704.md`).
- **Verification gates**: unit tests for every guard trigger and the cooldown path (deterministic, fake clocks, synthetic order/fill data, MockBroker); G1 (`go build ./src/go/... ./cmd/...`); G2 (`task test`); G6 (mandatory Fable adversarial review — safety-critical). Tradier-sandbox validation of broker-side stops is explicitly deferred (not overnight).
