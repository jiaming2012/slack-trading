# Design — widen-scan-results-columns

## Context

`scanner-l1-l2` discovered mid-implementation that the architecture doc's own `scan_results` SQL carries no columns for `price_vs_50ma`, `compression_score`, or `sector_momentum`. Per the spec-first discipline the gap was amended into the specs (all eight computed, five persisted) and deferred to this named change rather than silently re-opening `trading-stack-schema` overnight. The `FeatureVector` already computes and carries all three values; `RunScan` simply has nowhere to put them.

## Goals

- Persist all eight architecture-doc features on every qualifying `scan_results` row.
- Purely additive, idempotent schema change through the existing tradingstack migration path.

## Non-Goals

- No backfill of historical rows (raw values are immutable by spec; the features were never persisted, so there is nothing faithful to backfill — old rows stay NULL).
- No change to feature computation, Layer 1 gates, `data_as_of` invariants, or immutability semantics.
- No Layer 3 / ML-ranking work, no scan-entry-point or Feed-integration work.

## Decisions

### D1 — Nullable numeric columns via model fields + AutoMigrate (no bespoke migration script)

The three columns are added as `*float64` fields with explicit GORM tags (`column:price_vs_50ma;type:numeric`, etc.) on `tradingstack.ScanResult`, exactly matching the style of the existing five feature columns. `MigrateTradingStack` already runs `AutoMigrate(&ScanResult{}, ...)`, which adds missing columns idempotently — the tradingstack migration path *is* AutoMigrate plus idempotent constraint DO-blocks, so no new migration mechanism is introduced. Nullable pointers keep NULL round-tripping faithful (matching every other optional column) and make pre-change rows valid by construction. *Alternative rejected*: a JSONB `extra_features` column — breaks the architecture doc's columnar SQL contract and makes downstream training-data queries needlessly indirect.

### D2 — Partitioned-table compatibility is inherited, not special-cased

`db-partitioning-retention` may have converted `scan_results` into a partitioned table. `ALTER TABLE ... ADD COLUMN` on a partitioned parent cascades to all partitions in PostgreSQL, and AutoMigrate issues exactly that, so the additive migration works in both layouts. The existing partitioned-ness guard in `MigrateTradingStack` (which only affects the sim_outcomes FK) is untouched. The round-trip test suite runs `MigrateTradingStack` against a fresh container (non-partitioned layout); the partitioning package's own suite covers the partitioned layout's migration idempotence.

### D3 — Pipeline maps the three fields verbatim; no recomputation

`RunScan` copies `fv.PriceVs50MA`, `fv.CompressionScore`, and `fv.SectorMomentum10d` onto the row (nil stays NULL — e.g. `price_vs_50ma` with <50 closes, per the never-fabricate rule). `sector_momentum` remains a pass-through recorded verbatim from `ScanInput`. Insert-only persistence is untouched, so immutability keeps being enforced structurally. Comments in `pipeline.go` and `feature_vector.go` describing the five-of-eight gap are removed.

## Risks → Mitigations

- **Drift between model tags and architecture-doc column names** → column names are pinned in the spec delta scenarios and asserted by the round-trip test reading raw column values.
- **AutoMigrate against a live partitioned prod table** → additive `ADD COLUMN` is metadata-only and cascades; prod application is operator-only (task group 4) per the autonomous-run prohibitions, verified first on testcontainers and local dev.
- **Downstream readers assuming NULL means "insufficient history"** → NULL is already the never-fabricate signal for the existing feature columns; pre-change rows are distinguishable by `scanner_version` if a consumer ever needs to.

## Migration / Rollback

- **Migration**: run `MigrateTradingStack` (next server/test boot path that invokes it); purely additive, idempotent, safe to re-run.
- **Rollback**: revert the code commits — the columns remain in any database that migrated, which is harmless (nullable, unread by reverted code). Dropping them is an optional manual `ALTER TABLE ... DROP COLUMN`; no rollback script is shipped for an additive change.
