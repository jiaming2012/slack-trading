# trading-stack-schema — Delta for widen-scan-results-columns

## MODIFIED Requirements

### Requirement: scan_results model with data_as_of invariant

The system SHALL provide a `ScanResult` model mapped to table `scan_results` with fields faithful to the architecture SQL: `id` (UUID primary key, populated on create when unset), `scanned_at` (not null), `ticker` (not null), `regime_tag`, `regime_confidence`, `price`, `volume_ratio`, `rsi_14`, `atr_pct`, `price_vs_50ma`, `compression_score`, `short_interest`, `sector_momentum`, `sector`, `scanner_score`, `scanner_version`, and `data_as_of`. The `price_vs_50ma`, `compression_score`, and `sector_momentum` columns SHALL be nullable numeric columns added additively through the existing tradingstack migration path (`MigrateTradingStack`), idempotently, without altering or rewriting any existing column or row — rows persisted before these columns existed SHALL simply read back NULL for them. The system SHALL enforce the invariant `data_as_of <= scanned_at` both by Go validation before persist (returning a sentinel error) and by a database CHECK constraint, so a violating row cannot be stored.

#### Scenario: Valid scan_result round-trips

- **WHEN** a `ScanResult` with `data_as_of` earlier than or equal to `scanned_at` is saved and re-read by id
- **THEN** the read-back row equals the written row field-for-field, including a non-zero generated `id`

#### Scenario: data_as_of after scanned_at is rejected

- **WHEN** a `ScanResult` whose `data_as_of` is strictly after its `scanned_at` is saved
- **THEN** the save returns a non-nil error
- **AND** no row is written to `scan_results`

#### Scenario: The three widened feature columns round-trip including NULL

- **WHEN** one `ScanResult` is saved with `price_vs_50ma` 2.5, `compression_score` 37.0, and `sector_momentum` 0.04, and another is saved with all three unset, and both are re-read by id
- **THEN** the first row reads back exactly those three values from columns named `price_vs_50ma`, `compression_score`, and `sector_momentum`, and the second row reads back NULL for all three

#### Scenario: Widening migration is additive and idempotent

- **WHEN** `MigrateTradingStack` runs against a database whose `scan_results` table predates the three widened columns, and then runs a second time
- **THEN** the three columns exist as nullable numeric columns, every pre-existing row is preserved with NULL in them, and the second run returns a nil error leaving the schema unchanged
