# Phase 25: Complete Strategy Migrations - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning
**Source:** Adapted from Phase 22 CONTEXT.md (gap closure — missing V2s + unexecuted plans)

<domain>
## Phase Boundary

Complete the remaining V2 strategy migrations (covered_call, wheel, pdf_wheel), wire on_signal() delivery in Python client.py, update trading engine for V2-only execution, and move all V1 strategy files to deprecated/.

Requirements: MIG-01, MIG-02

**Out of scope for this phase:**
- Datasource __main__ blocks (Phase 26)
- DatasourceHeartbeat wiring (Phase 26)
- New WriteSignal RPC calls from datasources (Phase 26)

</domain>

<decisions>
## Implementation Decisions

### D-01: Apply Phase 21 pattern to remaining strategies
- Same approach: extract datasource → create V2 → diff test
- Full copy approach (not inheritance) for each V2
- Strategies still needing V2: CoveredCall (OptionsStrategyBasic), Wheel, PDFWheel
- Already migrated (on disk): MeanReversion, OptionsMeanReversion, CreditSpread

### D-02: on_signal() is a default no-op (already implemented in 22-01)
- BaseStrategy has `on_signal(signal)` as default no-op — not abstract
- V2 strategies can override if they consume signals from TickDelta.new_signals
- For now, V2 strategies call produce_signals() inline (Phase 22 pattern)
- on_signal() wiring in client.py enables future framework consumption path

### D-03: Wire on_signal() delivery in Python client.py
- client.py tick() method must extract new_signals from TickDelta proto response
- For each signal, call strategy.on_signal(signal)
- This unblocks the framework consumption path (even if V2s currently call produce_signals inline)

### D-04: Move ALL V1 originals to deprecated/
- After all V2s pass diff tests, move V1 files to `src/clients/python/deprecated/`
- Files to move: mean_reversion.py, options_mean_reversion.py, credit_spread.py, covered_call.py, wheel.py, pdf_wheel.py
- Update any imports that reference old locations
- deprecated/ already exists with __init__.py

### D-05: Update trading engine for V2-only
- trading_engine.py should be able to run with only V2 strategies
- No legacy signal detection paths should remain in production code

### D-06: PDFWheel extends Wheel — migration order matters
- Must create wheel_v2.py BEFORE pdf_wheel_v2.py
- PDFWheelV2 should extend WheelStrategyV2 (not V1 parent)
- This maintains clean V2 inheritance chain

### D-07: Rebuild missing V2s from scratch (not recovery)
- Phase 22-03 SUMMARY claimed covered_call_v2/wheel_v2 were built but files are absent from branch
- Rebuild from scratch using the proven Phase 21/22 pattern rather than investigating lost commits
- This is cleaner and avoids potential merge artifacts

### Claude's Discretion
- Exact datasource module design for covered_call_signals, wheel_signals, pdf_wheel_signals
- Diff test approach for each strategy (which boundaries to compare)
- Demo script updates
- Order of operations within the migration

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 21 Pattern (proven, follow exactly)
- `src/clients/python/datasources/ma_crossover.py` — datasource extraction pattern
- `src/clients/python/strategies/mean_reversion_v2.py` — V2 strategy pattern
- `src/clients/python/tests/test_mean_reversion_diff.py` — diff test pattern

### Already Migrated V2s (reference for consistency)
- `src/clients/python/strategies/options_mean_reversion_v2.py` — options V2 pattern
- `src/clients/python/strategies/credit_spread_v2.py` — credit spread V2 pattern
- `src/clients/python/datasources/options_ma_crossover.py` — options datasource
- `src/clients/python/datasources/credit_spread_signals.py` — credit spread datasource

### V1 Strategies to Migrate
- `src/clients/python/strategies/covered_call.py` — OptionsStrategyBasic
- `src/clients/python/strategies/wheel.py` — WheelStrategy (extends OptionsStrategyBasic)
- `src/clients/python/strategies/pdf_wheel.py` — PDFWheelStrategy (extends WheelStrategy)

### Infrastructure
- `src/clients/python/strategies/base_strategy.py` — BaseStrategy ABC with on_signal() hook
- `src/clients/python/engine/client.py` — BacktesterPlaygroundClient (tick() method needs new_signals wiring)
- `src/clients/python/engine/trading_engine.py` — run_engine() tick loop

### Audit
- `.planning/v3.0-MILESTONE-AUDIT.md` — Documents missing V2 files and dead on_signal() code

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- 3 completed V2 migrations as templates (mean_reversion, options_mean_reversion, credit_spread)
- 3 datasource modules as templates
- 3 diff test suites as templates
- on_signal() hook already on BaseStrategy (no-op default)
- deprecated/ directory already exists with __init__.py

### Key Complexity
- PDFWheelStrategy extends WheelStrategy — must migrate Wheel first
- CoveredCall (OptionsStrategyBasic) has complex options signal detection with feature_vector_fn
- Wheel adds put signal detection on top of CoveredCall's call signals

### Integration Points
- `engine/client.py` tick() — add new_signals extraction from TickDelta
- `engine/trading_engine.py` — update strategy imports to V2
- `strategies/__init__.py` — may need updating for V2 exports
- `deprecated/` — destination for V1 files

</code_context>

<specifics>
## Specific Ideas

- Follow Phase 22-03 plan design for covered_call and wheel (the plan exists, files just weren't committed)
- Use callable-based datasource pattern (feature_vector_fn) established in 22-03 for state-dependent signal extraction
- WheelStrategyV2 extends OptionsStrategyBasicV2 maintaining clean V2 inheritance chain

</specifics>

<deferred>
## Deferred Ideas

- Datasource __main__ blocks → Phase 26
- DatasourceHeartbeat instantiation → Phase 26
- WriteSignal RPC calls from standalone datasources → Phase 26

</deferred>

---

*Phase: 25-complete-strategy-migrations*
*Context gathered: 2026-03-31*
