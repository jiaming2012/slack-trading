# Phase 21: First Strategy Migration & Validation - Research

**Researched:** 2026-03-30
**Domain:** Python strategy migration, behavioral diff testing, datasource extraction
**Confidence:** HIGH

## Summary

Phase 21 migrates MeanReversionStrategy to consume TradeSignals from a new `datasources/ma_crossover.py` module instead of detecting signals inline. The strategy is well-structured for extraction: the signal detection logic lives in `_process_htf_candle()` (lines 236-261), which calls `detect_atomic_signals_on_bar()` from `lib/pdf_builder.py`, then does compound key construction and PDF lookup. This is a clean extraction boundary.

The WriteSignal RPC (Phase 19) has NOT been implemented yet -- the proto file has no WriteSignal endpoint. Per D-02/D-03 from CONTEXT.md, sim mode uses direct Python import (`from datasources.ma_crossover import produce_signals`) so no RPC is needed for this phase. The datasource module produces signal dicts that the migrated strategy consumes directly.

**Primary recommendation:** Extract the HTF signal detection from `_process_htf_candle()` into `datasources/ma_crossover.py` as `produce_signals(bar_dict, prev_bar, pdf)`, return a list of signal dicts. The migrated strategy reads these from TickDelta.new_signals in live mode, or calls produce_signals() directly in sim mode. Behavioral diff test runs both strategies on the same playground and compares trade-level output.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: First strategy is MeanReversionStrategy -- most explicit signal detection code (MA crossover logic in on_tick), clearest signal boundaries, most tests
- D-02: Datasource module at `src/clients/python/datasources/ma_crossover.py` with `produce_signals(candle_data)` returning list of TradeSignal dicts; sim mode imports directly, live mode uses WriteSignal RPC from `__main__`
- D-03: Migrated strategy at `src/clients/python/strategies/mean_reversion_v2.py`; original moved to `deprecated/` after validation; migrated strategy consumes TradeSignals from TickDelta.new_signals instead of computing inline

### Claude's Discretion
- D-04: Diff test implementation details (framework, assertions, test data source)
- How to extract MA crossover logic cleanly from on_tick()
- Whether MeanReversionStrategyV2 extends BaseStrategy or is a new class
- Demo script updates for the migrated strategy

### Deferred Ideas (OUT OF SCOPE)
- Remaining 6 strategy migrations -- Phase 22
- Moving original to deprecated/ -- happens after diff test passes (could be Phase 22)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MIG-03 | Each migration validated with behavioral diff tests against original strategy output | Signal extraction pattern identified; playground_metrics.py provides P&L/trade comparison; pytest framework exists with mock helpers in test_mean_reversion_strategy.py |
</phase_requirements>

## Architecture Patterns

### Signal Detection Extraction Boundary

The signal detection in MeanReversionStrategy lives in `_process_htf_candle()` (mean_reversion.py lines 236-261):

```python
def _process_htf_candle(self, bar_dict: dict) -> None:
    # 1. Detect atomic signals
    signals = detect_atomic_signals_on_bar(
        bar_dict, self._prev_htf_bar, timeframe="ltf",
    )
    self._prev_htf_bar = bar_dict

    # 2. Build compound key and PDF lookup
    if signals and self.pdf:
        key = "|".join(sorted(signals))
        pdf_entry = self.pdf.get_signal(key)
        if pdf_entry and pdf_entry.sufficient_samples:
            horizon = pdf_entry.horizons.get(self.htf_horizon)
            if horizon and horizon.mean > 0:
                self._try_create_group(key, bar_dict, pdf_entry)

    # 3. Evaluate stops
    self._evaluate_stops(bar_dict.get("close", 0.0))
```

The extraction target is steps 1-2: detecting signals and determining if they constitute a valid trade signal. Step 3 (stop evaluation) stays in the strategy.

### Datasource Module Pattern

```python
# datasources/ma_crossover.py

from lib.pdf_builder import detect_atomic_signals_on_bar

def produce_signals(bar_dict: dict, prev_bar: dict, pdf) -> list:
    """Produce MA crossover trade signals from an HTF bar.

    Returns list of dicts with keys: name, bar_dict, pdf_entry, signal_key
    Empty list if no valid signal detected.
    """
    signals = detect_atomic_signals_on_bar(
        bar_dict, prev_bar, timeframe="ltf",
    )

    if not signals or not pdf:
        return []

    key = "|".join(sorted(signals))
    pdf_entry = pdf.get_signal(key)

    if not pdf_entry or not pdf_entry.sufficient_samples:
        return []

    # Return signal dict for each valid horizon
    return [{
        "name": "ma_crossover",
        "signal_key": key,
        "bar_dict": bar_dict,
        "pdf_entry": pdf_entry,
    }]
```

### Migrated Strategy Pattern

The V2 strategy replaces inline signal detection with signal consumption:

```python
# strategies/mean_reversion_v2.py

class MeanReversionStrategyV2(BaseStrategy):
    """Same logic as MeanReversionStrategy but consumes signals
    instead of detecting them inline."""

    def _process_htf_candle(self, bar_dict: dict) -> None:
        self.funnel["htf_bars"] += 1
        self._prev_htf_bar = bar_dict

        # Instead of detect_atomic_signals_on_bar():
        # consume signals from produce_signals() (sim mode)
        from datasources.ma_crossover import produce_signals
        trade_signals = produce_signals(bar_dict, self._prev_htf_bar_for_signals, self.pdf)
        self._prev_htf_bar_for_signals = bar_dict

        for sig in trade_signals:
            pdf_entry = sig["pdf_entry"]
            horizon = pdf_entry.horizons.get(self.htf_horizon)
            if horizon and horizon.mean > 0:
                self._try_create_group(sig["signal_key"], bar_dict, pdf_entry)

        self._evaluate_stops(bar_dict.get("close", 0.0))
```

### Recommended Project Structure

```
src/clients/python/
  datasources/
    __init__.py
    ma_crossover.py          # produce_signals() -- extracted from MeanReversionStrategy
  strategies/
    mean_reversion.py        # Original (unchanged until deprecation)
    mean_reversion_v2.py     # Migrated version consuming signals
    base_strategy.py         # Unchanged
  tests/
    test_mean_reversion_diff.py  # Behavioral diff test
    test_ma_crossover.py         # Unit tests for datasource
```

### Strategy Class Design

MeanReversionStrategyV2 should extend BaseStrategy (not MeanReversionStrategy) because:
1. The migration changes the core `_process_htf_candle` method -- subclassing the original and overriding creates fragile coupling
2. The V2 needs to copy most private methods from V1 (entries, exits, stops) -- these are the SAME logic
3. Best approach: extract shared logic into a mixin or copy the class and modify only HTF processing

**Recommendation:** Copy `mean_reversion.py` to `mean_reversion_v2.py`, rename class to `MeanReversionStrategyV2`, modify only `_process_htf_candle()` to consume signals from the datasource module. This is the simplest path to zero metric drift because all other logic is identical by construction.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal detection | Custom detection in V2 | `datasources/ma_crossover.produce_signals()` | Whole point of the migration |
| Metric comparison | Manual P&L tracking | `tools/playground_metrics.py` | Already has `collect_data()`, win rate, profit factor |
| Bar-to-dict conversion | New helper | `_bar_to_dict()` from mean_reversion.py | Already handles all indicator fields |
| PDF lookup | New lookup logic | `PDFDocument.get_signal()` | Already handles compound key lookup |
| Atomic signal detection | Reimplementation | `lib.pdf_builder.detect_atomic_signals_on_bar()` | Canonical implementation with all patterns |

## Common Pitfalls

### Pitfall 1: State Dependency in Signal Detection
**What goes wrong:** `detect_atomic_signals_on_bar()` requires `prev_bar` for certain signals (engulfing, supertrend flip, SMA crossover). If V2's prev_bar tracking diverges from V1's, signals differ.
**Why it happens:** The original stores `self._prev_htf_bar` and passes it to `detect_atomic_signals_on_bar`. The datasource also needs prev_bar state.
**How to avoid:** The datasource's `produce_signals()` must accept `prev_bar` as a parameter. The strategy manages prev_bar state and passes it in each call. This keeps the datasource stateless.
**Warning signs:** Different signal counts between V1 and V2 in diff test.

### Pitfall 2: WriteSignal RPC Does Not Exist Yet
**What goes wrong:** Attempting to use WriteSignal RPC for sim mode will fail -- Phase 19 RPCs are planned but not implemented.
**Why it happens:** Phase 19 plan exists but has not been executed. The proto file has no WriteSignal endpoint.
**How to avoid:** Sim mode MUST use direct import (`from datasources.ma_crossover import produce_signals`). Do not attempt RPC-based signal writing. The `__main__` live mode path should be stubbed or deferred.
**Warning signs:** Import errors from non-existent RPC stubs.

### Pitfall 3: prev_bar Timing Between process_candles and produce_signals
**What goes wrong:** In the original, `self._prev_htf_bar` is set AFTER signal detection. If the datasource and strategy update prev_bar at different points, signals diverge.
**Why it happens:** Line 244 in original: `self._prev_htf_bar = bar_dict` is set after calling `detect_atomic_signals_on_bar` but uses the PREVIOUS value during the call.
**How to avoid:** The datasource must receive the bar that was prev_bar BEFORE the current bar. The strategy must pass the correct prev_bar and update its own state after calling produce_signals.

### Pitfall 4: Floating Point Comparison in Diff Tests
**What goes wrong:** Metrics like P&L may have tiny floating-point differences that fail exact comparison.
**Why it happens:** Different code paths can produce slightly different float arithmetic results.
**How to avoid:** Since both V1 and V2 call the same place_order() with the same prices, and the server fills deterministically, the results should be bit-identical. Use exact comparison first; only add tolerances if specific failures demonstrate the need.

### Pitfall 5: Demo Script Doesn't Import V2
**What goes wrong:** The demo_mean_reversion.py launcher needs a V2 variant or a flag to switch strategies.
**How to avoid:** Create `demo_mean_reversion_v2.py` that imports MeanReversionStrategyV2 instead.

## Code Examples

### Behavioral Diff Test Pattern

```python
# tests/test_mean_reversion_diff.py
"""
Behavioral diff test: runs original and migrated strategy on identical
playground inputs and verifies zero metric drift.
"""
import pytest
from unittest.mock import MagicMock, call
from strategies.mean_reversion import MeanReversionStrategy
from strategies.mean_reversion_v2 import MeanReversionStrategyV2


def _run_strategy_collecting_orders(strategy_cls, mock_playground, pdf, candle_sequence):
    """Run a strategy through a candle sequence, collect all place_order calls."""
    strategy = strategy_cls(
        mock_playground, "AAPL", pdf=pdf,
        max_loss_pct=0.02, stop_percentile=0.95,
        total_shares_per_group=100,
    )

    for tick_delta in candle_sequence:
        strategy.on_tick([tick_delta])

    return mock_playground.place_order.call_args_list


def test_v2_produces_same_orders_as_v1(candle_fixture, pdf_fixture):
    """V1 and V2 must produce identical order sequences."""
    pg_v1 = _make_mock_playground()
    pg_v2 = _make_mock_playground()

    orders_v1 = _run_strategy_collecting_orders(
        MeanReversionStrategy, pg_v1, pdf_fixture, candle_fixture,
    )
    orders_v2 = _run_strategy_collecting_orders(
        MeanReversionStrategyV2, pg_v2, pdf_fixture, candle_fixture,
    )

    assert len(orders_v1) == len(orders_v2), (
        f"Order count mismatch: V1={len(orders_v1)}, V2={len(orders_v2)}"
    )

    for i, (v1_call, v2_call) in enumerate(zip(orders_v1, orders_v2)):
        assert v1_call == v2_call, (
            f"Order {i} differs:\n  V1: {v1_call}\n  V2: {v2_call}"
        )
```

### Existing Test Infrastructure

The project has test helpers in `tests/test_mean_reversion_strategy.py`:
- `_make_mock_playground()` -- creates a MagicMock with all required interface fields
- `_make_pdf()` -- creates a minimal PDFDocument with configurable signals
- `_make_htf_bar()` -- creates bar dicts with indicator fields

These can be reused directly for the diff test.

### Key Files for Migration

| File | Role | Lines |
|------|------|-------|
| `strategies/mean_reversion.py` | Original strategy (V1) | ~985 |
| `lib/pdf_builder.py:detect_atomic_signals_on_bar()` | Signal detection (stays in lib) | ~80 |
| `lib/pdf_builder.py:_get, _get_dt` | Helper functions used by `_bar_to_dict` | ~15 |
| `tests/test_mean_reversion_strategy.py` | Existing tests with mock helpers | ~250+ |
| `demos/demo_mean_reversion.py` | Launcher script | ~611 |
| `tools/playground_metrics.py` | P&L, win rate, trade duration | ~200+ |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline signal detection in on_tick | Datasource module + signal consumption | Phase 21 (this phase) | Decouples signal production from strategy |
| Single strategy file | Strategy + datasource separation | Phase 21 (this phase) | Enables reuse, replay, and independent testing |

## Infrastructure Status (Critical)

| Component | Status | Phase | Impact on Phase 21 |
|-----------|--------|-------|---------------------|
| TradeSignal struct (Go) | Complete | 17 | Available |
| InMemorySignalRepository | Complete | 18 | Available for sim |
| ESDBSignalRepository | Complete | 20 | Available for live |
| TickDelta.new_signals field | Complete | 18 | Proto + Go delivery works |
| WriteSignal RPC | NOT IMPLEMENTED | 19 (planned) | Cannot use RPC in sim -- use direct import |
| Python proto stubs (TradeSignalProto) | Complete | 18 | Available in `rpc/playground_pb2.py` |
| datasources/ directory | Does not exist | 21 (this phase) | Must create |

**Key implication:** Since WriteSignal RPC does not exist, the sim-mode architecture MUST use direct Python import of the datasource module. The `__main__` live-mode path for the datasource should be a stub or placeholder noting Phase 19 dependency.

## Open Questions

1. **Signal dict schema for produce_signals() return value**
   - What we know: Must contain signal_key, bar_dict, pdf_entry at minimum
   - What's unclear: Whether to match TradeSignalProto schema (id, name, symbol, timestamp, attributes) or use a richer Python-native dict
   - Recommendation: Use a Python-native dict with all fields needed by `_try_create_group()`. The mapping to TradeSignalProto format can happen in the `__main__` live-mode path when Phase 19 is complete.

2. **Shared code between V1 and V2**
   - What we know: 90%+ of MeanReversionStrategy code is identical between V1 and V2 (entries, exits, stops, groups, EV computation)
   - What's unclear: Whether to use inheritance, composition, or full copy
   - Recommendation: Full copy. The diff test proves equivalence, and after validation the original is deprecated. Copy avoids fragile inheritance and makes the diff test cleaner.

## Sources

### Primary (HIGH confidence)
- `src/clients/python/strategies/mean_reversion.py` -- Full strategy source, signal detection in `_process_htf_candle()` lines 236-261
- `src/clients/python/lib/pdf_builder.py` -- `detect_atomic_signals_on_bar()` canonical implementation
- `src/go/backtester-api/models/signal_repository_interface.go` -- ISignalRepository interface
- `src/go/backtester-api/models/signal_repository_memory.go` -- InMemorySignalRepository (Write/ReadPending)
- `src/go/playground.proto` -- TickDelta.new_signals field (line 365), TradeSignalProto (line 479), NO WriteSignal RPC
- `src/clients/python/tests/test_mean_reversion_strategy.py` -- Existing test helpers

### Secondary (MEDIUM confidence)
- `.planning/phases/19-rpc-endpoints-python-integration/19-01-SUMMARY.md` -- WriteSignal RPC planned but not implemented

## Metadata

**Confidence breakdown:**
- Signal extraction pattern: HIGH -- directly inspected source code, extraction boundary is clean
- Diff test approach: HIGH -- existing test helpers, mock infrastructure, and playground_metrics.py all available
- Infrastructure status: HIGH -- verified proto file and Go source for presence/absence of WriteSignal RPC
- Architecture: HIGH -- sim-mode direct import confirmed viable; TickDelta.new_signals delivery path confirmed in Go

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable -- strategy code changes infrequently)
