# Phase 15: Simulator Persistence & Backtest Comparison - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-30
**Phase:** 15-simulator-persistence-backtest-comparison
**Areas discussed:** When to persist, backtest_runs table design, Comparison dashboard, Parameter capture

---

## When to Persist

| Option | Description | Selected |
|--------|-------------|----------|
| Always persist | Every simulator backtest saves automatically | |
| Explicit save_to_db flag | Only save when --save-to-db is passed | ✓ |
| You decide | Claude picks | |

**User's choice:** Explicit save_to_db flag
**Notes:** Keeps DB clean during rapid iteration

---

## backtest_runs Table Design

| Option | Description | Selected |
|--------|-------------|----------|
| Client ID as identifier | Human-readable, already used in Grafana/Metabase | ✓ |
| Auto-generated run ID | UUID, guaranteed unique | |
| You decide | Claude designs | |

**User's choice:** Client ID as identifier (Recommended)
**Notes:** None

---

## Comparison Dashboard

| Option | Description | Selected |
|--------|-------------|----------|
| Table + overlay charts | Sortable table + equity curve overlays, filter by strategy | ✓ |
| Side-by-side cards | Pick 2-3 runs, compare in columns | |
| You decide | Claude designs | |

**User's choice:** Table + overlay charts (Recommended)
**Notes:** None

---

## Parameter Capture

| Option | Description | Selected |
|--------|-------------|----------|
| JSONB column from Python | Flexible, no schema changes for new strategies | ✓ |
| Predefined columns | Typed columns, rigid | |
| You decide | Claude picks | |

**User's choice:** JSONB column from Python (Recommended)
**Notes:** None

---

## Claude's Discretion

- backtest_runs population method (new RPC vs Python SQL insert)
- Exact schema details beyond decided columns
- Dashboard layout for DASH-03
- Strategy parameter serialization specifics

## Deferred Ideas

- Automated comparison reports
- Backtest scheduling
- Parameter optimization dashboard
