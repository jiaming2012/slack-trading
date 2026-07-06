# Tasks — wire-companion-stops

> HARD CONSTRAINT: no task below group 8 may place a live or Paper broker order,
> touch the production database, push, or create infrastructure. All verification
> binds the Broker seam to MockBroker or runs in Simulation Mode.

## 1. Orphan pending row on halt rejection (nit d)

- [x] 1.1 In `src/go/data/database_service.go` `commitOrderRecord`: when `playground.PlaceOrder` fails after the non-Simulation row was pre-created, finalize the row — status rejected, rejection reason recorded (halt reason when the gate rejected), persisted
- [x] 1.2 Unit tests: engaged halt + live-mode order ⇒ row exists with status rejected and halt reason, never pending; non-halt `PlaceOrder` failure also finalizes; Simulation path unchanged (no row pre-created)

## 2. Deferred option auto-closes during a halt (nit e)

- [ ] 2.1 In `src/go/backtester/models/playground.go` `postTickProcessing`: consult `CheckOrderGate()` before placing assignment/expiration auto-closes; while halted, retain each constructed close request + fill parameters on a playground deferred-auto-close list, log `Warn`, and complete the tick
- [ ] 2.2 Retry deferred auto-closes at the start of each subsequent tick; on success, commit with the retained fill parameters and clear the entry
- [ ] 2.3 Telemetry: `safety_deferred_auto_closes` gauge on `telemetry.Default` (internal registry only, per ADR-0005) plus an AlertEngine Alert while deferrals are outstanding
- [ ] 2.4 Unit tests: assignment during halt ⇒ tick completes, close deferred; expiration during halt ⇒ same; acknowledge+release ⇒ next tick commits deferred closes with original fill parameters; clear halt ⇒ auto-closes behave exactly as today; gauge/Alert emitted

## 3. Companion-stop configuration (opt-in)

- [ ] 3.1 Parse `COMPANION_STOP_DISTANCE` in `cmd/main.go`: unset ⇒ feature disabled with one loud startup `Warn`; non-positive ⇒ startup `Fatal`; positive ⇒ `CompanionStopConfig` constructed and installed for the live pipeline
- [ ] 3.2 Unit tests: unset disables, non-positive refuses startup (parse function level), positive enables

## 4. Eligibility and idempotency helpers

- [ ] 4.1 Implement fill eligibility in `safety` (or a thin `services` helper): realtime Mode, equity class, entry side (buy/sell_short), no `CloseOrderId`, not adjustment, not system auto-close, not reconciliation-role, tag != `companion-stop`
- [ ] 4.2 Implement per-entry idempotency: in-memory placed-set keyed by entry order ID plus a durable association (entry order ID attribute on the stop request; lookup against ingested companion-stop orders on restart)
- [ ] 4.3 Unit tests: each exclusion (close, adjustment, auto-close, reconciliation, companion-stop tag, options class, Simulation), and duplicate-event ⇒ single stop

## 5. Live-fill pipeline invocation with halt bypass

- [ ] 5.1 Invoke `PlaceCompanionStop` at the end of `fillPendingOrder` (`src/go/backtester/services/order_queue.go`) for eligible committed live fills, via the `IBroker` seam directly (below the halt-gated path); placement failure never fails the fill
- [ ] 5.2 Telemetry + Alert: `safety_companion_stops_placed_total` / `safety_companion_stop_failures_total` counters; Alert on failure naming the unprotected symbol/quantity
- [ ] 5.3 Unit/e2e tests with MockBroker: long entry ⇒ sell stop below fill; short entry ⇒ buy_to_cover stop above fill; **halt engaged ⇒ stop still placed while ordinary submission stays rejected**; companion-stop fill event fed back through the pipeline ⇒ no second stop; broker error ⇒ fill remains committed, failure counter + Alert
- [ ] 5.4 Regression: Simulation-Mode fills through the same pipeline place no stop and are byte-for-byte unchanged (existing suites stay green)

## 6. Documentation of the wiring

- [ ] 6.1 Update `openspec/specs`-adjacent operator docs surface only if one exists for kill-switch/stops; otherwise ensure `taskfile.yml` target descriptions and startup log lines are self-explanatory (no new operator-visible command is introduced by this change; if one emerges during implementation, add its matching `task` target in the same commit)

## 7. Verification gates (autonomous — MockBroker/Simulation only)

- [ ] 7.1 `go build ./src/go/... ./cmd/...` green
- [ ] 7.2 `task test` green (backtester suite)
- [ ] 7.3 Targeted suites green: `go test ./src/go/backtester/safety/... ./src/go/backtester/services/... ./src/go/data/... ./src/go/backtester/models/...`
- [ ] 7.4 Simulation smoke: run a local Simulation session (`task test:smoke` harness or equivalent) and confirm no companion stops, no deferred-close regressions, ticks green with and without an engaged halt
- [ ] 7.5 Grep-gate: no autonomous task started a Paper/Margin session; no Tradier sandbox credentials touched by tests added in this change

## 8. Operator-only follow-ups (NOT part of the autonomous run)

- [ ] 8.1 Set `COMPANION_STOP_DISTANCE` for the Tradier sandbox deployment and run one supervised Paper-Mode session: verify a real sandbox entry fill produces the companion stop at the broker with correct side/quantity/price
- [ ] 8.2 Supervised Paper-Mode halt drill: engage the kill switch, take an entry fill, confirm the companion stop is still placed while new submission is rejected; confirm the orphan-row fix (rejected rows, none pending) against the live database
- [ ] 8.3 Decide the Margin-Mode rollout (distance value, arming date) and perform the first supervised Margin verification
- [ ] 8.4 Review Alert wiring end-to-end in Slack: companion-stop failure Alert and deferred-auto-close Alert both arrive and identify the exposure
