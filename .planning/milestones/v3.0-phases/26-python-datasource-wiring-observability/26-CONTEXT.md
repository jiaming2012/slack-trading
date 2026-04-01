# Phase 26: Python Datasource Wiring & Observability - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning
**Source:** Derived from Phase 19/23/24 decisions (gap closure)

<domain>
## Phase Boundary

Add `__main__` blocks to datasource scripts for standalone live-mode execution via WriteSignal RPC. Instantiate DatasourceHeartbeat in datasource scripts so OBS-02 alerts can fire. Ensure sim mode imports continue working.

Requirements: DS-03, OBS-02

</domain>

<decisions>
## Implementation Decisions

### D-01: Add __main__ to at least one datasource (DS-03)
- At minimum, `ma_crossover.py` gets a `__main__` block as the reference implementation
- The __main__ block: connects to server, calls produce_signals() in a loop, calls write_signal() RPC for each signal
- Other datasources can follow the same pattern
- Sim mode: strategies import produce_signals() directly (no __main__ needed)

### D-02: DatasourceHeartbeat instantiation (OBS-02)
- DatasourceHeartbeat class already exists in `engine/datasource_heartbeat.py`
- Instantiate it in the __main__ block of datasource scripts
- Call `record_check()` after each signal production cycle
- This emits the `grodt.datasource.heartbeat` gauge that the Grafana alert watches

### D-03: Standalone datasource pattern
- __main__ block accepts CLI args: --server-url, --symbol, --interval
- Uses BacktesterPlaygroundClient.write_signal() to send signals to server
- Runs in a loop with configurable interval between checks
- DatasourceHeartbeat.record_check() called each iteration

### Claude's Discretion
- Which datasources beyond ma_crossover get __main__ blocks (all vs subset)
- CLI argument design for standalone datasources
- Loop timing and error handling in __main__
- Whether to create a shared datasource runner utility or inline in each file

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Datasource Modules
- `src/clients/python/datasources/ma_crossover.py` — Primary target for __main__ block
- `src/clients/python/datasources/options_ma_crossover.py` — Options variant
- `src/clients/python/datasources/credit_spread_signals.py` — Credit spread signals
- `src/clients/python/datasources/covered_call_signals.py` — Covered call signals
- `src/clients/python/datasources/wheel_signals.py` — Wheel signals
- `src/clients/python/datasources/pdf_wheel_signals.py` — PDF wheel compound signals

### Infrastructure
- `src/clients/python/engine/datasource_heartbeat.py` — DatasourceHeartbeat class (exists, never instantiated)
- `src/clients/python/engine/client.py` — BacktesterPlaygroundClient with write_signal() method
- `src/clients/python/rpc/playground_twirp.py` — Auto-generated Twirp client with WriteSignal RPC

### Audit
- `.planning/v3.0-MILESTONE-AUDIT.md` — Documents DS-03 unsatisfied and OBS-02 heartbeat never instantiated

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- BacktesterPlaygroundClient.write_signal(name, symbol, timestamp, attributes) — ready to use
- DatasourceHeartbeat class with record_check() method — ready to instantiate
- All 6 datasource modules with produce_signals() functions — ready for __main__ wrapping

### Established Patterns
- Demo scripts (demos/) show CLI arg parsing with argparse
- BacktesterPlaygroundClient connection pattern in demo scripts
- produce_signals() returns list of signal dicts

### Integration Points
- Datasource __main__ blocks call write_signal() on client
- DatasourceHeartbeat emits gauge for Grafana alerting (OBS-02)
- Grafana alert rule for datasource-heartbeat-stale already exists (Phase 23)

</code_context>

<specifics>
## Specific Ideas

- ma_crossover.py is the simplest datasource — ideal as the reference __main__ implementation
- DatasourceHeartbeat should be instantiated with the datasource name for label differentiation
- The __main__ pattern should be copy-pasteable to other datasources

</specifics>

<deferred>
## Deferred Ideas

None — this is the last gap closure phase for v3.0.

</deferred>

---

*Phase: 26-python-datasource-wiring-observability*
*Context gathered: 2026-03-31*
