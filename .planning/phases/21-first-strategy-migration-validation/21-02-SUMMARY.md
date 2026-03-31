---
phase: 21-first-strategy-migration-validation
plan: 02
subsystem: testing
tags: [pytest, behavioral-diff, datasource, mean-reversion, migration-validation]

# Dependency graph
requires:
  - phase: 21-01
    provides: MeanReversionStrategyV2, ma_crossover datasource, test helpers
provides:
  - Behavioral diff test pattern proving V1/V2 zero metric drift
  - Datasource unit tests for produce_signals()
  - V2 demo launcher with identical CLI to V1
affects: [22-bulk-strategy-migration]

# Tech tracking
tech-stack:
  added: []
  patterns: [behavioral-diff-testing, normalized-group-id-comparison]

key-files:
  created:
    - src/clients/python/tests/test_mean_reversion_diff.py
    - src/clients/python/tests/test_ma_crossover.py
    - src/clients/python/demos/demo_mean_reversion_v2.py
  modified: []

key-decisions:
  - "Normalized group_id in order call comparisons since UUIDs are non-deterministic between V1/V2 runs"
  - "Patched _bar_to_dict in both strategy modules to control bar conversion deterministically"

patterns-established:
  - "Behavioral diff pattern: patch detect + _bar_to_dict in both source modules, normalize group_id, compare call_args_list"
  - "Reusable _normalize_order_calls helper for Phase 22 bulk migration validation"

requirements-completed: [MIG-03]

# Metrics
duration: 4min
completed: 2026-03-31
---

# Phase 21 Plan 02: Validation & Diff Testing Summary

**Behavioral diff tests proving zero metric drift between V1 and V2 mean-reversion strategies, with datasource unit tests and V2 demo launcher**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-31T00:36:38Z
- **Completed:** 2026-03-31T00:40:46Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Behavioral diff test validates V1/V2 produce identical order sequences on identical inputs (MIG-03)
- Datasource unit tests cover 5 edge cases for produce_signals() in isolation
- V2 demo launcher accepts same CLI args as V1 for direct comparison
- Diff test pattern documented and reusable for Phase 22 bulk migration

## Task Commits

Each task was committed atomically:

1. **Task 1: Behavioral diff test and datasource unit tests** - `706759b` (test)
2. **Task 2: Create V2 demo launcher script** - `d71b4c5` (feat)

## Files Created/Modified
- `src/clients/python/tests/test_ma_crossover.py` - 5 unit tests for produce_signals() edge cases
- `src/clients/python/tests/test_mean_reversion_diff.py` - 3 behavioral diff tests comparing V1 vs V2 order sequences
- `src/clients/python/demos/demo_mean_reversion_v2.py` - V2 demo launcher with identical CLI to V1

## Decisions Made
- Normalized group_id (UUID) in order call comparisons since UUIDs are non-deterministic -- each unique group_id mapped to stable label (group_0, group_1, etc.)
- Patched _bar_to_dict in both V1 and V2 modules to return known dicts, controlling exactly what bar data both strategies see
- Added 5th datasource test (signal_key_not_in_pdf) beyond the 4 specified in plan for completeness

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Normalized non-deterministic group_id in diff assertions**
- **Found during:** Task 1 (behavioral diff test)
- **Issue:** V1 and V2 generate different UUIDs for group_id, causing exact call comparison to fail even though all other order parameters match
- **Fix:** Created _normalize_order_calls() helper that maps each unique group_id to a stable label (group_0, group_1, ...)
- **Files modified:** src/clients/python/tests/test_mean_reversion_diff.py
- **Verification:** All 8 tests pass with normalized comparison
- **Committed in:** 706759b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential for correct diff testing -- UUIDs are inherently non-deterministic. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all tests wire real strategy classes with mocked dependencies.

## Next Phase Readiness
- Phase 21 complete: V1-to-V2 migration validated with behavioral diff tests
- Diff test pattern ready for Phase 22 bulk migration of 6 remaining strategies
- Key reusable artifacts: _normalize_order_calls(), _run_strategy() helper, dual-module patching pattern

---
*Phase: 21-first-strategy-migration-validation*
*Completed: 2026-03-31*
