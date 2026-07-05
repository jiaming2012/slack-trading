## 1. Halt controller and persistence

- [ ] 1.1 Create package `src/go/backtester-api/safety/` with `HaltStateStore` interface (`Load`/`Save`) in `halt_store_interface.go` and a file-backed JSON implementation in `halt_store_file.go` (configurable path; no database dependency)
- [ ] 1.2 Implement `HaltController` in `halt_controller.go`: state (engaged/clear, reason, source manual|auto, cooldown-ack-required), mutex-guarded `Engage`, `EngageAuto`, `Acknowledge`, `Release`, `AllowOrder`, `Status`; construct-from-store on init and `Save` on every transition
- [ ] 1.3 Unit tests: restart-stays-halted, restart-stays-clear, missing/empty store defaults to clear (temp-dir store)

## 2. Order-submission halt gate

- [ ] 2.1 Consult `controller.AllowOrder()` in `src/go/backtester-api/router/grpc.go` `PlaceOrder` and in `src/go/backtester-api/services/order_queue.go` before an order reaches the Broker seam; return a distinct halt error when engaged
- [ ] 2.2 Unit tests: order rejected while engaged (all three Modes), order forwarded while clear — using MockBroker, asserting no forward-to-Broker occurs when halted

## 3. Anomaly guards

- [ ] 3.1 Implement `RejectionRateGuard` over a rolling window in `guards.go`; unit tests for trip (over threshold) and no-trip (at/under threshold)
- [ ] 3.2 Implement `FillDeviationGuard` (absolute deviation vs. expected price, percent threshold); unit tests for trip and no-trip
- [ ] 3.3 Implement `TradesPerHourGuard` (current rate vs. supplied historical mean + 2σ); unit tests for trip (>2σ) and no-trip (≤2σ)
- [ ] 3.4 Implement `FeedStalenessGuard` consuming the `feed-health-staleness` last-Tick-age signal; unit tests for stale-trips, fresh-no-trip, and absent-signal-degrades-to-inactive
- [ ] 3.5 Wire all guards to the shared `HaltController` via `guards_registry.go` (staleness signal source optional/nilable); unit test that any guard trip engages the same controller and blocks subsequent orders with source=auto

## 4. Cooldown protocol

- [ ] 4.1 Enforce cooldown in `HaltController`: auto-halt sets cooldown-ack-required; `Release` is rejected while ack outstanding; `Acknowledge` then `Release` clears
- [ ] 4.2 Unit tests: release-blocked-until-ack, acknowledge-then-release-resumes, never-self-clears-when-anomaly-subsides, status reports ack-required after auto-halt and not-required after manual halt

## 5. REST endpoints and Taskfile wrappers

- [ ] 5.1 Add `killswitchapi` handlers and register `POST /kill-switch/engage`, `POST /kill-switch/release`, `POST /kill-switch/acknowledge`, `GET /kill-switch/status` in `src/go/backtester-api/router/handler.go` `SetupHandler`
- [ ] 5.2 Add `kill-switch:engage`, `kill-switch:release`, `kill-switch:status` targets to `taskfile.yml` wrapping the REST endpoints
- [ ] 5.3 Unit/handler tests: engage sets engaged+manual, status reports state/reason/source/ack-required, release clears when no ack outstanding

## 6. Broker-side stop losses

- [ ] 6.1 Extend `src/go/backtester-api/models/mock_broker.go` to record received stop orders (side, quantity, stop price)
- [ ] 6.2 Implement companion-stop placement on live (Paper/Margin) entry fill: construct a `Stop` order at the configured distance on the protective side, sized to the filled quantity, placed via the Broker seam; skip in Simulation Mode
- [ ] 6.3 Add and validate positive stop-distance configuration; non-positive value errors instead of placing a stop at/through the fill price
- [ ] 6.4 Unit tests against MockBroker: long-entry stop below fill, short-entry stop above fill, Simulation places no stop, non-positive distance errors

## 7. Verification and closeout

- [ ] 7.1 Unit tests green for every trigger (rejection-rate, fill-deviation, trades-per-hour, feed-staleness incl. absent-signal) and the full cooldown path, plus persistence and MockBroker companion-stop assertions
- [ ] 7.2 G1: `go build ./src/go/... ./cmd/...` green
- [ ] 7.3 G2: `task test` green
- [ ] 7.4 G6: mandatory Fable adversarial review of the full diff (safety-critical) — explicit approval required before commit
- [ ] 7.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time)
