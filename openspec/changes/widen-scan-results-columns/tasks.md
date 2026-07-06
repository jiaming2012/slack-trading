# Tasks — widen-scan-results-columns

## 1. Schema widening (tradingstack migration path)

- [x] 1.1 `src/go/tradingstack/scan_result.go` — add nullable `*float64` fields `PriceVs50MA` (`column:price_vs_50ma;type:numeric`), `CompressionScore` (`column:compression_score;type:numeric`), and `SectorMomentum` (`column:sector_momentum;type:numeric`) to `ScanResult`, matching the existing feature-column style; picked up additively and idempotently by `MigrateTradingStack`'s AutoMigrate.
- [x] 1.2 `src/go/tradingstack/tradingstack_test.go` — round-trip coverage: the three columns persist and read back exact values and NULL; migration against a pre-widening table preserves existing rows with NULL in the new columns; second `MigrateTradingStack` run is a nil-error no-op.

## 2. Persist all eight features in the scanner pipeline

- [ ] 2.1 `src/go/scanner/pipeline.go` — `RunScan` maps `fv.PriceVs50MA`, `fv.CompressionScore`, and `fv.SectorMomentum10d` onto the `ScanResult` (nil stays NULL); remove the five-of-eight gap comment.
- [ ] 2.2 `src/go/scanner/feature_vector.go` — update the stale computed-not-persisted commentary to reflect all-eight persistence.
- [ ] 2.3 `src/go/scanner/pipeline_test.go` — a qualifying ticker's persisted row carries all eight features matching the in-memory `FeatureVector`; a <50-close input persists NULL `price_vs_50ma` (never fabricated); immutability (no update on re-run) and `data_as_of` fail-closed cases still pass unchanged.

## 3. Verification gates

- [ ] 3.1 `task test:trading-stack` — green (round-trip, invariant, and widening-migration cases).
- [ ] 3.2 `task test:scanner` — green (all-eight persistence, NULL semantics, existing filter/feature/invariant suite).
- [ ] 3.3 G1: `go build ./src/go/... ./cmd/...` — green.
- [ ] 3.4 G2: `task test` — green (backtester suite unaffected).
- [ ] 3.5 `openspec validate widen-scan-results-columns --strict` — passes.

## 4. Operator-only follow-ups (NEVER autonomous)

- [ ] 4.1 Operator applies the additive migration to the production database (run the tradingstack migration path against prod) — prod DB access is prohibited for autonomous runs; local/dev and testcontainers verification only until then.
