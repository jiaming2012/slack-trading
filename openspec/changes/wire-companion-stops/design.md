# Design — wire-companion-stops

## Context

`PlaceCompanionStop` (`src/go/backtester/safety/companion_stop.go`) is a tested library: given a live entry fill it constructs a protective `Stop` order on the correct side at the configured distance and places it through the `IBroker` seam, returning `(nil, nil)` in Simulation. It has no production caller — the parent change's design deferred the invocation to this change because overnight verification would have placed real broker orders. The live fill pipeline it must hook is `DrainTradierOrderQueue` → `commitPendingOrders` → `fillPendingOrder` in `src/go/backtester/services/order_queue.go`, the point where a Tradier order-update event becomes a committed live fill.

Two review nits from the same cycle are halt-interaction bugs in already-shipped code and are folded in here because both live on the same order-placement path this change touches:

- **(d) Orphan pending row**: `DatabaseService.commitOrderRecord` (`src/go/data/database_service.go`) pre-creates the order row for non-Simulation modes (`s.db.Create(&order)`) *before* calling `playground.PlaceOrder`, where `CheckOrderGate` rejects during a halt — leaving a pending row that no pipeline will ever finalize.
- **(e) Auto-closes error the tick**: `Playground.postTickProcessing` (`src/go/backtester/models/playground.go`) places option-assignment/expiration auto-close orders via `dbService.PlaceOrders`, which crosses the halt-gated `Playground.PlaceOrder`; while halted, the returned halt error propagates and the whole tick fails.

A prior decision from the parent change's design is binding here: **when wired, protective companion stops BYPASS an engaged halt** — a halt stops new/adding exposure; it must never strand an open position without the stop that caps its loss.

**Hard constraint**: the autonomous run may NOT place live or Paper broker orders. All verification is MockBroker/Simulation; live verification is operator-only.

## Goals

- Every eligible live (Paper/Margin) equity entry fill gets exactly one broker-held companion stop, placed even while the halt is engaged.
- A placement failure is loud: log, Telemetry, Alert — never a silent unprotected position.
- Halt rejections leave clean state: no orphan pending rows, no failed ticks.
- Everything autonomous is verifiable against MockBroker and Simulation.

## Non-Goals

- No option-position companion stops (the library supports equity buy/sell_short only); named as future work.
- No stop management after placement — no trailing, no adjustment on partial exits, no cancel-on-close reconciliation beyond what the existing external-order ingestion already does. v1 places the protective exit; lifecycle management is a follow-up change.
- No Tradier-sandbox or Margin verification inside the autonomous run (operator-only, final task group).
- No change to the halt gate's placement or the cooldown protocol.

## Decisions

### 1. Invocation point: `fillPendingOrder`, after the fill commits and saves

The companion stop is placed at the end of `fillPendingOrder`, once the live fill has committed (`newTrade != nil`) and the order record has been re-saved. That is the earliest moment the fill price and quantity are authoritative, and it runs for every live fill regardless of which drain branch produced it.

- *Alternative — hook `DrainTradierOrderQueue`'s event loop directly*: rejected; the event carries broker-side state, but `fillPendingOrder` is where our books accept the fill — hooking there guarantees we never place a stop for a fill our own pipeline refused.
- Placement failure does NOT fail the fill: the fill already happened at the broker; failing our bookkeeping would desync us from reality. Instead the failure path is Warn + Telemetry + Alert (Decision 6).

### 2. Eligibility: opening equity entry fills only, with tag-based self-exclusion

A fill is companion-stop-eligible when all hold: realtime Mode (Paper/Margin — `PlaceCompanionStop` already hard-rejects others); equity class; the order is an entry (side buy or sell_short, no `CloseOrderId`, not `IsAdjustment`, not a system auto-close); and the order's tag is not `companion-stop` (the stop must never spawn a stop — unbounded recursion through the order-update stream otherwise). Reconciliation-role playground fills are excluded: companion stops attach to the *logical* entry, and reconciliation adjustments are netting artifacts, not exposure decisions.

### 3. Idempotency by cumulative coverage: total stop quantity tracks total filled quantity (revised after adversarial review)

One strategy order routinely nets into MULTIPLE broker trades (buy 10 against an account short 4 → buy_to_cover 4 + buy 6), and `fillPendingOrder` commits one fill per trade. A boolean "placed once per entry order" therefore under-protects: the first trade gets a stop and every further trade is misread as a redelivery (the review's blocker finding). The stopper instead tracks the CUMULATIVE protected quantity per entry order and sizes every placement to the still-unprotected delta against the entry's total FILLED quantity — never the requested total, which would over-size the stop and reverse instead of flatten. A redelivered fill event carries no new filled quantity (delta 0) and places nothing; a top-up failure is recorded and alerted for the missing DELTA while existing coverage stays tracked. A fractional remainder below one whole share is skipped with a Warn (not the failure alert) and stays outstanding, so accumulation across fills still reaches whole-share protection.

- *Durable association*: the stop's tag (`companion-stop-<entryOrderID>`). The reserved prefix is rejected for client-supplied tags at the PlaceOrder RPC boundary (4xx), so it cannot be poisoned from outside.
- *Restart window*: the in-memory tracker dies with the process; a fresh process seeds the baseline by scanning broker orders requiring the FULL match — the entry's tag AND stop order type AND the entry's symbol — and summing their quantities (a tag-only scan would let a decoy suppress a real stop). A failed broker lookup counts as nothing-protected; the residual duplicate-stop risk is accepted because a duplicate *protective* stop fails safe: it flattens, never reverses beyond the tag-check bugs we test for.

### 4. Halt bypass by construction, not by exception

`PlaceCompanionStop` places through `IBroker.PlaceOrder` directly — the Broker seam *below* the halt gate (`CheckOrderGate` lives in `Playground.PlaceOrder`, which this path never crosses). The bypass therefore needs no halt-controller special case; the requirement is enforced by routing and locked in by a test that engages the halt, commits a live entry fill against MockBroker, and asserts the stop was still placed.

- *Alternative — add an exempt-orders list to the halt controller*: rejected; exemption lists on the gate itself are exactly the kind of soft spot an adversarial review flags. Routing risk-reducing orders below the gate matches the established precedent (forced Simulation liquidations commit via `CommitOrderQueue` for the same reason).

### 5. Opt-in configuration: no distance, no feature

`COMPANION_STOP_DISTANCE` (env, absolute price distance) enables the feature; unset ⇒ disabled with one loud startup `Warn`; set but non-positive ⇒ startup `Fatal` (the library's positive-only validation, enforced early instead of erroring on every fill).

- *Why opt-in*: a shipped default distance would be wrong for someone's instrument and silently place mispriced stops on real accounts. Off-with-warning makes arming it a deliberate operator act, which also cleanly separates "code wired" (this change, autonomous) from "feature live" (operator-only).

### 6. Placement failure raises an Alert

On `PlaceCompanionStop` error: `log.Errorf` with entry order context, increment `safety_companion_stop_failures_total`, push an AlertEngine Alert ("position UNPROTECTED: companion stop failed for <symbol> qty <n>"). Success increments `safety_companion_stops_placed_total`. Internal telemetry registry only (ADR-0005); no OTel.

### 7. Nit (d): finalize the pre-created row as rejected, don't delete it

In `commitOrderRecord`, when `playground.PlaceOrder` fails after the non-Simulation DB row was created, the row is finalized: status set to rejected with the rejection reason recorded (halt or otherwise) and saved. Detection does not need the `safety` package (import cycle: `safety` → `models`); the cleanup applies to *any* `PlaceOrder` failure after row creation, halt included.

- *Alternative — delete the row*: rejected; an order the system attempted during a halt is audit-relevant. The existing status lifecycle already has rejected as a terminal state; marking preserves the trail and keeps `waitForOrderRecord`/cache paths consistent.

### 8. Nit (e): pre-check the gate and defer auto-closes, never fail the tick

`postTickProcessing` consults `CheckOrderGate()` before placing assignment/expiration auto-close orders. While halted: the auto-close request is queued on the playground as a deferred auto-close (the already-constructed close requests plus their fill parameters), the tick completes normally, a `Warn` is logged and `safety_deferred_auto_closes` (gauge) is set; each subsequent tick retries the deferred list first, and on success the close commits exactly as it would have originally. An Alert fires while deferred closes are outstanding — deferral is unhedged exposure the operator must know about.

Revised after adversarial review (finding 2), two hardenings:

- **Persistence**: deferrals originate from drain-once events, so an in-memory-only list loses them on halt + restart. Each deferral is persisted to the `deferred_auto_closes` table (playground ID, source order ID, serialized close request, fill parameters) when created, deleted when its close commits, and reloaded at playground load. A reloaded deferral whose source order no longer has remaining open quantity is discarded as stale (replaying a committed close would reverse the position); a resurrected stale row that slips past would additionally be rejected by the pending-close quantity check at placement rather than double-closing.
- **Sticky Alert**: the deferred-auto-close alert never auto-resolves purely from the in-memory condition — its firing condition is the authoritative deferral count (the DB-rehydrated playground sets, with the gauge only as fallback), and once fired it keeps notifying until the operator acknowledges, resolving only when acknowledged AND the deferral set is actually empty. A restart or reset metric can never fake an all-clear.

- *Alternative — bypass the halt for auto-closes (they are risk-reducing, like liquidations)*: defensible under the same principle as Decision 4, but rejected for this change: assignment auto-closes cross the *full* order-placement path (`dbService.PlaceOrders`, DB row creation, queue commit), so "bypass" would mean threading an exemption through the gated seam itself — the exemption-list design Decision 4 rejects. Deferral preserves the single-gate invariant, matches the reviewed remedy ("queue/skip gracefully"), and the Alert covers the exposure window. If deferral proves operationally painful, promoting auto-closes to a below-gate path is a future change with its own spec delta.
- *Event-once risk*: assignment/expiration events fire once; naive skipping would lose the close forever. The deferred-list design is what makes "skip" safe — the close *request* is retained, not the event.

## Risks & Mitigations

- **Real orders during autonomous verification**: structurally impossible in the task plan — no task starts a Paper/Margin session; unit/e2e tasks bind the seam to MockBroker; Simulation mode places no stops by spec. Live tasks are quarantined in the operator-only group.
- **Runaway stop placement (stop begets stop)**: tag exclusion + entry-only eligibility + a dedicated regression test that feeds a companion-stop fill event back through the pipeline and asserts no second stop.
- **Duplicate stops on event redelivery/restart**: idempotency per Decision 3; duplicate-direction tests; residual restart-window risk documented and accepted (duplicates are protective-side).
- **Deferred auto-closes accumulating unbounded exposure during a long halt**: Alert while outstanding; gauge visible in Telemetry; the cooldown protocol already forces operator engagement before release, at which point deferred closes commit on the next tick.
- **Orphan-row fix breaking Simulation ordering**: the finalization path only runs where the row was pre-created (non-Simulation); Simulation is byte-for-byte unchanged, locked by existing suites.

## Migration / Rollback

- **Migration**: none — no schema changes (rejected status and order attributes already exist). Feature ships disabled; the operator arms it by setting `COMPANION_STOP_DISTANCE` after completing the operator-only verification.
- **Rollback**: unset `COMPANION_STOP_DISTANCE` (feature off, no redeploy) or revert the change commits. Nit fixes (d)/(e) have no rollback hazard: they only change failure-path behavior (orphan row → rejected row; failed tick → deferred close), and reverting restores the prior, reviewed-as-buggy behavior without data cleanup.
