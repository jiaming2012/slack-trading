# Phase 1: Python Codebase Restructure - Research

**Researched:** 2026-03-26
**Domain:** Python project restructuring, strategy pattern consolidation, import refactoring
**Confidence:** HIGH

## Summary

This phase restructures ~25,000 lines of Python code from a flat `src/clients/python/` directory into a modular package layout (strategies/, engine/, lib/, tools/, demos/, tests/, deprecated/) and consolidates 6 active trading strategies behind a common base class + single engine tick loop.

The codebase currently has two distinct strategy architecture generations: (1) the older `BaseOpenStrategy`/`BaseOpenStrategyV2` hierarchy used by `OptionsStrategyBasic` and the trading_engine.py `run_strategy()` function, which has separate open/close strategy concepts, and (2) the newer standalone strategies (`MeanReversionStrategy`, `CreditSpreadStrategy`, `OptionsMeanReversionStrategy`) that each have their own `process_candles()` method and module-level `run_*()` loop functions. Unifying these requires a new base class that accommodates both patterns while preserving existing behavior.

**Primary recommendation:** Use absolute imports from a package root (e.g., `from strategies.mean_reversion_strategy import MeanReversionStrategy`), add `__init__.py` files to each subdirectory, and execute the restructure in two waves: (1) move files + fix imports, (2) consolidate strategy interfaces and engine.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Full standardization via common base class. All 6 strategies (covered_call, wheel, pdf_wheel, mean_reversion, options_mean_reversion, credit_spread) inherit from a common base class with standardized methods (init, tick, get_signals, is_complete). Chosen for best readability and testability.
- **D-03:** Strategy controls tick period. Strategy returns `next_tick_seconds` alongside signals, enabling smart HTF/LTF optimization (e.g., mean_reversion ticks at 3600s when idle, 300s when active). trading_engine respects the strategy's requested tick interval.
- **D-04:** Lifecycle hooks for retraining. Base class defines hooks (on_tick, on_retrain, on_complete) that strategies override. Strategies can receive pre-trained data via constructor (for testing), with production retraining managed through lifecycle hooks.
- **D-05:** Retraining must be configurable/skippable for optimizer compatibility. trading_engine_optimizer.py uses skopt Bayesian optimization and runs many iterations -- retraining hooks must be disableable to avoid excessive compute during optimization runs.
- **D-06:** trading_engine_optimizer.py is refactored in this phase to use the new engine and strategy interface. Not deferred.

### Claude's Discretion
- Signal type unification approach (D-02)
- Optimizer integration pattern with skopt (D-07)
- Import structure (relative vs absolute imports, __init__.py design)
- Migration approach (big-bang vs incremental file moves)
- How to handle the 8 deprecated files (move to deprecated/ subdirectory)
- Base class method signatures and return types
- Test fixture strategy for the common test harness

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIR-01 | Python client reorganized into strategies/, engine/, lib/, tools/, demos/, tests/, deprecated/ subdirectories | Architecture Patterns section defines complete directory structure |
| DIR-02 | Strategy files moved to strategies/ | File mapping table in Architecture Patterns |
| DIR-03 | Core runtime files moved to engine/ | File mapping table identifies: trading_engine, client, types, rpc_profiler |
| DIR-04 | Shared libraries moved to lib/ | File mapping table identifies all 7 lib files |
| DIR-05 | CLI tools moved to tools/ | File mapping table identifies: build_pdf, plot_option_candlestick, mean_reversion_report, credit_spread_visualizations |
| DIR-06 | Demo entry points moved to demos/ | File mapping table identifies all 6 demo files |
| DIR-07 | Deprecated files moved to deprecated/ | File mapping table identifies all 8 deprecated files + 3 additional candidates |
| DIR-08 | All imports updated across codebase to reflect new paths | Import strategy and pitfalls documented |
| CONS-01 | All 6 strategies implement common strategy interface | Base class design in Architecture Patterns |
| CONS-02 | trading_engine.py refactored as single tick loop orchestrator | Engine consolidation pattern documented |
| CONS-03 | Each demo script uses trading_engine.run_strategy() | Demo refactoring pattern documented |
| CONS-04 | All existing strategy tests pass after consolidation | Test strategy and validation architecture documented |
</phase_requirements>

## Standard Stack

No new libraries are required. This phase is purely a restructuring of existing Python code.

### Core (already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Python | 3.10 | Runtime | Conda grodt env |
| pytest | (installed) | Test runner | Already used for all strategy tests |
| loguru | 0.7.3 | Structured logging | Already used throughout all strategies |
| scikit-optimize | 0.10.2 | Bayesian optimization | Used by trading_engine_optimizer.py |

### No New Dependencies
This phase does not introduce any new packages. All work is restructuring existing code.

## Architecture Patterns

### Recommended Project Structure
```
src/clients/python/
├── __init__.py              # Package root (empty or minimal)
├── strategies/
│   ├── __init__.py
│   ├── base_strategy.py           # NEW: common base class
│   ├── covered_call.py            # FROM: options_strategy_basic_v7.py
│   ├── wheel.py                   # FROM: wheel_strategy.py
│   ├── pdf_wheel.py               # FROM: pdf_wheel_strategy.py
│   ├── mean_reversion.py          # FROM: mean_reversion_strategy.py
│   ├── options_mean_reversion.py  # FROM: options_mean_reversion_strategy.py
│   └── credit_spread.py          # FROM: credit_spread_strategy.py
├── engine/
│   ├── __init__.py
│   ├── trading_engine.py          # FROM: trading_engine.py (refactored)
│   ├── trading_engine_optimizer.py # FROM: trading_engine_optimizer.py (refactored)
│   ├── client.py                  # FROM: backtester_playground_client_grpc.py
│   ├── types.py                   # FROM: trading_engine_types.py + playground_types.py (merged)
│   └── rpc_profiler.py            # FROM: rpc_profiler.py
├── lib/
│   ├── __init__.py
│   ├── pdf_builder.py             # FROM: pdf_builder.py
│   ├── pdf_types.py               # FROM: pdf_types.py
│   ├── deviation_levels.py        # FROM: deviation_levels.py
│   ├── partial_exit_manager.py    # FROM: partial_exit_manager.py
│   ├── risk_management.py         # FROM: risk_management.py
│   ├── return_models.py           # FROM: return_models.py
│   └── candlestick_patterns.py    # FROM: candlestick_patterns.py
├── tools/
│   ├── __init__.py
│   ├── build_pdf_from_polygon.py  # FROM: build_pdf_from_polygon.py
│   ├── plot_option_candlestick.py # FROM: plot_option_candlestick.py
│   ├── mean_reversion_report.py   # FROM: mean_reversion_report.py
│   └── credit_spread_visualizations.py # FROM: credit_spread_visualizations.py
├── demos/
│   ├── __init__.py
│   ├── demo_covered_call.py       # FROM: demo_covered_call.py
│   ├── demo_wheel_strategy.py     # FROM: demo_wheel_strategy.py
│   ├── demo_pdf_wheel_strategy.py # FROM: demo_pdf_wheel_strategy.py
│   ├── demo_mean_reversion.py     # FROM: demo_mean_reversion.py
│   ├── demo_options_mean_reversion.py # FROM: demo_options_mean_reversion.py
│   └── demo_credit_spread.py      # FROM: demo_credit_spread.py
├── tests/
│   ├── __init__.py
│   ├── test_demo_covered_call.py
│   ├── test_credit_spread_strategy.py
│   ├── test_mean_reversion_strategy.py
│   ├── test_options_mean_reversion_strategy.py
│   ├── test_pdf_wheel_strategy.py
│   ├── test_deviation_levels.py
│   ├── test_kelly_sizing.py
│   ├── test_partial_exit_manager.py
│   ├── test_pdf_builder.py
│   ├── test_return_models.py
│   └── test_mean_reversion_report.py
├── deprecated/
│   ├── __init__.py
│   ├── base_open_strategy.py
│   ├── base_open_strategy_v2.py
│   ├── simple_open_strategy_v4.py
│   ├── simple_stack_open_strategy_v2.py
│   ├── simple_close_strategy.py
│   ├── simple_stack_close_strategy.py
│   ├── stack_close_strategy_psar.py
│   ├── generate_signals.py
│   ├── plot_playground.py
│   └── plot_candlestick.py
├── rpc/                           # STAYS IN PLACE (generated stubs)
│   ├── playground_pb2.py
│   └── playground_twirp.py
├── utils.py                       # STAYS (small utility, used by trading_engine)
├── playground_metrics.py          # STAYS or moves to tools/ (used by taskfile)
└── plot_best_fit_distribution.py  # MOVES to tools/ or deprecated/
```

### Pattern 1: Common Strategy Base Class

**What:** Abstract base class all 6 strategies inherit from, providing a uniform interface for trading_engine.

**Current state analysis:** There are two distinct strategy interface patterns:

1. **Options-based strategies** (covered_call, wheel, pdf_wheel): Use `tick(tick_delta) -> (open_signals, close_signals)` pattern. The strategy receives tick deltas and returns signal objects. The `run_*()` loop function handles order placement based on signals.

2. **PDF-based strategies** (mean_reversion, options_mean_reversion, credit_spread): Use `process_candles(new_candles) -> None` pattern. The strategy internally processes candles AND places orders directly via `self.playground.place_order()`. No signals returned.

**Design for the new base class:**

```python
from abc import ABC, abstractmethod
from typing import Optional
from engine.client import BacktesterPlaygroundClient

class BaseStrategy(ABC):
    """Common interface for all trading strategies."""

    def __init__(self, playground: BacktesterPlaygroundClient, symbol: str, logger=None, **kwargs):
        self.playground = playground
        self.symbol = symbol
        self.logger = logger

    @abstractmethod
    def on_tick(self, tick_deltas) -> None:
        """Process one tick's worth of data. Strategy handles its own
        signal generation and order placement internally."""
        pass

    @abstractmethod
    def get_next_tick_seconds(self) -> int:
        """Return the number of seconds for the next tick.
        Enables smart HTF/LTF optimization (D-03)."""
        pass

    @abstractmethod
    def should_fetch_account(self) -> bool:
        """Whether the engine should fetch account state on this tick."""
        pass

    def on_retrain(self) -> None:
        """Optional lifecycle hook for periodic retraining (D-04).
        Default: no-op. Override in strategies that need retraining."""
        pass

    def on_complete(self) -> None:
        """Optional lifecycle hook called when simulation completes.
        Default: no-op. Override for end-of-sim cleanup."""
        pass

    def is_complete(self) -> bool:
        """Check if the strategy/simulation is done."""
        return self.playground.is_backtest_complete()

    @classmethod
    def get_repositories(cls, symbol: str, start_date=None, end_date=None):
        """Return repository configuration for playground creation.
        Override per strategy."""
        raise NotImplementedError
```

**Key design decisions:**
- The `on_tick()` method is the single entry point. Both strategy families adapt to it:
  - PDF strategies: their existing `process_candles()` is called inside `on_tick()`
  - Options strategies: their existing `tick(tick_delta)` is called inside `on_tick()`, and order placement logic moves into the strategy
- Strategies place their own orders (the PDF pattern wins over the signal-return pattern, because the signal-return pattern requires the engine to know strategy-specific order placement logic)
- `get_next_tick_seconds()` implements D-03 (strategy controls tick period)
- `on_retrain()` is a no-op by default (D-04, D-05)

### Pattern 2: Consolidated Trading Engine

**What:** `trading_engine.py` becomes a thin orchestrator that runs any strategy through a single loop.

```python
def run_strategy(
    strategy: BaseStrategy,
    playground: BacktesterPlaygroundClient,
    logger,
    enable_retraining: bool = True,
    max_iterations: int = 500_000,
) -> BaseStrategy:
    """Universal tick loop for any strategy."""
    iteration = 0

    while not strategy.is_complete():
        iteration += 1
        if iteration > max_iterations:
            logger.warning(f"Max iterations ({max_iterations}) reached")
            break

        tick_deltas = playground.flush_new_state_buffer()
        strategy.on_tick(tick_deltas)

        if enable_retraining:
            strategy.on_retrain()

        tick_seconds = strategy.get_next_tick_seconds()
        fetch_account = strategy.should_fetch_account()
        playground.tick(tick_seconds, fetch_account=fetch_account)

    strategy.on_complete()
    return strategy
```

**Key insight:** The current `run_mean_reversion()`, `run_options_strategy()`, `run_credit_spread_strategy()`, etc. each implement essentially this same loop with strategy-specific customizations. Those customizations move into the strategy class methods.

### Pattern 3: Strategy Adaptation (Per-Strategy)

Each strategy wraps its existing logic:

**MeanReversionStrategy adaptation:**
```python
class MeanReversionStrategy(BaseStrategy):
    def on_tick(self, tick_deltas):
        for td in tick_deltas:
            new_candles = td.new_candles if hasattr(td, "new_candles") else []
            self.process_candles(new_candles)  # existing method unchanged

    def get_next_tick_seconds(self):
        has_active = any(g.status in ("pending", "active") for g in self.trade_groups)
        return self.playground.ltf_seconds if has_active else self.playground.htf_seconds

    def should_fetch_account(self):
        return any(g.status in ("pending", "active") for g in self.trade_groups)

    def on_complete(self):
        self.close_all_active_groups()
        self.log_summary()
```

**OptionsStrategyBasic (covered_call) adaptation:** This is the most complex because it currently returns signals and the loop handles order placement. The order placement logic from `run_options_strategy()` moves into `on_tick()`:
```python
class CoveredCallStrategy(BaseStrategy):
    def on_tick(self, tick_deltas):
        for td in tick_deltas:
            open_signals, close_signals = self._inner_tick(td)  # existing tick() method
            self._place_close_orders(close_signals)
            self._place_open_orders(open_signals)

    def get_next_tick_seconds(self):
        return self.playground.ltf_seconds  # covered call always ticks at LTF

    def should_fetch_account(self):
        return True
```

### Pattern 4: Demo Script Refactoring

Each demo script becomes a thin wrapper:
```python
# demos/demo_mean_reversion.py
from engine.trading_engine import run_strategy
from strategies.mean_reversion import MeanReversionStrategy

def main():
    # ... argparse, playground creation (stays the same) ...
    strategy = MeanReversionStrategy(playground, symbol, logger, pdf=pdf, ...)
    run_strategy(strategy, playground, logger)
    # ... print results ...
```

### Anti-Patterns to Avoid
- **Circular imports:** `engine/` imports from `strategies/` -- never let strategies import from engine internals beyond the base class and client
- **Giant __init__.py re-exports:** Keep `__init__.py` files minimal. Only re-export the main class per package. Do NOT re-export everything.
- **Relative imports across packages:** Use absolute imports from the `src/clients/python/` root throughout. All files reference `from strategies.X import Y`, `from engine.client import Z`, etc.
- **Breaking the test subprocess pattern:** `test_demo_covered_call.py` runs `demo_covered_call.py` as a subprocess. After restructure it must run `python -m demos.demo_covered_call` or the path must be adjusted.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Import path updating | Manual find/replace | AST-aware search with grep + systematic replacement | Bare string replacement breaks strings that happen to match import names |
| Package discovery | Custom sys.path manipulation | Proper `__init__.py` + running from package root | sys.path hacks are fragile across environments |

**Key insight:** The restructure is a refactoring problem, not a library problem. No custom solutions needed -- just careful file moves and import updates.

## Common Pitfalls

### Pitfall 1: Import Resolution After Move
**What goes wrong:** After moving files to subdirectories, Python cannot find modules because `sys.path` only includes the old flat directory.
**Why it happens:** Currently all Python files are in one flat directory, so bare `import trading_engine` works. After restructure, Python needs to find `engine.trading_engine`.
**How to avoid:** Add `__init__.py` to each subdirectory. Run all scripts with `python -m demos.demo_covered_call` from the `src/clients/python/` root, or ensure `src/clients/python/` is on `sys.path`.
**Warning signs:** `ModuleNotFoundError` on first test run after move.

### Pitfall 2: Circular Import Between Strategies and Engine
**What goes wrong:** `engine/trading_engine.py` imports `BaseStrategy` from `strategies/base_strategy.py`, and strategies import from `engine/types.py` or `engine/client.py` -- this is fine. But if strategies import from `engine/trading_engine.py` (e.g., for the `objective` function), circularity can arise.
**Why it happens:** The current `trading_engine_optimizer.py` imports `objective` from `trading_engine.py`. After refactoring, this dependency path must remain acyclic.
**How to avoid:** Keep `BaseStrategy` in `strategies/base_strategy.py` (not in engine/). Keep the `objective` function in engine/ and have it accept a strategy factory rather than building strategies internally. The optimizer imports from engine, strategies import from engine -- no strategy imports from trading_engine.py.
**Warning signs:** `ImportError: cannot import name...` or `AttributeError: partially initialized module`.

### Pitfall 3: test_demo_covered_call.py Subprocess Path
**What goes wrong:** This test runs `demo_covered_call.py` as a subprocess with `cwd=script_dir`. After the file moves to `demos/`, the test must update its subprocess command and cwd.
**Why it happens:** The test uses `os.path.dirname(os.path.abspath(__file__))` to find the demo script. After the test moves to `tests/` and the demo to `demos/`, the relative path breaks.
**How to avoid:** Update the test to use `python -m demos.demo_covered_call` with `cwd` set to the package root (`src/clients/python/`), or compute the path relative to the package root.
**Warning signs:** Test cannot find the demo script.

### Pitfall 4: OptionsStrategyBasic Inheritance Chain
**What goes wrong:** `WheelStrategy(OptionsStrategyBasic)` and `PDFWheelStrategy(WheelStrategy)` form an inheritance chain. Adding `BaseStrategy` as a parent of `OptionsStrategyBasic` while it currently inherits from `BaseOpenStrategyV2` requires careful MRO management.
**Why it happens:** `OptionsStrategyBasic` currently extends `BaseOpenStrategyV2(ABC)` which has its own `tick()` and `is_complete()` methods. The new `BaseStrategy` also defines these. Python's MRO must resolve cleanly.
**How to avoid:** Have `OptionsStrategyBasic` inherit from `BaseStrategy` directly (replacing `BaseOpenStrategyV2`). Copy any needed methods from `BaseOpenStrategyV2` into `OptionsStrategyBasic`. Then `WheelStrategy(OptionsStrategyBasic)` and `PDFWheelStrategy(WheelStrategy)` continue to work.
**Warning signs:** `TypeError: Cannot create a consistent method resolution order (MRO)`.

### Pitfall 5: Signal Type Fragmentation
**What goes wrong:** There are 7 signal types across the codebase: `OpenSignal`, `OpenSignalV2`, `OpenSignalV3`, `OpenSignalV4`, `CloseSignalV2`, `RollSignalV1`, `PutSellSignal`. Premature unification could break strategy-specific logic.
**Why it happens:** Each strategy generation added new signal types with different fields.
**How to avoid:** Move all signal types to `engine/types.py` as-is first. Unification (D-02) should be done after file moves are working and tests pass. Signal unification is a separate concern from directory restructure.
**Warning signs:** Tests fail with field-not-found errors on signal objects.

### Pitfall 6: Taskfile Path References
**What goes wrong:** `taskfile.yml` references `src/clients/python/trading_engine.py` and `src/clients/python/playground_metrics.py` with specific Python interpreter paths. These break after file moves.
**Why it happens:** Task definitions hardcode paths to Python scripts.
**How to avoid:** Update taskfile.yml entries to point to new paths. For scripts that move into subdirectories, use `python -m` invocation or update the path.
**Warning signs:** `task sim` or `task metrics:*` commands fail.

### Pitfall 7: rpc/ Proto Generation Path
**What goes wrong:** `task gen:proto` in taskfile.yml moves generated `playground_pb2.py` and `playground_twirp.py` into `src/clients/python/rpc/`. This path must remain stable.
**Why it happens:** Proto generation is a separate concern -- the `rpc/` directory stays at the same location.
**How to avoid:** Do NOT move the `rpc/` directory. Keep it at `src/clients/python/rpc/`. All imports of `from rpc.playground_pb2 import ...` continue to work because `rpc/` is a subdirectory of the package root.
**Warning signs:** Proto regeneration puts files in wrong location.

### Pitfall 8: utils.py and playground_metrics.py Placement
**What goes wrong:** `utils.py` contains `get_timespan_unit()` and Polygon utility classes. It's imported by `trading_engine.py`. `playground_metrics.py` is referenced in taskfile.yml tasks.
**Why it happens:** These files don't fit cleanly into one category.
**How to avoid:** Keep `utils.py` in the package root or move to `lib/`. `playground_metrics.py` is a tool (CLI entry point) -- move to `tools/` and update taskfile.
**Warning signs:** Import errors for `utils` after moves.

## Code Examples

### Complete File Inventory (48 .py files in src/clients/python/)

**Strategies (6 files, move to strategies/):**
| Current Name | New Location | Lines | Notes |
|---|---|---|---|
| options_strategy_basic_v7.py | strategies/covered_call.py | 1650 | Inherits BaseOpenStrategyV2, has own run loop |
| wheel_strategy.py | strategies/wheel.py | ~400 | Inherits OptionsStrategyBasic |
| pdf_wheel_strategy.py | strategies/pdf_wheel.py | 942 | Inherits WheelStrategy |
| mean_reversion_strategy.py | strategies/mean_reversion.py | 861 | Standalone class, process_candles pattern |
| options_mean_reversion_strategy.py | strategies/options_mean_reversion.py | 1109 | Standalone class, process_candles pattern |
| credit_spread_strategy.py | strategies/credit_spread.py | 1964 | Standalone class, process_candles pattern |

**Engine (5 files, move to engine/):**
| Current Name | New Location | Lines | Notes |
|---|---|---|---|
| trading_engine.py | engine/trading_engine.py | 717 | Contains run_strategy() + objective() |
| trading_engine_optimizer.py | engine/trading_engine_optimizer.py | 175 | Imports objective from trading_engine |
| backtester_playground_client_grpc.py | engine/client.py | 735 | Core RPC client |
| trading_engine_types.py | engine/types.py | 40 | Signal types (merge with playground_types) |
| rpc_profiler.py | engine/rpc_profiler.py | ~100 | RPC timing profiler |

**Lib (7 files, move to lib/):**
| Current Name | New Location | Lines |
|---|---|---|
| pdf_builder.py | lib/pdf_builder.py | ~340 |
| pdf_types.py | lib/pdf_types.py | ~260 |
| deviation_levels.py | lib/deviation_levels.py | ~260 |
| partial_exit_manager.py | lib/partial_exit_manager.py | ~150 |
| risk_management.py | lib/risk_management.py | ~130 |
| return_models.py | lib/return_models.py | ~260 |
| candlestick_patterns.py | lib/candlestick_patterns.py | ~155 |

**Tools (4-5 files, move to tools/):**
| Current Name | New Location | Lines |
|---|---|---|
| build_pdf_from_polygon.py | tools/build_pdf_from_polygon.py | ~260 |
| plot_option_candlestick.py | tools/plot_option_candlestick.py | 935 |
| mean_reversion_report.py | tools/mean_reversion_report.py | 495 |
| credit_spread_visualizations.py | tools/credit_spread_visualizations.py | ~440 |
| playground_metrics.py | tools/playground_metrics.py | 605 |

**Demos (6 files, move to demos/):**
All demo_*.py files move as-is.

**Tests (11 files, move to tests/):**
All test_*.py files move as-is.

**Deprecated (8+ files, move to deprecated/):**
| File | Lines | Reason |
|---|---|---|
| simple_open_strategy_v4.py | ~310 | Superseded by new strategies |
| simple_stack_open_strategy_v2.py | ~320 | Superseded |
| simple_close_strategy.py | ~120 | Superseded |
| simple_stack_close_strategy.py | ~180 | Superseded |
| stack_close_strategy_psar.py | ~180 | Superseded |
| generate_signals.py | ~360 | Superseded |
| plot_playground.py | ~120 | Superseded |
| plot_candlestick.py | ~45 | Superseded |
| base_open_strategy.py | 207 | Replaced by new BaseStrategy |
| base_open_strategy_v2.py | 127 | Replaced by new BaseStrategy |
| renko.py | 15 | Unused stub |

**Stay in place:**
| File | Reason |
|---|---|
| rpc/ directory | Generated stubs, protoc output path |
| utils.py | Small utility, keep at root or move to lib/ |
| playground_types.py | Merged into engine/types.py (original deleted) |

### Import Dependency Map (Critical for ordering)

The following shows which modules import from which, determining the move order:

```
engine/types.py        <- strategies/*, engine/trading_engine.py, engine/client.py
engine/client.py       <- strategies/*, demos/*, engine/trading_engine.py
lib/pdf_builder.py     <- strategies/mean_reversion.py, credit_spread.py, options_mean_reversion.py
lib/pdf_types.py       <- lib/pdf_builder.py, strategies/*, tests/*
lib/deviation_levels.py <- strategies/mean_reversion.py, credit_spread.py
lib/partial_exit_manager.py <- strategies/mean_reversion.py
rpc/playground_pb2.py  <- engine/client.py, strategies/covered_call.py, strategies/wheel.py
strategies/covered_call.py <- strategies/wheel.py, strategies/pdf_wheel.py
strategies/base_strategy.py <- all strategies, engine/trading_engine.py
engine/trading_engine.py <- engine/trading_engine_optimizer.py, demos/*
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| Flat file layout | Package-based layout | This phase | Enables module-level `python -m` invocation |
| Per-strategy run loops | Single engine tick loop | This phase | CONS-02, CONS-03 |
| BaseOpenStrategy/V2 hierarchy | Single BaseStrategy | This phase | CONS-01 |
| Separate open/close strategy pattern | Strategy handles own orders | This phase | Simplifies engine |

**Deprecated/outdated:**
- `BaseOpenStrategy` / `BaseOpenStrategyV2`: Replaced by new `BaseStrategy` base class
- `SimpleCloseStrategy` / `SimpleStackCloseStrategy` / `StackCloseStrategyPsar`: Close strategy concept absorbed into strategy classes
- Per-strategy `run_*()` module-level functions: Replaced by `engine.trading_engine.run_strategy()`

## Open Questions

1. **playground_types.py enums vs engine/client.py enums**
   - What we know: `playground_types.py` defines `RepositorySource`, `OrderSide`, `LiveAccountType`. But `backtester_playground_client_grpc.py` ALSO defines `OrderSide`, `RepositorySource` as its own classes/enums. Multiple strategies import from both locations.
   - What's unclear: Whether these are identical or have diverged
   - Recommendation: Audit both definitions during implementation. Merge into `engine/types.py` with the canonical versions. Update all imports.

2. **Optimizer refactoring depth**
   - What we know: `trading_engine_optimizer.py` currently imports `objective` from `trading_engine.py`. The `objective()` function reads env vars, creates a playground, builds strategies, calls `run_strategy()`, and returns (equity, meta). It is tightly coupled to the old open/close strategy pattern.
   - What's unclear: How much of `objective()` needs rewriting vs. wrapping
   - Recommendation: Refactor `objective()` to accept a strategy factory function. The optimizer passes hyperparameters to the factory, which creates the strategy. This keeps optimizer and strategy decoupled.

3. **How demos/test_demo_covered_call.py should invoke the demo**
   - What we know: It runs `subprocess.run([python, demo_script, ...], cwd=script_dir)`. After restructure, script and test are in different directories.
   - What's unclear: Whether to use `python -m demos.demo_covered_call` or absolute path
   - Recommendation: Use `python -m demos.demo_covered_call` with `cwd=src/clients/python/` (package root). This is the most portable approach.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | pytest (installed in grodt conda env) |
| Config file | none -- no pytest.ini or pyproject.toml |
| Quick run command | `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -x -q` |
| Full suite command | `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIR-01 | Subdirectories exist with __init__.py | smoke | `python -c "import strategies; import engine; import lib"` | Wave 0 |
| DIR-08 | All imports resolve | smoke | `python -m pytest tests/ --co -q` (collection = import check) | Existing tests |
| CONS-01 | All 6 strategies inherit BaseStrategy | unit | `python -c "from strategies.mean_reversion import MeanReversionStrategy; from strategies.base_strategy import BaseStrategy; assert issubclass(MeanReversionStrategy, BaseStrategy)"` | Wave 0 |
| CONS-02 | Engine runs any strategy | unit | existing strategy tests with engine.run_strategy() | Existing (modified) |
| CONS-03 | Demo scripts use run_strategy() | smoke | `python -m demos.demo_covered_call --help` (no crash) | Existing demos |
| CONS-04 | All existing tests pass | integration | `/Users/jamal/miniconda3/envs/grodt/bin/python -m pytest tests/ -v` | Existing tests |

### Sampling Rate
- **Per task commit:** `python -m pytest tests/ -x -q` (from src/clients/python/)
- **Per wave merge:** `python -m pytest tests/ -v` (full suite)
- **Phase gate:** Full suite green + `python -m demos.demo_covered_call --help` succeeds

### Wave 0 Gaps
- [ ] `pytest.ini` or `pyproject.toml` -- configure test discovery for `tests/` subdirectory
- [ ] `__init__.py` in each subdirectory -- required for imports to work
- [ ] Verify `python -m pytest tests/ --co` collects all existing tests after moves

## Project Constraints (from CLAUDE.md)

- **Python env:** grodt conda env (`/Users/jamal/miniconda3/envs/grodt/bin/python`), numpy pinned to 1.26.4
- **Python naming:** `snake_case.py` for source, `test_*.py` for tests
- **No linter/formatter config:** No flake8, pyproject.toml, or formatter config present
- **Taskfile:** Build/test/deploy tasks in `taskfile.yml` -- paths must be updated after restructure
- **Proto generation:** `task gen:proto` outputs to `src/clients/python/rpc/` -- this path must not change
- **Git workflow:** Feature branches `claude/<descriptive-name>`, short imperative commit messages
- **gh alias:** User's shell aliases `gh` to `git checkout`. Use `/usr/local/bin/gh` or `command gh`

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection of all 48 .py files in src/clients/python/
- All import patterns verified by grep across the codebase
- Strategy class hierarchies verified by reading source files
- Test patterns verified by pytest --co collection

### Secondary (MEDIUM confidence)
- Python packaging best practices (abc module, __init__.py patterns) -- well-established Python stdlib patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, all existing code
- Architecture: HIGH -- based on direct reading of all 6 strategies, both base classes, trading engine, and optimizer
- Pitfalls: HIGH -- derived from actual import dependency analysis and test inspection
- Strategy interface design: MEDIUM -- the base class design is a synthesis of two patterns; exact method signatures may need adjustment during implementation

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable -- no external dependencies changing)
