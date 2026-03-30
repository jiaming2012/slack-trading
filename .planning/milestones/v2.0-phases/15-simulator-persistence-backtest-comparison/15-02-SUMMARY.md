---
phase: 15-simulator-persistence-backtest-comparison
plan: 02
subsystem: infra
tags: [metabase, dashboards, backtest-comparison, strategy-analytics, python]

requires:
  - phase: 15-01
    provides: "backtest_runs table and persistence module"
  - phase: 14-01
    provides: "provision-metabase.py with 3 dashboards"
provides:
  - "Strategy Comparison dashboard (Dashboard 4) in provision-metabase.py"
  - "5 comparison cards: runs table, equity curves, parameters, best/worst return"
  - "strategy_name filter for cross-backtest comparison"
affects: [16-spread-analytics]

tech-stack:
  added: []
  patterns: ["_make_strategy_card helper for strategy-filtered SQL cards", "STRATEGY_TEMPLATE_TAGS and _strategy_dash_card for strategy-scoped dashboards"]

key-files:
  created: []
  modified: ["infra/provision-metabase.py"]

key-decisions:
  - "Separate template tags (STRATEGY_TEMPLATE_TAGS) for strategy dashboard rather than parameterizing _make_card"
  - "Equity curve overlay via JOIN backtest_runs to equity_plot_records, grouped by client_id"

patterns-established:
  - "Strategy-filtered cards: use _make_strategy_card + _strategy_dash_card for dashboards filtered by strategy_name"

requirements-completed: [DASH-03]

duration: 1m21s
completed: 2026-03-30
---

# Phase 15 Plan 02: Strategy Comparison Dashboard Summary

**Strategy Comparison dashboard with 5 cards (runs table, equity curves overlay, parameter comparison, best/worst return scalars) filtered by strategy_name**

## Performance

- **Duration:** 1m 21s
- **Started:** 2026-03-30T16:15:23Z
- **Completed:** 2026-03-30T16:16:44Z
- **Tasks:** 1 of 1 auto tasks (Task 2 is human-verify checkpoint)
- **Files modified:** 1

## Accomplishments
- Added Strategy Comparison dashboard (Dashboard 4) to provision-metabase.py
- Created 5 comparison cards: sortable backtest runs table, equity curve overlay, parameter values, best/worst return scalars
- Implemented strategy_name filter using separate STRATEGY_TEMPLATE_TAGS (not playground_id)
- Equity curves JOIN backtest_runs to equity_plot_records for multi-run overlay by client_id
- Updated summary output from 3 to 4 dashboards

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Strategy Comparison dashboard to provision-metabase.py** - `749f37d` (feat)

**Plan metadata:** pending (docs: complete plan)

## Files Created/Modified
- `infra/provision-metabase.py` - Added Dashboard 4 (Strategy Comparison) with build_strategy_comparison_cards(), assemble_strategy_comparison(), STRATEGY_TEMPLATE_TAGS, STRATEGY_PARAMETER, _make_strategy_card, _strategy_param_mapping, _strategy_dash_card helpers

## Decisions Made
- Used separate STRATEGY_TEMPLATE_TAGS rather than parameterizing existing _make_card -- cleaner separation between playground-scoped and strategy-scoped dashboards
- Equity curve overlay groups by client_id (the backtest run identifier) for meaningful comparison labels

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required. Run `python infra/provision-metabase.py --password <pw>` to provision.

## Next Phase Readiness
- Dashboard 4 ready for human verification (Task 2 checkpoint)
- Requires backtest_runs table to exist (from analytics-schema.sql migration)
- All 4 dashboards provisioned in a single script run

---
*Phase: 15-simulator-persistence-backtest-comparison*
*Completed: 2026-03-30*
