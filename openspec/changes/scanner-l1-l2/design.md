# Design — scanner-l1-l2

## Approach

Layers 1-2 of the AI Scanner are purely deterministic — rule-based thresholds and closed-form indicator math, no ML. They are implemented as a self-contained Go package operating over a lightweight, already-assembled `ScanInput` contract, decoupled from any live Feed integration. This keeps the whole engine testable against hand-built fixture candle series with hand-computed expected values (the overnight verification substitute), and keeps Go build/test as the natural gate (`G1`/`G2`), consistent with the rest of the deterministic v4 components. Layer 3 (XGBoost ranking, Python via gRPC) is a separate future change and is not touched here.

`ScanInput` is the seam: it carries everything Layer 1 and Layer 2 need (candle history, current price, 20-day average volume, market cap, days-to-next-earnings, recent-halt flag, options-chain-exists flag, short interest, sector 10-day return, regime tag). This change defines that contract and builds the engine against it; assembling a real `ScanInput` from the live Feed (Massive/Polygon), a fundamentals source, and a regime classifier is explicitly out of scope and left to a future integration change.

## Package and file layout

New package `src/go/scanner/` (package name `scanner`, import path `github.com/jiaming2012/slack-trading/src/go/scanner`):

- `input.go` — `ScanInput` struct: `Ticker`, `ScannedAt`, `Candles []Candle` (OHLCV + timestamp history, ascending order), `Price`, `AvgDailyVolume20d`, `MarketCap`, `DaysToNextEarnings *int`, `RecentHalt bool`, `HasOptionsChain bool`, `ShortInterest *float64`, `SectorMomentum10d *float64`, `RegimeTag string`, `ScannerVersion string`.
- `hard_filters.go` — `LiquidityGate(input) (pass bool, reason string)`, `DataQualityGate(input) (pass bool, reason string)`, `RegimeFilter(input) (pass bool, reason string)`, `RunLayer1(input) Layer1Result` (aggregates all three, collects every failure reason, never short-circuits after the first).
- `regime_thresholds.go` — built-in default per-regime ATR%-ceiling / price-vs-50MA-bound table (`trending`, `mean_reverting`, `high_vol`); explicitly NOT the Scanner Optimizer's dynamic `scanner_configs` payload — that hot-swappable config is out of scope here.
- `features.go` — `ComputeRSI14`, `ComputeATRPct`, `ComputePriceVs50MA`, `ComputeCompressionScore` (Bollinger Band width percentile against a trailing window), `ComputeVolumeRatio` (pace-adjusted: normalizes both today's partial volume and the 20-day baseline to the same elapsed-session-time fraction before dividing).
- `feature_vector.go` — `FeatureVector` struct mirroring the eight architecture-doc features + `RegimeTag`; `BuildFeatureVector(input) (*FeatureVector, dataAsOf time.Time, error)` — computes all features, derives `data_as_of` from the latest candle timestamp used, and enforces `data_as_of <= scanned_at` (fail closed, returns error, no partial vector).
- `pipeline.go` — `RunScan(db *gorm.DB, input ScanInput) (*tradingstack.ScanResult, error)`: runs `RunLayer1`; on failure returns `(nil, ErrFilteredOut{reasons})` and performs no feature computation or write; on pass, calls `BuildFeatureVector`, maps it onto a `tradingstack.ScanResult`, stamps `scanner_version`, and `db.Create()`s it. `RunScan` only ever inserts — there is no update/mutate path for an existing row's feature values, which is how immutability-of-raw-values is enforced structurally rather than by convention.
- `errors.go` — sentinel errors: `ErrDataAsOfAfterScannedAt`, `ErrFilteredOut` (carries the Layer 1 failure reasons).
- `hard_filters_test.go`, `features_test.go`, `pipeline_test.go` — fixture-driven table tests. Candle fixtures are small, hand-built, ascending-time OHLCV series with independently hand-computed expected RSI/ATR/MA/BB values (computed once outside the code under test and hard-coded as expectations, so the tests cannot pass by mirroring a bug in the implementation). `pipeline_test.go` additionally uses a testcontainers Postgres instance (mirroring the `trading-stack-schema` test pattern) to verify persistence, the fail-closed `data_as_of` path, and the no-update-on-rerun immutability guarantee.

## Data flow

```
(future, out of scope) Feed + fundamentals + regime classifier
                │
                ▼
           ScanInput  ◄── this change's fixtures stand in for the above, overnight
                │
                ▼
        RunLayer1 (hard filters)
                │
        fail ───┴─── pass
         │             │
   (no write,           ▼
    no Layer 2)   BuildFeatureVector (Layer 2)
                       │
              fail-closed on data_as_of  ──► error, no write
                       │ pass
                       ▼
              tradingstack.ScanResult
                       │
                       ▼
             Postgres scan_results (via trading-stack-schema)
```

## Out of scope

- Layer 3 ML ranking (XGBoost scoring, `scanner_version`-tagged model weights) — separate future roadmap card `scanner-ml-ranking`.
- Populating `ScanInput` from the real Feed (Massive/Polygon), a fundamentals provider, or a regime classifier — this change consumes a pre-assembled `ScanInput`; live assembly is a future integration change.
- The Scanner Optimizer's dynamic, hot-swappable `scanner_configs` payload and its regime-conditional feature weights/thresholds — Layer 1's regime thresholds here are static, built-in defaults only.
- Any RPC (Twirp/REST), CLI command, or Temporal workflow that invokes `RunScan` on a schedule or against a live universe — no operator-facing "run a scan" entry point ships in this change.
- Point-in-time delisted-ticker universe handling (Data Layer concern, not this engine).
- The `sim_outcomes` join, labeled-training-data assembly, or anything downstream of `scan_results` (owned by `optimizer-validation-pipeline` and `scanner-ml-ranking`).

## Dependency ordering within the batch

Depends on `trading-stack-schema`: `pipeline.go` imports `tradingstack.ScanResult` and `pipeline_test.go`'s persistence test needs the six-table migration (`MigrateTradingStack`) to exist. `trading-stack-schema` must land and archive (or at minimum merge to the run branch) before this change's persistence code compiles. The hard-filter and feature-computation code (`hard_filters.go`, `features.go`, `feature_vector.go`) has no such dependency and could in principle be authored and unit-tested in parallel, but `pipeline.go` and its tests are gated on the schema change per the overnight schedule (Wave 3a → Wave 3b).

## Verification gates

- **Fixture-driven unit tests** — hard filter scenarios (liquidity/data-quality/regime, pass and every individual-condition failure), feature computations (rsi_14, atr_pct, price_vs_50ma, compression_score, volume_ratio) against hand-computed fixture expectations, pass-through feature recording, the `data_as_of <= scanned_at` fail-closed path, the no-update-on-rerun immutability guarantee, and `scanner_version` tagging — run via the new `task test:scanner` Taskfile target (`go test -count=1 ./src/go/scanner/...`).
- **G1** — `go build ./src/go/... ./cmd/...` green (new package compiles, nothing else broken).
- **G2** — `task test` green (existing backtester-api suite unaffected; `task test` is scoped to `src/go/backtester-api` and does not itself exercise the new package — that is what `test:scanner` is for).

## Deferred validation

Live-feed validation against the real Massive/Polygon universe is explicitly deferred (Tier B). This change ships the engine and its fixture-based test suite only; it does not run against, or make any network call to, a real market data feed, and no `ScanInput`-assembly adapter for a live feed is built here. That integration, along with wiring `RunScan` into a schedulable entry point, is left to a future change once `scanner-l1-l2` and its upstream dependency have landed.
