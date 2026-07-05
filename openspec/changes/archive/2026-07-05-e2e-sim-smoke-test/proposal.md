## Why

Nothing in the repo currently boots the server and drives a full Simulation-mode strategy loop end to end in one shot; HANDOFF.md lists this as a NOT-verified gap. Every other overnight change needs a fast, deterministic pass/fail signal (G5) that proves the platform still actually runs, not just that it compiles.

## What Changes

- Add a headless, permanent end-to-end smoke test that boots the Go server in development mode, creates a Simulation Playground over AAPL 2025 data via Twirp, drives a bounded Tick loop that places one deterministic order, asserts the order reaches filled, and tears everything down.
- Add a new orchestration script (`src/clients/python/run_e2e_smoke_test.sh`) that manages server boot, Twirp-port readiness polling, teardown, and exit-code propagation — tolerating the known port-8080 REST conflict per the CLAUDE.md gotcha (it only waits on Twirp port 5051).
- Add a minimal "smoke strategy" driver module, decoupled from the existing covered-call demo's signal-generation logic, so the test's runtime and pass/fail condition are bounded and deterministic rather than dependent on when (or whether) a real trading signal fires.
- Add a new Taskfile target, `task test:smoke`, as the operator-facing entry point (project convention: no new command without a Taskfile wrapper).
- No production Go or Python client code is modified — this change is purely additive test-harness surface. Nothing here is **BREAKING**.

## Capabilities

### New Capabilities
- `e2e-sim-smoke-test`: A headless, bounded, exit-code-driven end-to-end smoke test (server boot → Twirp → Simulation Playground → bounded Tick loop → order fill → teardown) exposed via `task test:smoke`, intended to serve as the permanent gate (G5) for this change and every future change tonight and beyond.

### Modified Capabilities
(none — no existing specs exist in this repo yet, and no existing requirement's behavior changes)

## Impact

- **New files only**: `src/clients/python/run_e2e_smoke_test.sh`, `src/clients/python/tests/e2e_smoke_strategy.py`, `src/clients/python/tests/test_e2e_smoke.py`, plus a new `test:smoke` entry in `taskfile.yml`.
- **Reads but does not modify**: `cmd/run-dev.sh` (env-var pattern for `GO_ENV` / `OPTIONS_CONFIG_PATH`), `engine/client.py` (`BacktesterPlaygroundClient`, `CreatePolygonPlaygroundRequest`), the existing Twirp RPC surface (`CreatePolygonPlaygroundRequest`, `NextTick`, `PlaceOrder`, `GetAccount`/`GetOpenOrders`).
- **Dependencies on other changes in this batch**: none technically required. Per `todo/overnight-run-plan-20260704.md` this lands as Phase 2 step 5 (after `migrate-crossed-enums`) purely for scheduling reasons — once merged it becomes the reusable G5 gate for Phase 3 changes. It exercises only Simulation mode, which is unaffected by the enum-migration or reconcile-models refactors landing earlier in Phase 2; it can be implemented and merged against HEAD independent of their outcome.
- **Verification gates**: G1 (`go build ./src/go/... ./cmd/...`) and G5 (the smoke test's own first green headless run via `task test:smoke`). G2/G3/G4 are not applicable — no Go core code or Python production code is touched.
