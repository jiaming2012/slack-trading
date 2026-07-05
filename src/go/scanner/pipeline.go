package scanner

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// RunScan runs Layer 1 hard filters against input; on failure it returns
// (nil, ErrFilteredOut{reasons}) and performs no feature computation or
// write. On pass, it builds the Layer 2 feature vector, maps it onto a
// tradingstack.ScanResult, stamps scanner_version, and persists it via
// db.Create(). RunScan only ever inserts -- there is no update/mutate path
// for an existing row's feature values, which is how immutability-of-raw-
// values is enforced structurally rather than by convention.
//
// NOTE: the v4 trading-stack scan_results schema (owned by the
// trading-stack-schema change, not modified here) does not carry dedicated
// columns for price_vs_50ma, compression_score, or sector_momentum -- only
// regime_tag, regime_confidence, price, volume_ratio, rsi_14, atr_pct,
// short_interest, sector, scanner_score, scanner_version, and data_as_of are
// columns on scan_results. BuildFeatureVector still computes all eight
// architecture-doc features (available on the in-memory FeatureVector for
// any caller that wants them), but only the fields with a corresponding
// scan_results column are persisted here. Widening the schema to carry the
// remaining three is left to a future change.
func RunScan(db *gorm.DB, input ScanInput) (*tradingstack.ScanResult, error) {
	layer1 := RunLayer1(input)
	if !layer1.Pass {
		return nil, ErrFilteredOut{Reasons: layer1.Reasons}
	}

	fv, err := BuildFeatureVector(input)
	if err != nil {
		return nil, fmt.Errorf("RunScan: failed to build feature vector: %w", err)
	}

	price := fv.Price
	dataAsOf := fv.DataAsOf

	sr := &tradingstack.ScanResult{
		ScannedAt:      fv.ScannedAt,
		Ticker:         fv.Ticker,
		RegimeTag:      nilIfEmpty(fv.RegimeTag),
		Price:          &price,
		VolumeRatio:    fv.VolumeRatio,
		Rsi14:          fv.Rsi14,
		AtrPct:         fv.AtrPct,
		ShortInterest:  fv.ShortInterest,
		ScannerVersion: nilIfEmpty(fv.ScannerVersion),
		DataAsOf:       &dataAsOf,
	}

	if err := db.Create(sr).Error; err != nil {
		return nil, fmt.Errorf("RunScan: failed to persist scan result: %w", err)
	}

	return sr, nil
}

// nilIfEmpty returns nil for an empty string, else a pointer to s -- keeping
// optional string fields NULL in the database rather than empty-string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
