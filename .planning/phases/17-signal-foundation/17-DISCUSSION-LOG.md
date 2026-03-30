# Phase 17: Signal Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-03-30
**Phase:** 17-signal-foundation
**Areas discussed:** Signal name registry, Attributes type in proto, signal_id design, RecordSignal relationship

---

## Signal Name Registry

| Option | Description | Selected |
|--------|-------------|----------|
| Go string type + constants | type SignalName string with const block | ✓ |
| Proto enum | Strongest typing but requires regen for new signals | |
| You decide | | |

**User's choice:** Go string type + constants (Recommended)

---

## Attributes Type in Proto

| Option | Description | Selected |
|--------|-------------|----------|
| map<string, string> + JSON | Simple, matches existing pattern | ✓ |
| google.protobuf.Struct | Native dynamic type, verbose in Go | |
| You decide | | |

**User's choice:** map<string, string> + JSON for complex values (Recommended)

---

## signal_id Design

| Option | Description | Selected |
|--------|-------------|----------|
| Go server generates | UUID on signal creation, Python passes it forward | |
| Python generates | Simpler but no server-side validation | |
| You decide | | ✓ |

**User's choice:** You decide

---

## RecordSignal Relationship

| Option | Description | Selected |
|--------|-------------|----------|
| Deprecate, replace later | Keep functional, remove in Phase 22 | ✓ |
| Remove immediately | Delete from proto now | |
| You decide | | |

**User's choice:** Deprecate, replace later (Recommended)

---

## Claude's Discretion

- signal_id generation strategy
- TradeSignal struct location and naming
- Convenience methods timing

## Deferred Ideas

- WriteSignal RPC, repositories, datasource scripts, strategy migration — all later phases
