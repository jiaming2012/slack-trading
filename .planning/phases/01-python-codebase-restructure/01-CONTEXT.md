# Phase 1: Python Codebase Restructure - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Reorganize the Python client from a flat `src/clients/python/` layout into structured subdirectories (strategies/, engine/, lib/, tools/, demos/, tests/, deprecated/) and consolidate all 6 active strategies to run through a single trading_engine tick loop with a common base class interface.

</domain>

<decisions>
## Implementation Decisions

### Strategy Interface Design
- **D-01:** Full standardization via common base class. All 6 strategies (covered_call, wheel, pdf_wheel, mean_reversion, options_mean_reversion, credit_spread) inherit from a common base class with standardized methods (init, tick, get_signals, is_complete). Chosen for best readability and testability.
- **D-02:** Signal type consolidation is Claude's discretion — unify the versioned signal types (OpenSignal/V2/V3/V4, CloseSignal/V2, RollSignalV1) into the cleanest approach during implementation.

### Trading Engine Adaptation
- **D-03:** Strategy controls tick period. Strategy returns `next_tick_seconds` alongside signals, enabling smart HTF/LTF optimization (e.g., mean_reversion ticks at 3600s when idle, 300s when active). trading_engine respects the strategy's requested tick interval.
- **D-04:** Lifecycle hooks for retraining. Base class defines hooks (on_tick, on_retrain, on_complete) that strategies override. Strategies can receive pre-trained data via constructor (for testing), with production retraining managed through lifecycle hooks.
- **D-05:** Retraining must be configurable/skippable for optimizer compatibility. trading_engine_optimizer.py uses skopt Bayesian optimization and runs many iterations — retraining hooks must be disableable to avoid excessive compute during optimization runs.
- **D-06:** trading_engine_optimizer.py is refactored in this phase to use the new engine and strategy interface. Not deferred.
- **D-07:** Optimizer integration approach (objective function vs standard interface) is Claude's discretion — pick what works cleanest with skopt's gp_minimize pattern.

### Claude's Discretion
- Signal type unification approach (D-02)
- Optimizer integration pattern with skopt (D-07)
- Import structure (relative vs absolute imports, __init__.py design)
- Migration approach (big-bang vs incremental file moves)
- How to handle the 8 deprecated files (move to deprecated/ subdirectory)
- Base class method signatures and return types
- Test fixture strategy for the common test harness

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Files to Understand
- `src/clients/python/trading_engine.py` — Current engine with open/close strategy pattern and tick loop
- `src/clients/python/trading_engine_optimizer.py` — Bayesian optimizer using skopt, imports `objective` from trading_engine
- `src/clients/python/backtester_playground_client_grpc.py` — Twirp RPC client (tick, place_order, fetch_candles)

### Active Strategies (all need refactoring)
- `src/clients/python/options_strategy_basic_v7.py` — Covered call, has own tick loop (~1400 lines)
- `src/clients/python/wheel_strategy.py` — Two-phase wheel, extends OptionsStrategyBasic (~400 lines)
- `src/clients/python/pdf_wheel_strategy.py` — PDF-guided wheel variant (~600 lines)
- `src/clients/python/mean_reversion_strategy.py` — PDF mean-reversion with smart tick optimization (~800 lines)
- `src/clients/python/options_mean_reversion_strategy.py` — Options variant of mean-reversion (~800 lines)
- `src/clients/python/credit_spread_strategy.py` — PDF credit spread selling (~800 lines)

### Strategy Base Classes (existing, to be replaced/evolved)
- `src/clients/python/base_open_strategy.py` — Abstract base for open strategies (207 lines)
- `src/clients/python/base_open_strategy_v2.py` — Simpler variant (127 lines)

### Demo Entry Points (all need refactoring)
- `src/clients/python/demo_covered_call.py`
- `src/clients/python/demo_wheel_strategy.py`
- `src/clients/python/demo_pdf_wheel_strategy.py`
- `src/clients/python/demo_mean_reversion.py`
- `src/clients/python/demo_options_mean_reversion.py`
- `src/clients/python/demo_credit_spread.py`

### Shared Libraries (move to lib/)
- `src/clients/python/pdf_builder.py` — PDF generation engine (~1000 lines)
- `src/clients/python/pdf_types.py` — PDF data structures (~400 lines)
- `src/clients/python/deviation_levels.py` — Sigma deviation bands (~200 lines)
- `src/clients/python/partial_exit_manager.py` — Exit tier computation (~150 lines)
- `src/clients/python/risk_management.py` — Kelly criterion sizing (~150 lines)
- `src/clients/python/return_models.py` — Distribution models (~300 lines)
- `src/clients/python/candlestick_patterns.py` — Pattern detection (~155 lines)

### Type Definitions (merge into engine/types.py)
- `src/clients/python/playground_types.py` — Enums (19 lines)
- `src/clients/python/trading_engine_types.py` — Signal types (~40 lines)

### Tests (all must pass after restructure)
- `src/clients/python/test_demo_covered_call.py`
- `src/clients/python/test_credit_spread_strategy.py`
- `src/clients/python/test_mean_reversion_strategy.py`
- `src/clients/python/test_options_mean_reversion_strategy.py`
- `src/clients/python/test_pdf_wheel_strategy.py`
- `src/clients/python/test_deviation_levels.py`
- `src/clients/python/test_kelly_sizing.py`
- `src/clients/python/test_partial_exit_manager.py`
- `src/clients/python/test_pdf_builder.py`
- `src/clients/python/test_return_models.py`
- `src/clients/python/test_mean_reversion_report.py`

### Deprecated Files (move to deprecated/)
- `src/clients/python/simple_open_strategy_v4.py`
- `src/clients/python/simple_stack_open_strategy_v2.py`
- `src/clients/python/simple_close_strategy.py`
- `src/clients/python/simple_stack_close_strategy.py`
- `src/clients/python/stack_close_strategy_psar.py`
- `src/clients/python/generate_signals.py`
- `src/clients/python/plot_playground.py`
- `src/clients/python/plot_candlestick.py`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `base_open_strategy.py` and `base_open_strategy_v2.py` — existing abstract base patterns that can inform the new unified base class design
- `rpc_profiler.py` — RPC timing profiler, already wraps client calls (move to engine/)
- `playground_types.py` — enums (RepositorySource, OrderSide, LiveAccountType) used everywhere

### Established Patterns
- All strategies use `BacktesterPlaygroundClient` for RPC communication
- Newer strategies (mean_reversion, credit_spread) use `tick_delta` pattern with `flush_new_state()` for candle updates
- Older strategies (covered_call) use `run_options_strategy()` with embedded loop
- PDF-based strategies share common initialization: build PDF → create playground → run loop
- Mean reversion uses smart tick: HTF period (3600s) when idle, LTF period (300s) when active

### Integration Points
- `trading_engine.objective()` function — imported by trading_engine_optimizer.py for skopt
- `BacktesterPlaygroundClient.tick()` and `place_order()` — the actual RPC calls that all strategies use
- `rpc/playground_twirp.py` and `rpc/playground_pb2.py` — generated stubs, stay in rpc/
- All demo scripts are CLI entry points with argparse — need to work from new paths

</code_context>

<specifics>
## Specific Ideas

- Lifecycle hooks must support a "pre-trained data" mode where PDF/model is passed via constructor (for testing) and retraining hooks are skipped
- Optimizer runs many iterations — retraining hooks must be configurable to reduce compute cost during parameter sweeps
- The test_demo_covered_call.py is a regression test that pins reference metrics — it MUST pass with only import path changes

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-python-codebase-restructure*
*Context gathered: 2026-03-26*
