# scanner-hard-filters

## ADDED Requirements

### Requirement: Liquidity gate

The system SHALL provide a liquidity gate that admits a ticker to Layer 2 only when all three conditions hold strictly: 20-day average daily volume `> 500,000` shares, current price `> $5`, and market capitalization `> $300,000,000`. A ticker failing any one of the three conditions SHALL be rejected by the liquidity gate.

#### Scenario: Ticker passes when all three liquidity thresholds are exceeded

- **WHEN** a `ScanInput` has average daily volume 800,000, price $12.50, and market cap $1,200,000,000
- **THEN** the liquidity gate reports pass

#### Scenario: Ticker fails on average daily volume at or below the floor

- **WHEN** a `ScanInput` has average daily volume 500,000 (equal to the floor) with price and market cap otherwise passing
- **THEN** the liquidity gate reports fail with a reason identifying the volume condition

#### Scenario: Ticker fails on price at or below the floor

- **WHEN** a `ScanInput` has price $5.00 (equal to the floor) with volume and market cap otherwise passing
- **THEN** the liquidity gate reports fail with a reason identifying the price condition

#### Scenario: Ticker fails on market cap at or below the floor

- **WHEN** a `ScanInput` has market cap $300,000,000 (equal to the floor) with volume and price otherwise passing
- **THEN** the liquidity gate reports fail with a reason identifying the market-cap condition

### Requirement: Data quality gate

The system SHALL provide a data quality gate that admits a ticker to Layer 2 only when all three conditions hold: no earnings announcement is scheduled within 3 calendar days of `scanned_at` (before or after), the ticker has no recent trading halt flagged on the input, and an options chain is confirmed to exist for the ticker. These three signals arrive on `ScanInput` as already-derived fields (`DaysToNextEarnings`, `RecentHalt`, `HasOptionsChain`); computing them from a live feed is out of scope for this change.

#### Scenario: Ticker passes when all three data-quality conditions hold

- **WHEN** a `ScanInput` has `DaysToNextEarnings` of 10, `RecentHalt` false, and `HasOptionsChain` true
- **THEN** the data quality gate reports pass

#### Scenario: Ticker fails when earnings fall within the 3-day window

- **WHEN** a `ScanInput` has `DaysToNextEarnings` of 2
- **THEN** the data quality gate reports fail with a reason identifying the earnings condition

#### Scenario: Ticker fails on a recent halt

- **WHEN** a `ScanInput` has `RecentHalt` true with earnings and options chain otherwise passing
- **THEN** the data quality gate reports fail with a reason identifying the halt condition

#### Scenario: Ticker fails when no options chain exists

- **WHEN** a `ScanInput` has `HasOptionsChain` false with earnings and halt otherwise passing
- **THEN** the data quality gate reports fail with a reason identifying the options-chain condition

### Requirement: Regime-conditional filter

The system SHALL provide a regime filter that applies an ATR% ceiling and a price-vs-50-day-moving-average bound whose thresholds vary by the input's `RegimeTag` (`trending`, `mean_reverting`, or `high_vol`), using built-in default thresholds per regime. A ticker whose `RegimeTag` is empty or unrecognized SHALL NOT be rejected by the regime filter — the filter is a no-op when no regime classification is available.

#### Scenario: High-volatility regime rejects a ticker above the tighter ATR ceiling

- **WHEN** a `ScanInput` has `RegimeTag` `high_vol` and an ATR% above the `high_vol` ceiling, with liquidity and data-quality otherwise passing
- **THEN** the regime filter reports fail with a reason identifying the ATR% condition

#### Scenario: Same ATR% passes under a regime with a looser ceiling

- **WHEN** a `ScanInput` has the same ATR% as the previous scenario but `RegimeTag` `trending` (looser ceiling)
- **THEN** the regime filter reports pass

#### Scenario: Missing regime tag does not cause rejection

- **WHEN** a `ScanInput` has an empty `RegimeTag` and would otherwise fail a regime-specific ATR%/MA bound if one were applied
- **THEN** the regime filter reports pass, since no regime-specific threshold applies

### Requirement: Layer 1 elimination gates Layer 2

Layer 1 SHALL run before Layer 2 for every ticker. A ticker rejected by any Layer 1 gate (liquidity, data quality, or regime) SHALL NOT proceed to Layer 2 feature extraction and SHALL NOT produce a `scan_results` row. The overall Layer 1 result SHALL report every failing gate, not just the first one encountered.

#### Scenario: A ticker failing Layer 1 produces no scan_results row and skips feature extraction

- **WHEN** a `ScanInput` fails the liquidity gate
- **THEN** running the full pipeline for that ticker returns a rejection result and performs no feature computation and no database write

#### Scenario: A ticker failing multiple gates reports all of them

- **WHEN** a `ScanInput` fails both the liquidity gate (price too low) and the data quality gate (recent halt)
- **THEN** the Layer 1 result lists both failure reasons, not only the first one evaluated
