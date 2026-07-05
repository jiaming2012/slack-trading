## Context

HANDOFF.md lists "End-to-end sim smoke test (server + demo strategy)" as NOT verified and recommends it as the next step. The existing closest artifact, `src/clients/python/tests/test_demo_covered_call.py`, is a *reference-metrics regression test*: it assumes a server is already running externally, drives the full covered-call strategy over a ~2-month AAPL window, and takes 10-15 minutes to pin exact P&L figures. That test is valuable but unsuitable as a nightly gate — it is slow, depends on the covered-call signal engine actually firing (nondeterministic timing), and does not manage server lifecycle itself. This change adds a separate, much smaller harness purpose-built to answer one question fast and unattended: "does the platform still boot and complete a real Simulation loop with a real fill?"

## Goals / Non-Goals

**Goals:**
- Boot the real Go server (not a mock) in development mode, headlessly, from a single command.
- Exercise the real path: Twirp → `CreatePlaygroundPolygon` → Simulation Playground → Tick loop → `PlaceOrder` → Simulated Broker fill → account/order state readback → teardown.
- Bound both the tick count and wall-clock time so the test is safe to run every night without risk of hanging.
- Produce a clean process exit code (0/nonzero) usable as gate G5 by every other change tonight and in the future.
- Ship a Taskfile target (`task test:smoke`) per project convention — no bare script without a wrapper.

**Non-Goals:**
- Not a byte-for-byte reference-metrics regression test (that remains `test_demo_covered_call.py`'s job; unchanged by this proposal).
- Not testing Paper or Margin Mode (real broker / Tradier sandbox paths) — Simulation Mode only, no network calls beyond the Feed (Polygon/Massive) and localhost Twirp.
- Not testing the covered-call options strategy's signal-generation logic or options-chain fetch timing. The smoke test places one deterministic stock order directly rather than waiting on a strategy signal, specifically to keep runtime and pass/fail bounded and independent of market-data-dependent signal timing.
- Not a CI/cron wiring change (e.g. GitHub Actions) — only the local, invocable Taskfile target. Scheduling it is a future capability.
- No changes to server code, the Twirp API surface, or `engine/client.py` — purely additive test-harness files.
- Not a replacement for `task test` (Go unit tests), `task test:e2e` (live-account Tradier tests), or `task test:integration`.

## Decisions

**File / package layout** (all new, no existing file is modified except `taskfile.yml`):
- `src/clients/python/tests/e2e_smoke_strategy.py` — minimal "smoke strategy" driver: places one deterministic market buy order for the underlying stock shortly after playground creation, independent of `deprecated/covered_call.py` or `strategies/covered_call_v2.py` signal logic.
- `src/clients/python/tests/test_e2e_smoke.py` — pytest module: assumes a Twirp server is already reachable (default `http://127.0.0.1:5051`, overridable via env/arg matching `demo_covered_call.py`'s `--twirp-host` convention); creates the Simulation Playground for AAPL over a short, fixed 2025 date window (days, not months — short enough to bound runtime, per the CLAUDE.md gotcha that AAPL 2025 dates are known to work with Polygon); drives the bounded tick loop via `e2e_smoke_strategy.py`; asserts the fill and clean teardown. Pytest's own process exit code (0/nonzero) is the pass/fail signal consumed by the orchestration script.
- `src/clients/python/run_e2e_smoke_test.sh` — the headless orchestration script wired to `task test:smoke`: boots `go run ./main.go` from `cmd/` with `GO_ENV=development` (mirroring `cmd/run-dev.sh`'s env-var resolution for `OPTIONS_CONFIG_PATH`) as a background process; polls Twirp port 5051 only (ignoring REST 8080 per the known conflict gotcha) with a bounded retry loop; runs the pytest module; captures its exit code; sends SIGTERM (SIGKILL fallback after a grace period) to the server's process group for teardown regardless of pass/fail; exits with the pytest exit code.
- `taskfile.yml` — new `test:smoke` target, `dir: src/clients/python`, invoking `./run_e2e_smoke_test.sh`.

**Naming deviation from the brief**: the brief's example was `task test-smoke`. This repo's existing Taskfile convention is colon-namespaced (`test:e2e`, `test:integration`, `test`), so the implementation uses `task test:smoke` instead, for consistency with existing operator muscle memory. Noted here explicitly as a judgment call.

**Why a bespoke smoke strategy instead of reusing `demo_covered_call.py` directly**: the covered-call strategy's signal-generation logic takes an unpredictable number of ticks to fire (or may not fire at all on a short window), which would make the bounded-tick-loop requirement either flaky or require a much longer window (and 10-15 minute runtime), defeating the purpose of a fast nightly gate. Placing one deterministic order right after playground creation keeps the test's timing and pass condition fully within the harness's control while still exercising the identical `PlaceOrder` → Simulated Broker → fill → account-update path that any real strategy uses.

**Data flow**: orchestration script (bash) → `go run ./main.go` (Go server, dev mode) → Twirp readiness poll (port 5051 only) → Python `BacktesterPlaygroundClient` creates Simulation Playground (`CreatePolygonPlaygroundRequest`, AAPL, short 2025 window) → bounded Tick loop → `PlaceOrder` for one stock buy → poll `GetAccount`/`GetOpenOrders` until filled, max-tick-count, or stop-date → teardown (remove playground, terminate server process group) → exit code propagation back through the orchestration script.

## Dependency ordering on other batch changes

No hard technical dependency. Per `todo/overnight-run-plan-20260704.md`, this is scheduled as Phase 2 step 5 (after `migrate-crossed-enums`), but purely for sequencing convenience — once merged it becomes the reusable G5 gate that later Phase 3 changes can invoke. It touches only Simulation Mode via pre-existing Twirp RPCs, so it can be authored and merged against HEAD independent of the outcome of `reconcile-models-packages`, `rename-event-packages`, `close-gorm-db-leaks`, or `migrate-crossed-enums`. If any of those earlier Phase 2 changes fail and are reverted, this change is unaffected and can still land.

## Verification gates

- **G1** — `go build ./src/go/... ./cmd/...` stays green. This change adds no Go code, so G1 here is a regression check that the harness's assumptions (server still builds and boots the same way) still hold.
- **G5** — the smoke test itself is the gate: a headless run of `task test:smoke` exits 0, with evidence captured (stdout/stderr log showing playground ID, the filled order, and clean teardown) in the same session that authors it. This is the change's own acceptance test, and becomes the standing gate for every change that runs after it tonight.
- G2/G3/G4 are not applicable to this change (no Go core packages, no Python production/client code, and no fixed-scenario diff-test are touched).

## Deferred validation

None. This is a Tier A card — full authoring, G1, and G5 are expected to complete within this run; there is no external dependency (Tradier sandbox, prod DB, new infra) blocking any part of it.

## Risks / Trade-offs

- **Polygon/Feed availability**: if the Feed is unreachable or rate-limited when the smoke test runs, G5 will fail even though the harness itself is correct. Per the overnight risk register, this change (along with `scanner-l1-l2`) is explicitly called out as one that may "degrade to skipped-with-note" if Polygon API calls fail — a Feed outage is not a harness bug and should be reported as such in the morning report rather than triggering a revert.
- **Process-lifecycle fragility**: managing a background `go run` process from bash (PID tracking, process-group signaling) is inherently more fragile than in-process test harnesses. Mitigated by always attempting teardown on both pass and fail paths (see Requirement: Clean teardown) and by a bounded wall-clock timeout so a hung server cannot block the gate indefinitely.
- **Short date window may reduce realism**: using a multi-day window instead of `test_demo_covered_call.py`'s multi-month window trades strategy realism for speed and determinism. Acceptable because this test's job is "does the platform still work end to end," not "does the strategy still produce the same P&L" (that remains the other test's job).
