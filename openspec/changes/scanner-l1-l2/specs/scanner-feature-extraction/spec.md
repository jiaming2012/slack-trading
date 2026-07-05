# scanner-feature-extraction

## ADDED Requirements

### Requirement: rsi_14 momentum feature

The system SHALL compute `rsi_14` as the standard 14-period Relative Strength Index over the ticker's close-price history. When fewer than 15 closes (14 price changes) are available, `rsi_14` SHALL be left unset (nil) rather than a fabricated value, and this SHALL NOT by itself block Layer 1 admission or feature-vector persistence.

#### Scenario: rsi_14 matches a hand-computed fixture value

- **WHEN** feature extraction runs against a fixture candle series with a known, hand-computed RSI-14 value
- **THEN** the computed `rsi_14` equals the expected value within a small numeric tolerance

#### Scenario: Insufficient history yields no fabricated value

- **WHEN** feature extraction runs against a fixture candle series with fewer than 15 closes
- **THEN** `rsi_14` is nil in the resulting feature vector

### Requirement: atr_pct volatility feature

The system SHALL compute `atr_pct` as the 14-period Average True Range divided by current price. When fewer than 15 candles are available, `atr_pct` SHALL be left unset (nil).

#### Scenario: atr_pct matches a hand-computed fixture value

- **WHEN** feature extraction runs against a fixture candle series with a known ATR(14) and a known current price
- **THEN** the computed `atr_pct` equals `ATR(14) / price` within a small numeric tolerance

### Requirement: price_vs_50ma trend feature

The system SHALL compute `price_vs_50ma` as the percentage the current price is above or below the 50-day simple moving average of close prices. When fewer than 50 closes are available, `price_vs_50ma` SHALL be left unset (nil).

#### Scenario: price_vs_50ma matches a hand-computed fixture value

- **WHEN** feature extraction runs against a fixture candle series of at least 50 closes with a known 50-day moving average
- **THEN** the computed `price_vs_50ma` equals the expected signed percentage within a small numeric tolerance

#### Scenario: Insufficient history yields no fabricated value

- **WHEN** feature extraction runs against a fixture candle series with fewer than 50 closes
- **THEN** `price_vs_50ma` is nil in the resulting feature vector

### Requirement: compression_score Bollinger Band width percentile

The system SHALL compute `compression_score` as the percentile rank (0-100) of the current Bollinger Band width against a trailing historical distribution of Bollinger Band widths for the same ticker. A narrower-than-typical current band width SHALL rank lower than a wider-than-typical one.

#### Scenario: compression_score matches a hand-computed fixture percentile

- **WHEN** feature extraction runs against a fixture candle series whose current Bollinger Band width and trailing width distribution are known
- **THEN** the computed `compression_score` equals the expected percentile rank within a small numeric tolerance

### Requirement: volume_ratio pace-adjusted intraday feature

The system SHALL compute `volume_ratio` as today's volume-so-far normalized to the same elapsed-session-time fraction, divided by the 20-day average volume observed at that same time-of-day fraction — so a partial trading day is compared against a like-for-like partial baseline rather than the full-day 20-day average. This prevents a ticker from appearing anomalously high- or low-volume purely because less of the trading day has elapsed.

#### Scenario: volume_ratio near 1.0 for pace-typical partial-day volume

- **WHEN** a fixture provides a ticker whose volume 30 minutes into the session is exactly proportional to its 20-day average volume observed at that same 30-minutes-in mark
- **THEN** the computed `volume_ratio` is approximately 1.0, not artificially depressed by comparing partial-day volume to a full-day average

#### Scenario: volume_ratio correctly flags a genuine early-session spike

- **WHEN** a fixture provides a ticker whose volume 30 minutes into the session is double its pace-adjusted 20-day average at that mark
- **THEN** the computed `volume_ratio` is approximately 2.0

### Requirement: Pass-through features recorded verbatim

The system SHALL record `short_interest`, `sector_momentum` (sector ETF 10-day return), and `regime_tag` on the feature vector exactly as provided on `ScanInput`, without recomputation or transformation. Computing these values from a live feed is out of scope for this change.

#### Scenario: Pass-through features are recorded unchanged

- **WHEN** a `ScanInput` provides `short_interest` 0.18, `sector_momentum` 0.04, and `regime_tag` `trending`
- **THEN** the resulting feature vector has `short_interest` 0.18, `sector_momentum` 0.04, and `regime_tag` `trending`

### Requirement: data_as_of accuracy invariant

Every feature vector SHALL be stamped with a single `data_as_of` timestamp reflecting the timestamp of the underlying market data used to compute it, and `data_as_of` MUST be `<= scanned_at`. When the underlying data's timestamp would make `data_as_of` exceed `scanned_at`, feature extraction SHALL fail closed: it SHALL return an error and SHALL NOT write a `scan_results` row, rather than clamping or silently correcting the timestamp.

#### Scenario: Valid data_as_of produces a persisted row

- **WHEN** a `ScanInput`'s latest candle timestamp is at or before `scanned_at`
- **THEN** feature extraction succeeds and the resulting `scan_results` row's `data_as_of` equals the latest candle timestamp

#### Scenario: data_as_of after scanned_at fails closed

- **WHEN** a `ScanInput`'s latest candle timestamp is strictly after `scanned_at`
- **THEN** feature extraction returns a non-nil error and no `scan_results` row is written

### Requirement: Raw values are immutable once recorded

Feature values written to a `scan_results` row at scan time SHALL never be recomputed or overwritten retroactively. The package SHALL expose no operation that mutates the feature values of an already-persisted `scan_results` row; re-running extraction against the same or updated input for a ticker/`scanned_at` that already has a row SHALL only ever produce a new, independent row (or no write), never an update to the existing one.

#### Scenario: Re-running extraction does not alter a previously persisted row

- **WHEN** feature extraction is run once for a ticker at a given `scanned_at` and persisted, and then run a second time for the same ticker and `scanned_at` with different input values
- **THEN** the first row's stored feature values are unchanged after the second run completes

### Requirement: scanner_version tagging and persistence

Every `scan_results` row written by this pipeline SHALL be tagged with the `scanner_version` string identifying the Layer 1+2 code version that produced it, and SHALL be persisted as a `tradingstack.ScanResult` via GORM using the model from `trading-stack-schema`. A ticker that passes Layer 1 and completes Layer 2 SHALL result in exactly one `scan_results` row containing all extracted features, `regime_tag`, `scanner_version`, and `data_as_of`.

#### Scenario: Two runs configured with different scanner_version values are tagged distinctly

- **WHEN** feature extraction is run for two different tickers with the pipeline configured with `scanner_version` `"l1l2-v1"` and `"l1l2-v2"` respectively
- **THEN** each resulting `scan_results` row's `scanner_version` matches the value configured for its run

#### Scenario: A qualifying ticker produces exactly one fully populated row

- **WHEN** a `ScanInput` passes all Layer 1 gates and has sufficient history for every Layer 2 feature
- **THEN** exactly one `scan_results` row is written via `tradingstack.ScanResult`, populated with all eight features, `regime_tag`, `scanner_version`, and `data_as_of`
