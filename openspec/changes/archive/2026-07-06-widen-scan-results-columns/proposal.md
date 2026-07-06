# Widen scan_results Columns

## Why

The scanner (Layer 2) computes all eight architecture-doc features, but the `scan_results` table only has columns for five — `price_vs_50ma`, `compression_score`, and `sector_momentum` are computed and then dropped (a documented design amendment from the 2026-07-05 overnight run, deferred to exactly this named change). Every scan cycle run before this lands permanently loses three features that downstream training-data assembly (`sim_outcomes` labeling, Layer 3 ML ranking) will need, and scan rows are immutable by spec — the gap cannot be backfilled later.

## What Changes

- Add three nullable numeric columns to `scan_results` — `price_vs_50ma`, `compression_score`, `sector_momentum` — as fields on the `tradingstack.ScanResult` model, applied through the existing tradingstack migration path (`MigrateTradingStack`); the migration is purely additive and idempotent, and works whether or not `scan_results` has been converted to a partitioned table.
- Persist all eight computed features in the scanner pipeline (`RunScan`): the three new columns are mapped from the in-memory `FeatureVector`, which already carries them.
- Update the round-trip and pipeline tests: `ScanResult` round-trips the three new columns (including NULL), and a qualifying ticker's persisted row now carries all eight features.
- Rows written before this change simply have NULL in the new columns — no backfill, no rewrite (raw values are immutable by spec).
- No new operator-visible commands (existing `task test:trading-stack` and `task test:scanner` cover verification), no proto/RPC changes, no Python client changes. **Not BREAKING** — additive columns, existing readers unaffected.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `trading-stack-schema`: the `scan_results` model requirement gains the three feature columns.
- `scanner-feature-extraction`: the persistence requirement changes from the documented five-of-eight subset to persisting all eight features.

## Impact

- `src/go/tradingstack/scan_result.go` — three new nullable pointer fields (`*float64`) with GORM column tags; `MigrateTradingStack` picks them up via AutoMigrate with no code change of its own.
- `src/go/scanner/pipeline.go` — `RunScan` maps `PriceVs50MA`, `CompressionScore`, `SectorMomentum10d` onto the row; stale five-of-eight comments removed.
- Tests: `src/go/tradingstack/tradingstack_test.go` (round-trip) and `src/go/scanner/pipeline_test.go` (all-eight persistence) updated; both run against ephemeral testcontainers Postgres.
- Local/dev databases gain the columns on the next `MigrateTradingStack` run; prod application is operator-only per the autonomous-run prohibitions.
- Depends on archived changes `trading-stack-schema` and `scanner-l1-l2` (both landed 2026-07-05).
