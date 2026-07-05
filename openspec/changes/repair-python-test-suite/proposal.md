# Repair the Python test suite (retire the 179-failure baseline)

## Why

The Python client suite carries 179 permanently-failing tests (committed as the G3 parity baseline in `tests/baselines/pytest-failures-20260704.txt`), all caused by test doubles frozen against production code that moved on — not by broken product code. A suite that is 42% red cannot catch real regressions except by baseline-diffing, and the baseline discipline is a workaround, not a state to live in.

## What Changes

- Repair the stale test doubles so `pytest tests/` (with the existing e2e exclusions) is **fully green**:
  - The three strategy test files (147 failures) get a shared, contract-faithful mock playground fixture — their `MagicMock` playgrounds fail at strategy construction because they never stub accessors the constructors now read (e.g. current price → `TypeError: MagicMock > int`). The strategies under test live in the Python `deprecated/` package but are **live-reachable** (imported by `demos/` and `strategies/` wrappers), so they are repaired, not deleted.
  - Engine-facing tests (~14 failures: otel lifecycle, heartbeat, trace propagation) get test doubles matching the current strategy interface (`on_signal` is now required by `engine/trading_engine.py`).
  - Persistence tests (4 failures) get fixtures matching the current 4-column stats row (`total_trades, total_pnl, win_rate, profit_factor` unpack at `engine/persistence.py:93`).
  - `test_demo_covered_call.py` (8 failures) and the remaining small buckets are triaged case-by-case; any drifted pinned reference metric is re-baselined **only with a committed explanation of why the drift is legitimate** — never silently.
- **Retire the G3 baseline**: once green, the parity gate ("no NEW failures vs 179 IDs") becomes a plain zero-failure gate; the baseline file is removed and the HANDOFF/gotcha documentation updated.
- Add a `task test:python` Taskfile target running the suite headlessly (project convention: operator-visible commands ship with a task target), making the green suite a first-class gate for future changes.
- No production Python or Go code behavior changes. Test-only, plus the Taskfile target and docs.

## Capabilities

### New Capabilities

- `python-test-suite-health` — a green Python suite with contract-faithful test doubles, a zero-failure gate replacing the parity baseline, and the `task test:python` entry point.

### Modified Capabilities

_None — no existing spec's requirements change; e2e-sim-smoke-test and model-migration-diff-test gates are unaffected._

## Impact

- **Files**: `src/clients/python/tests/**` (the 11 failing files + a new shared fixture module/conftest addition), `taskfile.yml` (one target), `tests/baselines/pytest-failures-20260704.txt` (deleted at the end), HANDOFF.md gotcha note.
- **Explicitly untouched**: strategy/engine production code (`deprecated/`, `strategies/`, `engine/` behavior), Go code, reference fixtures for model-diff/smoke.
- **Sizing**: 147 of 179 failures share one root cause (unstubbed playground accessors), so the bulk is one fixture + mechanical adoption; the tail (~32) is per-case.
- **Risk**: low — test-only; the main judgment risk is re-pinning `test_demo_covered_call` metrics for the wrong reason, which the spec forbids doing silently.
- **Follow-up candidate (not this change)**: `python-strategy-module-reorg` — the `deprecated/` package name is a lie (live code imports it); renaming/moving it is a separate mechanical change.
