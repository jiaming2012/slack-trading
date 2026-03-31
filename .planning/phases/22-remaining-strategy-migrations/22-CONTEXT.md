# Phase 22: Remaining Strategy Migrations - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate all remaining strategies (5) to TradeSignal framework using the proven Phase 21 pattern. Add on_signal() hook to BaseStrategy. Move all originals to deprecated/. Verify each with behavioral diff tests. Update trading engine to use only migrated strategies.

Requirements: MIG-01, MIG-02

</domain>

<decisions>
## Implementation Decisions

### D-01: Apply Phase 21 pattern to all remaining strategies
- Same approach: extract datasource → create V2 → diff test → move original to deprecated/
- Full copy approach (not inheritance) for each V2
- Strategies to migrate: CoveredCall, CreditSpread, Wheel, PDFWheel, OptionsMeanReversion

### D-02: Add on_signal() hook to BaseStrategy
- Add `on_signal(signal)` abstract or default method to BaseStrategy ABC
- V2 strategies override `on_signal()` to handle their specific signal types
- Trading engine calls `on_signal()` for each signal in TickDelta.new_signals
- This standardizes signal consumption across all strategies

### D-03: Move originals to deprecated/
- After each V2 passes its diff test, move the original .py to `src/clients/python/deprecated/`
- Also move MeanReversionStrategy V1 (from Phase 21) to deprecated/
- Update any imports that reference the old locations

### D-04: Update trading engine
- `run_engine()` in trading_engine.py should call `strategy.on_signal(signal)` for each new signal
- End-to-end test: trading engine runs with only V2 strategies (no legacy signal paths)

### Claude's Discretion
- Migration order for the 5 strategies
- Each datasource module design (what signals each strategy needs)
- Whether on_signal() is abstract (required) or has a default no-op (optional)
- Demo script updates for each migrated strategy
- How to handle PDFWheel which extends Wheel (migration dependency)

</decisions>

<canonical_refs>
## Canonical References

### Phase 21 Pattern (proven)
- `src/clients/python/datasources/ma_crossover.py` — datasource extraction pattern
- `src/clients/python/strategies/mean_reversion_v2.py` — V2 strategy pattern
- `src/clients/python/tests/test_mean_reversion_diff.py` — diff test pattern
- `src/clients/python/tests/test_ma_crossover.py` — datasource unit test pattern

### Strategies to Migrate
- `src/clients/python/strategies/covered_call.py` — OptionsStrategyBasic
- `src/clients/python/strategies/credit_spread.py` — CreditSpreadStrategy
- `src/clients/python/strategies/wheel.py` — WheelStrategy
- `src/clients/python/strategies/pdf_wheel.py` — PDFWheelStrategy (extends Wheel)
- `src/clients/python/strategies/options_mean_reversion.py` — OptionsMeanReversionStrategy
- `src/clients/python/strategies/base_strategy.py` — BaseStrategy ABC

### Trading Engine
- `src/clients/python/engine/trading_engine.py` — run_engine() tick loop

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 21 pattern: datasource extraction, V2 strategy, diff test, demo launcher
- `datasources/` package already exists with `__init__.py` and `base.py`
- `test_mean_reversion_diff.py` as template for diff tests

### Key Complexity
- PDFWheelStrategy extends WheelStrategy — must migrate Wheel first, then PDFWheel extends WheelV2
- CreditSpreadStrategy uses multi-leg orders with group_id — datasource must handle spread signal detection
- OptionsMeanReversion has different signal types than equity MeanReversion

</code_context>

<specifics>
## Specific Ideas

- Migration order should respect inheritance: Wheel before PDFWheel
- on_signal() with default no-op allows gradual adoption (strategies that don't override it still work)
- Each datasource should produce signals matching the TradeSignal struct format (name, symbol, timestamp, attributes)

</specifics>

<deferred>
## Deferred Ideas

- RecordSignal RPC removal — can happen after all strategies migrated (was deprecated in Phase 17)
- Live datasource scripts with __main__ WriteSignal calls — Phase 23 or later
- Composite signal framework — v3.1

</deferred>

---

*Phase: 22-remaining-strategy-migrations*
*Context gathered: 2026-03-31*
