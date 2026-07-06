package scanner

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// RunScan runs Layer 1 hard filters against input; on failure it returns
// (nil, ErrFilteredOut{reasons}) and performs no feature computation or
// write. On pass, it builds the Layer 2 feature vector, maps all eight
// architecture-doc features onto a tradingstack.ScanResult, stamps
// scanner_version, and persists it via db.Create(). A feature that is nil on
// the FeatureVector (insufficient history, per the never-fabricate rules)
// persists as NULL in its column. RunScan only ever inserts -- there is no
// update/mutate path for an existing row's feature values, which is how
// immutability-of-raw-values is enforced structurally rather than by
// convention.
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
		ScannedAt:        fv.ScannedAt,
		Ticker:           fv.Ticker,
		RegimeTag:        nilIfEmpty(fv.RegimeTag),
		Price:            &price,
		VolumeRatio:      fv.VolumeRatio,
		Rsi14:            fv.Rsi14,
		AtrPct:           fv.AtrPct,
		PriceVs50MA:      fv.PriceVs50MA,
		CompressionScore: fv.CompressionScore,
		ShortInterest:    fv.ShortInterest,
		SectorMomentum:   fv.SectorMomentum10d,
		ScannerVersion:   nilIfEmpty(fv.ScannerVersion),
		DataAsOf:         &dataAsOf,
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
