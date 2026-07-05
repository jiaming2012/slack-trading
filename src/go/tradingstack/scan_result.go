package tradingstack

import (
	"time"

	"gorm.io/gorm"
)

// ScanResult mirrors the scan_results table from the v4 trading-stack schema.
// Nullable NUMERIC/TEXT/TIMESTAMPTZ columns use pointer types so NULL
// round-trips faithfully; scanned_at and ticker are NOT NULL.
type ScanResult struct {
	BaseModel
	ScannedAt        time.Time  `gorm:"column:scanned_at;type:timestamptz;not null"`
	Ticker           string     `gorm:"column:ticker;type:text;not null"`
	RegimeTag        *string    `gorm:"column:regime_tag;type:text"`
	RegimeConfidence *float64   `gorm:"column:regime_confidence;type:numeric"`
	Price            *float64   `gorm:"column:price;type:numeric"`
	VolumeRatio      *float64   `gorm:"column:volume_ratio;type:numeric"`
	Rsi14            *float64   `gorm:"column:rsi_14;type:numeric"`
	AtrPct           *float64   `gorm:"column:atr_pct;type:numeric"`
	ShortInterest    *float64   `gorm:"column:short_interest;type:numeric"`
	Sector           *string    `gorm:"column:sector;type:text"`
	ScannerScore     *float64   `gorm:"column:scanner_score;type:numeric"`
	ScannerVersion   *string    `gorm:"column:scanner_version;type:text"`
	DataAsOf         *time.Time `gorm:"column:data_as_of;type:timestamptz"`

	// FeedHealth is an additive, nullable tag set by the feedhealth package's
	// TagScanResult helper to the string form of the computed
	// FeedHealthStatus ("healthy"/"degraded"/"stale") for this scan result's
	// asset class at scan time. Owned by the feed-health-staleness change;
	// no existing column, tag, or constraint on this model is altered.
	FeedHealth *string `gorm:"column:feed_health;type:text"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (ScanResult) TableName() string {
	return "scan_results"
}

// BeforeSave enforces the data_as_of <= scanned_at invariant in Go before the
// row reaches the database CHECK constraint. NULL data_as_of is allowed.
func (s *ScanResult) BeforeSave(tx *gorm.DB) error {
	if s.DataAsOf != nil && s.DataAsOf.After(s.ScannedAt) {
		return ErrScanDataAsOfAfterScannedAt
	}
	return nil
}
