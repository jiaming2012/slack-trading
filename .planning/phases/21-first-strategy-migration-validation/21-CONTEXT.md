# Phase 21: First Strategy Migration & Validation - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate MeanReversionStrategy to consume TradeSignals instead of inline signal detection. Create the MA crossover datasource module. Prove the behavioral diff testing pattern with zero metric drift. Document the pattern for Phase 22 bulk migration.

Requirements: MIG-03

</domain>

<decisions>
## Implementation Decisions

### D-01: First strategy — MeanReversionStrategy
- Most explicit signal detection code (MA crossover logic in on_tick)
- Clearest signal boundaries — best for proving the pattern
- Has the most tests, demo scripts (demo_mean_reversion.py), and metrics tooling (playground_metrics.py)

### D-02: Datasource module — ma_crossover
- Create `src/clients/python/datasources/ma_crossover.py`
- Implements `produce_signals(candle_data)` → list of TradeSignal dicts
- Extracts the MA crossover detection logic from MeanReversionStrategy.on_tick()
- Sim mode: strategy imports directly (`from datasources.ma_crossover import produce_signals`)
- Live mode: runs from `__main__`, calls WriteSignal RPC

### D-03: Migrated strategy location
- New file: `src/clients/python/strategies/mean_reversion_v2.py` (or similar)
- Original `mean_reversion.py` moved to `deprecated/` after validation
- Migrated strategy consumes TradeSignals from TickDelta.new_signals instead of computing signals inline

### D-04: Behavioral diff testing — Claude's discretion
- Compare migrated vs original strategy output on the same input data
- Metrics to compare: total P&L, win rate, profit factor, trade count, trade timestamps
- Use playground_metrics.py or v_playground_stats for comparison
- Zero metric drift tolerance (exact match, not approximate)
- Pattern must be reusable for remaining 6 strategies in Phase 22

### Claude's Discretion
- Diff test implementation details (framework, assertions, test data source)
- How to extract MA crossover logic cleanly from on_tick()
- Whether MeanReversionStrategyV2 extends BaseStrategy or is a new class
- Demo script updates for the migrated strategy

</decisions>

<canonical_refs>
## Canonical References

### Strategy Source (to migrate)
- `src/clients/python/strategies/mean_reversion.py` — MeanReversionStrategy with inline MA crossover detection
- `src/clients/python/demos/demo_mean_reversion.py` — Demo/launcher script

### Signal Infrastructure (Phases 17-20)
- `src/clients/python/datasources/base.py` — Datasource skeleton pattern
- `src/clients/python/engine/client.py` — write_signal(), get_processed_signals() wrappers
- `src/go/playground.proto` — TickDelta.new_signals field

### Testing Reference
- `src/clients/python/tools/playground_metrics.py` — P&L, win rate, profit factor calculations
- `src/clients/python/tests/` — Existing Python test patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `produce_signals()` pattern from `datasources/base.py` skeleton
- `write_signal()` on BacktesterPlaygroundClient
- `playground_metrics.py` for metric comparison
- `demo_mean_reversion.py` as launcher template

### Integration Points
- MeanReversionStrategy.on_tick() — extract signal detection into datasource module
- TickDelta.new_signals — migrated strategy reads signals from here
- `from datasources.ma_crossover import produce_signals` — sim import

</code_context>

<specifics>
## Specific Ideas

- "Zero metric drift" means exact same P&L, trades, timing when run on same data
- Diff test should be a pytest that runs both strategies on identical playground and compares
- The MA crossover datasource is also an example for the Notion doc's `ma_crossover` signal type

</specifics>

<deferred>
## Deferred Ideas

- Remaining 6 strategy migrations — Phase 22
- Moving original to deprecated/ — happens after diff test passes (could be Phase 22)

</deferred>

---

*Phase: 21-first-strategy-migration-validation*
*Context gathered: 2026-03-31*
