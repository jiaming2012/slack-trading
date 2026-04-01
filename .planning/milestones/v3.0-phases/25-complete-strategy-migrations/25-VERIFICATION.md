---
phase: 25-complete-strategy-migrations
verified: 2026-03-31T12:05:00Z
status: gaps_found
score: 4/6 success criteria verified
gaps:
  - truth: "Each migrated strategy passes its behavioral diff test"
    status: failed
    reason: "15 of 26 diff tests fail — stale @patch() decorators in 4 test files reference strategies.<v1_module> which no longer exists; V1s moved to deprecated/"
    artifacts:
      - path: "src/clients/python/tests/test_pdf_wheel_diff.py"
        issue: "@patch('strategies.pdf_wheel.detect_atomic_signals_on_bar') fails — module moved to deprecated.pdf_wheel; 4 tests fail"
      - path: "src/clients/python/tests/test_credit_spread_diff.py"
        issue: "@patch('strategies.credit_spread._bar_to_dict') and @patch('strategies.credit_spread.detect_atomic_signals_on_bar') fail; 5 tests fail"
      - path: "src/clients/python/tests/test_mean_reversion_diff.py"
        issue: "@patch('strategies.mean_reversion._bar_to_dict') and @patch('strategies.mean_reversion.detect_atomic_signals_on_bar') fail; 3 tests fail"
      - path: "src/clients/python/tests/test_options_mean_reversion_diff.py"
        issue: "@patch('strategies.options_mean_reversion._bar_to_dict') and @patch('strategies.options_mean_reversion.detect_atomic_signals_on_bar') fail; 3 tests fail"
    missing:
      - "Update all @patch('strategies.<v1_module>.*') decorators in 4 diff test files to @patch('deprecated.<v1_module>.*')"
  - truth: "Trading engine runs end-to-end with only V2 strategies"
    status: partial
    reason: "run_strategy() in trading_engine.py is generic (BaseStrategy) and does wire on_signal(), but no demo or run script instantiates any V2 strategy class — all demos still use V1 classes via deprecated.* imports"
    artifacts:
      - path: "src/clients/python/demos/demo_pdf_wheel_strategy.py"
        issue: "Instantiates PDFWheelStrategy (V1 from deprecated), not PDFWheelStrategyV2"
      - path: "src/clients/python/demos/demo_covered_call.py"
        issue: "Instantiates OptionsStrategyBasic (V1 from deprecated), not OptionsStrategyBasicV2"
      - path: "src/clients/python/demos/demo_credit_spread.py"
        issue: "Instantiates CreditSpreadStrategy (V1 from deprecated), not CreditSpreadStrategyV2"
      - path: "src/clients/python/demos/demo_wheel_strategy.py"
        issue: "Instantiates WheelStrategy (V1 from deprecated)"
    missing:
      - "At minimum one entry-point script (demo or trading_engine main) must instantiate a V2 strategy class and call run_strategy() to prove end-to-end V2 execution"
      - "Or the ROADMAP success criterion must be scoped to 'run_strategy() wired for V2 dispatch' (the callback exists) rather than 'end-to-end V2 strategy run'"
human_verification:
  - test: "Run a backtest with a V2 strategy (e.g., demo_mean_reversion_v2.py if it instantiates MeanReversionStrategyV2)"
    expected: "Strategy executes tick loop, on_signal() receives signals from TickDelta, no import errors"
    why_human: "Requires live server on port 5051 and Polygon data; cannot verify programmatically"
---

# Phase 25: Complete Strategy Migrations — Verification Report

**Phase Goal:** All strategies migrated to TradeSignal framework, on_signal() wired in Python client, V1 files moved to deprecated/
**Verified:** 2026-03-31T12:05:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | covered_call_v2.py and wheel_v2.py exist with datasource modules and diff tests | VERIFIED | Files exist, correct class hierarchy, 9 diff tests pass |
| 2 | PDFWheel V2 strategy and datasource exist with diff test | VERIFIED | pdf_wheel_v2.py, pdf_wheel_signals.py exist; 2/6 tests pass (4 fail — stale patches) |
| 3 | Python client.py tick() extracts new_signals from TickDelta and calls strategy.on_signal() for each | VERIFIED | _signal_callback attr at line 321, dispatch loop at lines 622-626 |
| 4 | Trading engine runs end-to-end with only V2 strategies | PARTIAL | run_strategy() is wired for V2; no entry point actually instantiates a V2 strategy |
| 5 | All V1 strategy files moved to deprecated/ folder | VERIFIED | All 6 V1 files absent from strategies/, present in deprecated/ |
| 6 | Each migrated strategy passes its behavioral diff test | FAILED | 15/26 diff tests fail — stale @patch() targets in 4 test files |

**Score:** 4/6 truths fully verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `src/clients/python/datasources/covered_call_signals.py` | produce_open_signals for supertrend signal extraction | VERIFIED | Exists, contains `def produce_open_signals`, callable-based pattern |
| `src/clients/python/strategies/covered_call_v2.py` | OptionsStrategyBasicV2 class | VERIFIED | Exists, class present, imports `from datasources.covered_call_signals import produce_open_signals` |
| `src/clients/python/tests/test_covered_call_diff.py` | Behavioral diff test V1 vs V2 | VERIFIED | 4/4 tests pass |
| `src/clients/python/datasources/wheel_signals.py` | produce_open_signals for put sell signals | VERIFIED | Exists, contains `def produce_open_signals` |
| `src/clients/python/strategies/wheel_v2.py` | WheelStrategyV2(OptionsStrategyBasicV2) | VERIFIED | Exists, correct inheritance, imports from datasources.wheel_signals |
| `src/clients/python/tests/test_wheel_diff.py` | Behavioral diff test V1 vs V2 wheel | VERIFIED | 5/5 tests pass |
| `src/clients/python/datasources/pdf_wheel_signals.py` | produce_signals for compound signals | VERIFIED | Exists, contains `def produce_signals` |
| `src/clients/python/strategies/pdf_wheel_v2.py` | PDFWheelStrategyV2(WheelStrategyV2) | VERIFIED | Exists, correct inheritance |
| `src/clients/python/tests/test_pdf_wheel_diff.py` | Behavioral diff test V1 vs V2 PDFWheel | STUB | File exists but 4/6 tests fail — stale @patch("strategies.pdf_wheel.*") |
| `src/clients/python/engine/client.py` | on_signal() dispatch from TickDelta.new_signals | VERIFIED | _signal_callback at line 321; dispatch loop at lines 622-626 |
| `src/clients/python/deprecated/covered_call.py` | Archived V1 CoveredCall | VERIFIED | Exists in deprecated/ |
| `src/clients/python/deprecated/wheel.py` | Archived V1 Wheel | VERIFIED | Exists in deprecated/ |
| `src/clients/python/deprecated/pdf_wheel.py` | Archived V1 PDFWheel | VERIFIED | Exists in deprecated/ |
| `src/clients/python/deprecated/mean_reversion.py` | Archived V1 MeanReversion | VERIFIED | Exists in deprecated/ |
| `src/clients/python/deprecated/options_mean_reversion.py` | Archived V1 OptionsMeanReversion | VERIFIED | Exists in deprecated/ |
| `src/clients/python/deprecated/credit_spread.py` | Archived V1 CreditSpread | VERIFIED | Exists in deprecated/ |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| covered_call_v2.py | covered_call_signals.py | import produce_open_signals | WIRED | Line 38: `from datasources.covered_call_signals import produce_open_signals` |
| wheel_v2.py | covered_call_v2.py (OptionsStrategyBasicV2) | class WheelStrategyV2(OptionsStrategyBasicV2) | WIRED | Line 47: `class WheelStrategyV2(OptionsStrategyBasicV2):` |
| wheel_v2.py | wheel_signals.py | import produce_open_signals | WIRED | Line 32: `from datasources.wheel_signals import produce_open_signals` |
| pdf_wheel_v2.py | wheel_v2.py (WheelStrategyV2) | class PDFWheelStrategyV2(WheelStrategyV2) | WIRED | Line 70: `class PDFWheelStrategyV2(WheelStrategyV2):` |
| pdf_wheel_v2.py | pdf_wheel_signals.py | import produce_signals | WIRED | Line 60: `from datasources.pdf_wheel_signals import produce_signals` |
| client.py | base_strategy.py (on_signal callback) | _signal_callback = strategy.on_signal | WIRED | Line 321 init; lines 622-626 dispatch loop |
| trading_engine.py | client.py | playground._signal_callback = strategy.on_signal | WIRED | Line 117: set before tick loop |

### Data-Flow Trace (Level 4)

Not applicable — this phase produces strategy routing logic and file organization, not UI components rendering dynamic data.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All V2 strategy imports succeed | `python -c "from strategies.covered_call_v2 import OptionsStrategyBasicV2; ..."` | All V2 imports OK | PASS |
| client.py imports cleanly | `python -c "from engine.client import BacktesterPlaygroundClient; print('import OK')"` | import OK | PASS |
| Covered call + wheel diff tests | `pytest test_covered_call_diff.py test_wheel_diff.py` | 9/9 pass | PASS |
| PDF wheel diff tests | `pytest test_pdf_wheel_diff.py` | 2/6 pass, 4 fail | FAIL |
| Credit spread diff tests | `pytest test_credit_spread_diff.py` | 0/5 pass, 5 fail | FAIL |
| Mean reversion diff tests | `pytest test_mean_reversion_diff.py` | 0/3 pass, 3 fail | FAIL |
| Options mean reversion diff tests | `pytest test_options_mean_reversion_diff.py` | 0/3 pass, 3 fail | FAIL |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| MIG-01 | 25-01, 25-02, 25-03 | All existing strategies migrated to consume TradeSignals instead of inline signal detection | PARTIAL | V2 classes exist and import from datasources; but diff tests for 4 strategies fail |
| MIG-02 | 25-03 | Original strategy files moved to deprecated/ folder | SATISFIED | All 6 V1 files confirmed absent from strategies/, present in deprecated/ |
| MIG-03 | Phase 21 (prior) | Each migration validated with behavioral diff tests | PARTIAL | 11/26 diff tests pass; 15 fail due to stale @patch() targets from the V1 move |

Note from REQUIREMENTS.md traceability: MIG-01 and MIG-02 map to Phase 25 and are marked `[x]` (complete) in REQUIREMENTS.md. That marking should reflect the actual test results — MIG-01 is not fully satisfied while 15 diff tests are broken.

**No orphaned requirements:** MIG-01 and MIG-02 appear in both plans (25-01 through 25-03) and REQUIREMENTS.md. No unmapped IDs found.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| tests/test_pdf_wheel_diff.py | 177, 201, 238, 265 | `@patch("strategies.pdf_wheel.*")` — module no longer at this path | Blocker | 4/6 PDFWheel diff tests fail with ModuleNotFoundError |
| tests/test_credit_spread_diff.py | 177-179, 228-229 | `@patch("strategies.credit_spread.*")` — module no longer at this path | Blocker | 5/5 credit spread diff tests fail |
| tests/test_mean_reversion_diff.py | 177-179, 210, 234 | `@patch("strategies.mean_reversion.*")` — module no longer at this path | Blocker | 3/3 mean reversion diff tests fail |
| tests/test_options_mean_reversion_diff.py | 203-206, 278-279 | `@patch("strategies.options_mean_reversion.*")` — module no longer at this path | Blocker | 3/3 options mean reversion diff tests fail |

**Root cause:** When V1 strategy files were moved from `strategies/` to `deprecated/`, the PLAN correctly noted "Also check diff test files for V1 imports and update them." The direct imports were updated (e.g., `from deprecated.pdf_wheel import PDFWheelStrategy` correctly updated at line 182), but the `@patch()` decorator strings — which are module path strings evaluated at test runtime — were not updated.

### Human Verification Required

1. **End-to-end V2 strategy run**
   **Test:** Start the Go server, then run any demo that instantiates a V2 strategy class through `run_strategy()`. Confirm that `on_signal()` receives signals and no import errors occur.
   **Expected:** Tick loop runs, signals dispatched through _signal_callback to strategy.on_signal(), strategy processes them without error.
   **Why human:** Requires live server on port 5051, Polygon API key, and a scenario where signals are actually generated; cannot stub programmatically.

### Gaps Summary

Two gaps block full phase goal achievement:

**Gap 1 — Stale @patch() decorators (15 test failures):** When V1 strategy files were moved to `deprecated/` in Plan 03, the SUMMARY claims "Updated imports in 20 files." The `from deprecated.<module> import` statements were correctly updated. However, the `@patch("strategies.<v1_module>.*")` decorator strings in 4 diff test files were not updated to `@patch("deprecated.<v1_module>.*")`. The fix is mechanical: replace `strategies.pdf_wheel`, `strategies.credit_spread`, `strategies.mean_reversion`, and `strategies.options_mean_reversion` in the `@patch()` strings within those 4 test files. This directly blocks success criterion 6 ("each migrated strategy passes its behavioral diff test").

**Gap 2 — No V2 strategy instantiated in any runnable entry point:** All demos still use V1 classes imported from `deprecated.*`. The `run_strategy()` function is correctly wired with `_signal_callback`, but success criterion 4 ("trading engine runs end-to-end with only V2 strategies") cannot be confirmed. This is a weaker gap — the infrastructure is in place, the V2 classes exist and import cleanly, but no script exercises them end-to-end. A single demo updated to use a V2 class would close this criterion.

The two gaps share a single root cause: Plan 03 Task 2 incompletely updated references to moved V1 files — it caught `from strategies.<module> import` but missed `@patch("strategies.<module>.*")` strings and demo instantiation sites.

---

_Verified: 2026-03-31T12:05:00Z_
_Verifier: Claude (gsd-verifier)_
