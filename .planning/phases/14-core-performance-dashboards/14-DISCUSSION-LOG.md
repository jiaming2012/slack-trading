# Phase 14: Core Performance Dashboards - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-30
**Phase:** 14-core-performance-dashboards
**Areas discussed:** Dashboard structure, Playground selector, Dashboard-as-code, Equity curve, Maintenance

---

## Dashboard Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Separate dashboards | 3 dashboards: Trading Performance, Slippage Analysis, Portfolio Breakdown | ✓ |
| One combined dashboard | Single dashboard with tabs/sections for all metrics | |
| You decide | Claude picks best structure | |

**User's choice:** Separate dashboards (Recommended)
**Notes:** None

---

## Playground Selector

| Option | Description | Selected |
|--------|-------------|----------|
| Client ID dropdown | Filter by client_id — unique human-readable identifier | |
| Multi-filter chain | Environment → date range → client_id | |
| You decide | Claude designs filter pattern | |

**User's choice:** Custom — "Client ID or Playground ID (if not or in case not available), then environment, then tags. It would be nice to be able to select multiple playgrounds and see combined stats"
**Notes:** Multi-select for combined stats across playgrounds is important to the user.

---

## Dashboard-as-Code

| Option | Description | Selected |
|--------|-------------|----------|
| Metabase API scripting | Python script using Metabase REST API, fully reproducible | ✓ |
| Hand-build + JSON export | Build in UI, export JSON for backup | |
| Hand-build only | Build in UI, no export | |

**User's choice:** Metabase API scripting (Recommended)
**Notes:** None

---

## Equity Curve & Time Series

| Option | Description | Selected |
|--------|-------------|----------|
| Line chart with starting balance | Simple line + horizontal reference line | |
| Line chart + drawdown overlay | Line chart plus shaded drawdown from peak | ✓ |
| You decide | Claude picks visualization | |

**User's choice:** Line chart + drawdown overlay
**Notes:** None

---

## Maintenance Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Python script in infra/ | infra/provision-metabase.py, version-controlled | ✓ |
| Bash + curl script | infra/provision-metabase.sh, simpler but harder to maintain | |
| You decide | Claude picks approach | |

**User's choice:** Python script in infra/ (Recommended)
**Notes:** User specifically asked about minimizing maintenance debt. Key mitigations: SQL views abstract schema, API scripting for reproducibility, idempotent scripts.

---

## Claude's Discretion

- Exact Metabase question types (native SQL vs simple vs custom)
- Chart types for non-equity visualizations
- Dashboard layout and card sizing
- Filter widget types

## Deferred Ideas

- Strategy comparison dashboard (DASH-03) — Phase 15
- Spread analytics dashboard (DASH-05) — Phase 16
- Per-asset-class split in v_playground_stats — can address via GROUP BY in Phase 14 dashboards
