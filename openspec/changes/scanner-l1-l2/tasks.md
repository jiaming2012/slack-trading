# Tasks — scanner-l1-l2

## 1. Package scaffold and input contract

- [x] 1.1 Create `src/go/scanner/` package with `input.go` (`ScanInput`, `Candle` history type) and `errors.go` (`ErrDataAsOfAfterScannedAt`, `ErrFilteredOut`).

## 2. Layer 1 — hard filters

- [x] 2.1 `hard_filters.go` — `LiquidityGate` (avg daily volume > 500k, price > $5, market cap > $300M, each with its own failure reason).
- [x] 2.2 `hard_filters.go` — `DataQualityGate` (no earnings within 3 days via `DaysToNextEarnings`, no recent halt via `RecentHalt`, options chain exists via `HasOptionsChain`).
- [x] 2.3 `regime_thresholds.go` — built-in default per-regime (`trending`/`mean_reverting`/`high_vol`) ATR%-ceiling and price-vs-50MA-bound table.
- [x] 2.4 `hard_filters.go` — `RegimeFilter` using the table from 2.3, no-op when `RegimeTag` is empty/unrecognized.
- [x] 2.5 `hard_filters.go` — `RunLayer1` aggregating all three gates, collecting every failure reason (no short-circuit after first failure).

## 3. Layer 2 — feature extraction

- [x] 3.1 `features.go` — `ComputeRSI14` (standard 14-period RSI; nil when fewer than 15 closes).
- [x] 3.2 `features.go` — `ComputeATRPct` (ATR(14) / price; nil when fewer than 15 candles).
- [x] 3.3 `features.go` — `ComputePriceVs50MA` (signed % vs 50-day SMA; nil when fewer than 50 closes).
- [x] 3.4 `features.go` — `ComputeCompressionScore` (Bollinger Band width percentile vs trailing historical distribution).
- [x] 3.5 `features.go` — `ComputeVolumeRatio` (pace-adjusted: normalize both today's partial volume and the 20-day baseline to the same elapsed-session-time fraction before dividing).
- [x] 3.6 `feature_vector.go` — `FeatureVector` struct + pass-through recording of `ShortInterest`, `SectorMomentum10d`, `RegimeTag` verbatim.
- [x] 3.7 `feature_vector.go` — `BuildFeatureVector` deriving `data_as_of` from the latest candle timestamp used and enforcing `data_as_of <= scanned_at` (fail closed: error, no partial vector).

## 4. Pipeline and persistence

- [x] 4.1 `pipeline.go` — `RunScan(db, input)`: Layer 1 gate (no write on failure) → `BuildFeatureVector` → map onto `tradingstack.ScanResult` (import from `trading-stack-schema`) → stamp `scanner_version` → `db.Create()`. No update/mutate path for an existing row's feature values.

## 5. Tests (fixture-driven)

- [x] 5.1 `hard_filters_test.go` — table tests for every gate: pass-all-thresholds, and one failure case per individual condition (liquidity x3, data-quality x3, regime x1 plus the "same value passes under a looser regime" and "missing regime is a no-op" cases), plus the multi-gate-failure-reports-all-reasons case.
- [x] 5.2 `features_test.go` — hand-computed fixture expectations for `rsi_14`, `atr_pct`, `price_vs_50ma`, `compression_score`, `volume_ratio` (including the pace-adjustment cases), plus insufficient-history-yields-nil cases for `rsi_14` and `price_vs_50ma`.
- [x] 5.3 `pipeline_test.go` — testcontainers Postgres (via `trading-stack-schema`'s `MigrateTradingStack`) tests: qualifying ticker produces exactly one `scan_results` row populated with the five schema-backed features (`rsi_14`, `volume_ratio`, `atr_pct`, `short_interest`, `regime_tag`) plus `scanner_version`/`data_as_of`, while the in-memory `FeatureVector` for the same input also has `price_vs_50ma`, `compression_score`, and `sector_momentum` computed (not persisted — see `design.md`); Layer-1-rejected ticker produces no row and no feature computation; `data_as_of` after `scanned_at` fails closed with no row written; re-running for the same ticker/`scanned_at` does not mutate the first row's stored values; two runs with different `scanner_version` values tag their rows distinctly.

## 6. Taskfile wiring

- [x] 6.1 Add `test:scanner` target to `taskfile.yml` running `go test -count=1 ./src/go/scanner/...`, exiting non-zero on any failure.

## 7. Verification and closeout

- [x] 7.1 Fixture-driven unit tests pass: `task test:scanner` green (all hard-filter, feature-computation, accuracy-invariant, immutability, and persistence cases).
- [x] 7.2 G1 — `go build ./src/go/... ./cmd/...` green.
- [x] 7.3 G2 — `task test` green (existing backtester-api suite unaffected).
- [x] 7.4 Confirm `trading-stack-schema` has landed (its models/migration are what `pipeline_test.go` depends on) before closing this change.
- [ ] 7.5 Flip this change's card(s) in usm/roadmap.txt to green #C5E1A5 and refresh ROADMAP.md current-state (done at archive time).
