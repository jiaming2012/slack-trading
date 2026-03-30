---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Metabase Analytics
status: executing
stopped_at: Phase 15 context gathered
last_updated: "2026-03-30T15:50:23.505Z"
last_activity: 2026-03-30
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 4
  completed_plans: 4
  percent: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-30)

**Core value:** When a live simulation is running, the operator can always tell whether the system is alive and what it's doing -- even when no trades are being placed.
**Current focus:** Phase 14 — core-performance-dashboards

## Current Position

Phase: 15
Plan: Not started
Status: Ready to execute
Last activity: 2026-03-30

Progress: [██░░░░░░░░] 20%

## Performance Metrics

**Velocity:**

- Total plans completed: 18 (v1.0) + 4 (v1.1) + 1 (v2.0) = 23
- Average duration: ~5min
- Total execution time: ~1.8 hours

**Recent Trend (v1.1):**

- Last 2 plans: Phase 10 P01 (2m24s), Phase 11 P01 (2min)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap v2.0]: 5 phases derived from 18 requirements following research-recommended build order
- [Roadmap v2.0]: Spread analytics last (Phase 16) -- highest complexity, needs stable P&L baselines from Phase 14
- [Roadmap v2.0]: DASH-03 grouped with PERSIST phase (not core dashboards) because it requires backtest_runs data
- [Phase 12]: Metabase on Windows Desktop with read-only metabase_ro user and idempotent SQL init
- [Phase 13]: Used simple AVG(price) not VWAP for GetAvgFillPrice to match Go server CalcRealizedPL() as authoritative source
- [Phase 13]: v_playground_stats aggregates only opening-side orders to avoid double-counting P&L
- [Phase 14]: Metabase v0.59 API uses dashcards key and unique negative IDs for card creation

### Pending Todos

None yet.

### Blockers/Concerns

- [RESOLVED] JVM OOM risk -- Metabase runs on Windows Desktop, not DO droplet. JVM capped at -Xmx768m + mem_limit 1.5g
- [Research]: Verify equity_plot_records write timing before Phase 15 planning (incremental vs on-completion)
- [Research]: calc_pnl SQL function must mirror Go CalcRealizedPL() -- read order_record.go before Phase 16

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260328-rk1 | E2E integration test: live playground equity trade + dashboard metric verification | 2026-03-28 | d21a7f0 | [260328-rk1](./quick/260328-rk1-integration-test-live-playground-with-eq/) |
| 260329-11y | Python E2E: BaseStrategy subclass with signals, mock fill, dashboard metrics | 2026-03-29 | 1a5ca0a | [260329-11y](./quick/260329-11y-python-integration-test-basestrategy-sub/) |
| 260329-f7k | MockAddCandle RPC + candle injection E2E test with CandlesProcessed metric | 2026-03-29 | 79fa0bd | [260329-f7k](./quick/260329-f7k-add-mockaddcandle-rpc-for-live-playgroun/) |
| Phase 10 P01 | 2m24s | 3 tasks | 5 files |
| Phase 11-per-strategy-dashboards P01 | 2min | 2 tasks | 4 files |
| Phase 14 P02 | human-gated | 2 tasks | 2 files |

## Session Continuity

Last session: 2026-03-30T15:50:23.499Z
Stopped at: Phase 15 context gathered
Resume file: .planning/phases/15-simulator-persistence-backtest-comparison/15-CONTEXT.md
