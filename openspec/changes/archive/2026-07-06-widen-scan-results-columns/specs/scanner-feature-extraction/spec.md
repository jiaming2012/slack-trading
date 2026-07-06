# scanner-feature-extraction — Delta for widen-scan-results-columns

## MODIFIED Requirements

### Requirement: scanner_version tagging and persistence

Every `scan_results` row written by this pipeline SHALL be tagged with the `scanner_version` string identifying the Layer 1+2 code version that produced it, and SHALL be persisted as a `tradingstack.ScanResult` via GORM using the model from `trading-stack-schema`.

All eight architecture-doc features (`rsi_14`, `volume_ratio`, `atr_pct`, `price_vs_50ma`, `compression_score`, `short_interest`, `sector_momentum`, `regime_tag`) SHALL be COMPUTED on the in-memory `FeatureVector` for every ticker that passes Layer 1, and — now that `trading-stack-schema` carries dedicated columns for all eight — a ticker that passes Layer 1 and completes Layer 2 SHALL result in exactly one `scan_results` row PERSISTING all eight features plus `scanner_version` and `data_as_of`. A feature that is nil on the `FeatureVector` (insufficient history, per the never-fabricate rules) SHALL persist as NULL in its column, never as a fabricated value. Rows persisted before the schema was widened retain NULL in the three late-added columns (`price_vs_50ma`, `compression_score`, `sector_momentum`) and SHALL NOT be backfilled or rewritten, per the raw-values-are-immutable requirement.

#### Scenario: Two runs configured with different scanner_version values are tagged distinctly

- **WHEN** feature extraction is run for two different tickers with the pipeline configured with `scanner_version` `"l1l2-v1"` and `"l1l2-v2"` respectively
- **THEN** each resulting `scan_results` row's `scanner_version` matches the value configured for its run

#### Scenario: A qualifying ticker produces exactly one row persisting all eight features

- **WHEN** a `ScanInput` passes all Layer 1 gates and has sufficient history for every Layer 2 feature
- **THEN** exactly one `scan_results` row is written via `tradingstack.ScanResult`, populated with `rsi_14`, `volume_ratio`, `atr_pct`, `price_vs_50ma`, `compression_score`, `short_interest`, `sector_momentum`, `regime_tag`, `scanner_version`, and `data_as_of`
- **AND** each persisted feature value equals the corresponding value on the in-memory `FeatureVector` built for that input

#### Scenario: A nil feature persists as NULL, not a fabricated value

- **WHEN** a `ScanInput` passes Layer 1 but has fewer than 50 closes, so `price_vs_50ma` is nil on the `FeatureVector`
- **THEN** the persisted row's `price_vs_50ma` column is NULL while the other computed features persist normally
