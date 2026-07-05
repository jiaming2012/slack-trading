# Crowding Detection

## Why

Strategies currently run independently with no visibility into whether several of them are converging on the same tickers in the same scan cycle, which quietly multiplies concentration risk without multiplying edge.

## What Changes

- Add a `crowding` subpackage that computes, per scan cycle (all `scan_results` sharing one `scanned_at`), what percentage of candidates were selected/simulated by two or more distinct strategies (via `sim_outcomes.strategy_id`).
- Add a configurable overlap threshold (percent, default 30%); a cycle is flagged when overlap is strictly greater than the threshold.
- Add two new GORM-backed tables, owned by this change: `crowding_metrics` (one row per scan-cycle summary) and `crowding_flagged_candidates` (one row per crowded candidate, naming the overlapping strategies) — both with their own migration entry point, additive only, no changes to the six `trading-stack-schema` tables.
- Add a `CrowdingStore` interface with a GORM-backed production implementation and an in-memory fake for deterministic unit tests, so the aggregation logic is testable without a live database.
- Add a `task test:crowding-detection` Taskfile target running this package's unit tests (new operator-visible command per project convention).
- No changes to any existing playground table, model, or migration, and no changes to the `trading-stack-schema` tables themselves. **Not BREAKING.**
- Wiring the detector into the live scan pipeline and into the (not-yet-built) portfolio risk overlay's consumption path is explicitly out of scope — see design.md.

## Capabilities

### New Capabilities

- `crowding-detection`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/tradingstack/crowding/` (models, migration, detection logic, store, tests). Imports the `tradingstack` package's `ScanResult` and `SimOutcome` models read-only; writes only to its own two new tables.
- No existing files modified other than `taskfile.yml` (adds `test:crowding-detection`).
- **Depends on `trading-stack-schema`**: this change imports `ScanResult` and `SimOutcome` from `github.com/jiaming2012/slack-trading/src/go/tradingstack` and must be authored and merged after `trading-stack-schema` lands.
- Feeds the (separately proposed, not-yet-drafted) `portfolio-risk-overlay` change: the `crowding_metrics` and `crowding_flagged_candidates` tables are the intended read surface for that change's pre-trade concentration checks. This change does not implement or depend on `portfolio-risk-overlay` — it only produces data that change can later consume.
- Verification is local-only: pure-function unit tests over fixture data plus `go build` / `task test`. No prod DB, no live/paper broker, no new external infrastructure.
