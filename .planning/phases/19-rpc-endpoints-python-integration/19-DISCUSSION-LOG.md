# Phase 19: RPC Endpoints & Python Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-03-31
**Phase:** 19-rpc-endpoints-python-integration
**Areas discussed:** GetSignals RPC, Python datasource pattern, WriteSignal details, Single stream design

---

## GetSignals / Processed Signals RPC

| Option | Description | Selected |
|--------|-------------|----------|
| Per-playground query | GetProcessedSignals(playground_id, filters) | ✓ |
| Global signal query | GetSignals(filters) regardless of playground | |
| Both endpoints | Per-playground + global | |

**User's choice:** Per-playground query (Recommended)
**Notes:** Later clarified: signals are GLOBAL events. Per-playground query returns consumed copies. A global query is also needed.

---

## Python Datasource Script Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| src/clients/python/datasources/ | New subdir, each module has produce_signals() | ✓ |
| src/clients/python/scripts/ | Separate from strategy code | |
| You decide | | |

**User's choice:** datasources/ (Recommended)

## Sim Import Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Direct function import | from datasources.ma_crossover import produce_signals | ✓ |
| Datasource registry | Declarative name-based loading | |

**User's choice:** Direct function import (Recommended)

---

## WriteSignal RPC — Key Architectural Clarification

**User's major insight:** "Producing a signal is a global event. It should be possible that a signal is produced but not consumed by any strategy."

Key decisions from this clarification:
- WriteSignal does NOT take playground_id — signals are global
- Each produced signal gets an OTel counter
- Each consumed signal gets a separate OTel counter per strategy
- Playground query returns COPIES of consumed signals
- Global query reads from ESDB stream directly

---

## Example Datasources

**User's decision:** "Example datasources should be built while migrating legacy strategy. Maybe in a later phase."
→ Deferred to Phase 21-22. Phase 19 builds infrastructure pattern only.

---

## Claude's Discretion

- WriteSignal/GetSignals proto message design
- Python datasource skeleton structure
- How playgrounds store consumed signal copies
- OTel metric names

## Deferred Ideas

- Example datasource implementations → Phase 21-22
- Signal Grafana dashboard → Phase 23
- Missing signal alerting → Phase 23
