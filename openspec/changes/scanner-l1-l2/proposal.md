# Scanner Layers 1-2 (Hard Filters + Feature Extraction)

## Why

The AI Scanner cannot surface candidates without a fast, deterministic first pass that eliminates unfit tickers and extracts a faithful, bias-free feature vector for every survivor — this change builds that pass (Layers 1-2 only; the ML ranking Layer 3 is a separate future change).

## What Changes

- Add a new Go package implementing Layer 1 hard filters: liquidity gate (avg daily volume > 500k, price > $5, market cap > $300M), data quality gate (no earnings within 3 days, no recent halts, options chain exists), and a regime-conditional filter (ATR% and moving-average bounds vary by `regime_tag`).
- Add Layer 2 feature extraction computing all eight architecture-doc features: `rsi_14`, `volume_ratio` (pace-adjusted intraday), `atr_pct`, `price_vs_50ma`, `compression_score`, `short_interest`, `sector_momentum`, `regime_tag`. All eight are computed on the in-memory feature vector; **persistence is a subset** — the current `trading-stack-schema` `scan_results` table only has columns for five of them (`rsi_14`, `volume_ratio`, `atr_pct`, `short_interest`, `regime_tag`), so `price_vs_50ma`, `compression_score`, and `sector_momentum` are computed but not written to `scan_results` in this change. Widening the schema to persist all eight is a named future change, `widen-scan-results-columns`.
- Enforce the Feature Vector Accuracy Rules from the architecture doc: every feature vector is stamped with a single `data_as_of` timestamp that must be `<= scanned_at` (fail closed, no row written, on violation); raw values captured at scan time are never recomputed retroactively; every persisted row is tagged with `scanner_version`.
- Persist qualifying scan results as `tradingstack.ScanResult` rows (the model landed by `trading-stack-schema`) via GORM — Layer 1 rejects never reach persistence; persisted columns are the five-feature subset above plus `scanner_version` and `data_as_of`.
- Add fixture-driven unit tests covering every filter, every feature computation, and every accuracy rule against hand-built candle series with known expected values.
- Add a `test:scanner` Taskfile target to run this package's tests in isolation, matching the `test:trading-stack` convention.
- No changes to any existing playground table, model, RPC surface, or migration path. **Not BREAKING.**
- Layer 3 (ML ranking / XGBoost), the Scanner Optimizer's dynamic hot-swappable config, live Feed integration to assemble scan inputs, and any RPC/CLI/scheduled entry point to invoke a scan cycle are explicitly **out of scope** for this change.

## Capabilities

### New Capabilities

- `scanner-hard-filters`
- `scanner-feature-extraction`

### Modified Capabilities

(none)

## Impact

- New package: `src/go/scanner/` (hard filters, feature extraction, feature-vector accuracy enforcement, persistence, fixture-based tests). No existing files modified other than `taskfile.yml` (adds `test:scanner`).
- Depends on `trading-stack-schema`: imports the `tradingstack.ScanResult` model to persist qualifying results. `trading-stack-schema` must land and archive before this change's persistence code can compile and its tests can run against a real schema.
- New dependency surface only within existing modules already vendored: `gorm.io/gorm`, `github.com/testcontainers/testcontainers-go` (for the persistence round-trip test). No new external infrastructure.
- Overnight scope is the engine plus fixture-candle-data tests only. Verification is local-only (in-process unit tests over fixtures, plus a testcontainers Postgres persistence test). No prod DB, no live/paper broker orders, no live Massive/Polygon feed calls. Validating this engine against the real Polygon universe is explicitly deferred (see `design.md`).
