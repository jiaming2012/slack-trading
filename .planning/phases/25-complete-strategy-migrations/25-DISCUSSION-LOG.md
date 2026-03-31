# Phase 25: Complete Strategy Migrations - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 25-complete-strategy-migrations
**Areas discussed:** Reuse Phase 22 context

---

## Context Source

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse Phase 22 context | Phase 22 CONTEXT.md has detailed decisions for this work. Adapt for Phase 25 scope. | ✓ |
| on_signal() Python wiring | How client.py delivers new_signals from TickDelta | |
| Recovery vs rebuild | Investigate lost commits or rebuild from scratch | |

**User's choice:** Reuse Phase 22 context
**Notes:** Phase 25 is gap closure for Phase 22 (missing V2 files + unexecuted plans 22-03/04/05). All decisions from Phase 22 carry forward. Added D-07: rebuild from scratch rather than recovering lost commits.

---

## Claude's Discretion

- Datasource module design for covered_call_signals, wheel_signals, pdf_wheel_signals
- Diff test boundaries for each strategy
- Demo script updates
- Migration order within the phase

## Deferred Ideas

- Datasource __main__ blocks → Phase 26
- DatasourceHeartbeat wiring → Phase 26
