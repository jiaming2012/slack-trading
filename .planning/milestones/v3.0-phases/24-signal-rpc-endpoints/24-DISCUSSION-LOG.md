# Phase 24: Signal RPC Endpoints - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 24-signal-rpc-endpoints
**Areas discussed:** Reuse Phase 19 context

---

## Context Source

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse Phase 19 context | Phase 19 CONTEXT.md has detailed decisions (D-01 through D-07) for exactly this work. Adapt for Phase 24's narrower scope. | ✓ |
| Global signal storage | How does WriteSignal store signals server-side? | |
| Python client wrappers | Should Phase 24 include Python client wrappers or defer to Phase 26? | |
| Proto message design | Exact field design for request/response messages | |

**User's choice:** Reuse Phase 19 context
**Notes:** Phase 24 is gap closure for Phase 19 (work absent from branch). All decisions from Phase 19 CONTEXT.md carry forward. Scope narrowed: DS-03 and OBS-02 moved to Phase 26, MIG-01/MIG-02 moved to Phase 25.

---

## Claude's Discretion

- WriteSignal/GetSignals/GetProcessedSignals proto message field design
- globalSignalRepo initialization approach
- filterSignals helper design
- Python wrapper method exact signatures

## Deferred Ideas

- Datasource __main__ blocks → Phase 26
- DatasourceHeartbeat wiring → Phase 26
- Strategy on_signal() wiring → Phase 25
