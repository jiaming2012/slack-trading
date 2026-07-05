# Design — repair-python-test-suite

## Evidence base (gathered 2026-07-05)

Failure distribution of the 179 baseline IDs, with sampled root causes:

| Bucket | Count | Files | Root cause (sampled traceback) |
|---|---|---|---|
| A. Stale mock playgrounds | 147 | test_options_mean_reversion_strategy (54), test_mean_reversion_strategy (49), test_credit_spread_strategy (44) | Strategy constructors read accessors the `MagicMock` playground never stubs → `TypeError: '>' not supported between 'MagicMock' and 'int'` at `deprecated/mean_reversion.py:120` |
| B. Engine interface drift | ~14 | test_trading_engine_otel (10), test_heartbeat (2), test_trace_propagation (2) | `MockStrategy` lacks `on_signal`, now required (`engine/trading_engine.py:114` wires `playground._signal_callback = strategy.on_signal`) |
| C. Persistence row-shape drift | 4 | test_persistence | Production unpacks 4 columns (`engine/persistence.py:93`); fixtures feed old shape |
| D. Per-case triage | ~14 | test_demo_covered_call (8), test_mean_reversion_diff (2), test_otel (2), test_client_trace_id (2) | Mixed; demo_covered_call pins reference metrics |

## Key decision: repair, not delete

The tested strategies live in the Python `deprecated/` package, but that package is **live-reachable**: `demos/demo_mean_reversion.py` and `demos/demo_options_mean_reversion.py` import the two mean-reversion modules; `strategies/wheel_v2.py`, `covered_call_v2.py`, `pdf_wheel_v2.py`, `engine/trading_engine.py`, and four other demos import six more `deprecated.*` modules. Deleting the tests would strip coverage from running code. The package's name is the actual defect — deferred to a future `python-strategy-module-reorg` card.

## Bucket A approach: one contract-faithful fixture

- Build `tests/fixtures/mock_playground.py` (or a conftest fixture): a hand-written fake (not bare `MagicMock`) exposing the accessors strategy code reads, each returning a correctly-typed default and overridable per-test.
- Derive the accessor list empirically: instrument construction/tick of the three strategies against a spy, not by guessing.
- Adopt it mechanically across the three files; per-test overrides preserve each test's scenario intent (the assertions themselves are expected to be largely still valid — the failures are at setup, before any assertion runs).
- Anti-silent-leak rule: the fake raises `AttributeError` for unknown accessors instead of returning a new mock, satisfying the "fixture drift is detectable" requirement.

## Bucket D rules

`test_demo_covered_call.py` pins reference metrics. Diagnosis order: (1) does the demo still run at all headlessly; (2) if metrics drifted, bisect to the causing commit; (3) re-pin ONLY with the cause named in the commit body (spec forbids silent re-pins); (4) if no cause is traceable, STOP and surface — do not re-pin.

## Gate/document updates at completion

- Delete `tests/baselines/pytest-failures-20260704.txt`; update HANDOFF.md "G3 discipline" gotcha to the zero-failure rule.
- `task test:python` added; existing gates (G1/G2/G4/G5, lint, leak-guard) unaffected — this change touches no Go code and no reference fixtures other than possibly the explained demo re-pin.

## Out of scope

- Renaming/moving the Python `deprecated/` package (future `python-strategy-module-reorg`).
- The two pre-existing Go `workers` test failures (uint64/uint) — Go-side, separate concern.
- `test_strategy_e2e.py` (already excluded by convention) and the e2e smoke harness (owned by e2e-sim-smoke-test).
- Fixing any real product bug a repaired test might newly reveal: if a repaired test fails against genuine current behavior, that is a STOP-and-report (possible real regression), not a test tweak.

## Verification gates

- `task test:python` green (the change's own deliverable is the gate), zero failures, run twice for flake-resistance.
- G1 `go build` + G2 `task test` (must be untouched/green), `task test:smoke` once (proves no engine/client contract was altered by fixture work).
- G6 Fable adversarial review before archive (test-honesty review: no assertion weakened to force green — reviewer diffs assertions, not just setup).
