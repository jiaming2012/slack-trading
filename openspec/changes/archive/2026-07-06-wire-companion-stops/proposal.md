# Proposal — wire-companion-stops

## Why

The broker-side companion-stop pipeline (`kill-switch-and-broker-side-stops`, archived 2026-07-05) shipped `PlaceCompanionStop` as a tested library exercised against MockBroker, but nothing in the live fill pipeline invokes it — deliberately, because verifying that wiring overnight would have placed real broker orders. Until it is wired, a live (Paper/Margin) entry fill carries no broker-held protective exit: if our infrastructure dies with a position open, nothing caps the loss. The same review cycle flagged two halt-interaction nits that belong in this pass: a live order rejected by the halt gate leaves an orphan pending row in the database, and option-assignment/expiration auto-closes error the entire tick while a halt is engaged.

## What Changes

- Invoke `PlaceCompanionStop` from the live fill pipeline (`DrainTradierOrderQueue` → `fillPendingOrder` in `src/go/backtester/services/order_queue.go`) on each Paper- or Margin-Mode equity entry fill that opens or increases a position. Closes, adjustments, auto-closes, and companion stops themselves are excluded; placement is idempotent per entry fill.
- Honor the documented prior decision: protective companion stops BYPASS an engaged halt — placement goes directly through the `IBroker` seam, never through the halt-gated submission path, so the kill switch can never strand an open position without its protective exit.
- Companion stops are explicit opt-in: enabled only when a positive stop distance is configured; when unconfigured the feature is off with a loud startup warning (a guessed default stop distance is more dangerous than none).
- A companion-stop placement failure (position left unprotected) logs a warning, increments a Telemetry counter, and pushes an Alert to the operator.
- Review nit (d): when a live (non-Simulation) order is rejected by the halt gate after its database row was pre-created, that row SHALL be finalized as rejected with the halt reason — no orphan pending rows.
- Review nit (e): option-assignment and option-expiration auto-closes attempted while the halt is engaged SHALL NOT error the tick — they are deferred and retried on subsequent ticks until the halt clears, with a warning and a Telemetry counter, and an Alert while deferred closes are outstanding.
- **HARD CONSTRAINT** honored throughout: the autonomous run places no live or Paper broker orders. Every verification runs against MockBroker / Simulation; Tradier-sandbox and Margin verification are explicitly operator-only tasks.

## Capabilities

### New Capabilities

None — this change wires and hardens existing capabilities; no new capability spec is introduced.

### Modified Capabilities

- `broker-side-stop-losses`: the placement requirement's "invoking this routine from the live fill pipeline is deferred to `wire-companion-stops`" is discharged — the pipeline invokes it on live entry fills, with entry/close discrimination, idempotency, and opt-in configuration. New requirements: companion stops bypass an engaged halt (recording the prior decision as spec), and placement failures raise an Alert. The MockBroker requirement is extended to cover end-to-end pipeline verification and to name live verification as operator-only.
- `kill-switch`: new requirements for halt-rejection hygiene — a halt-rejected live order leaves no orphan pending database row (nit d), and option auto-closes during an engaged halt defer gracefully instead of failing the tick (nit e).

## Impact

- **Code**: `src/go/backtester/services/order_queue.go` (companion-stop invocation on live entry fills), `src/go/backtester/safety/companion_stop.go` (idempotency/eligibility helpers as needed), `src/go/data/database_service.go` `commitOrderRecord` (orphan-row finalization on halt rejection), `src/go/backtester/models/playground.go` `postTickProcessing` (deferred auto-closes under halt), `src/go/backtester/models/mock_broker.go` (already records stop orders), `cmd/main.go` (companion-stop config), `taskfile.yml` (no new operator command expected; any added entry point ships with a matching target).
- **Config**: new env var `COMPANION_STOP_DISTANCE` (feature off when unset). No new dependencies, no schema changes, no new infrastructure.
- **Telemetry**: counters for companion stops placed/failed and deferred auto-closes on the internal registry (`telemetry.Default`) plus AlertEngine rules; per ADR-0005 no OpenTelemetry is referenced.
- **Behavioral risk**: once enabled by the operator, every live equity entry fill produces one additional real broker order (the stop). Wrong distance configuration is guarded by validation (positive-only) and by opt-in default-off. Simulation behavior is unchanged.
- **Autonomous-run boundaries respected**: MockBroker/Simulation verification only; live wiring is code-complete but its live-fire proof is the operator's final task group.
