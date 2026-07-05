package scanner

import "time"

// FeatureVector mirrors the eight architecture-doc features computed by
// Layer 2, plus the identifying/provenance fields every persisted row needs.
// Individual computed features are nil when there was insufficient history
// to compute them faithfully (see features.go) -- this alone does not block
// persistence; only the data_as_of <= scanned_at invariant does.
type FeatureVector struct {
	Ticker    string
	ScannedAt time.Time
	Price     float64

	Rsi14             *float64
	VolumeRatio       *float64
	AtrPct            *float64
	PriceVs50MA       *float64
	CompressionScore  *float64
	ShortInterest     *float64
	SectorMomentum10d *float64
	RegimeTag         string

	// DataAsOf is derived from the latest candle timestamp used to compute
	// this vector and must be <= ScannedAt.
	DataAsOf time.Time

	// ScannerVersion identifies the Layer 1+2 code version that produced this
	// vector.
	ScannerVersion string
}

// BuildFeatureVector computes all eight architecture-doc features from a
// ScanInput, derives data_as_of from the latest candle timestamp used, and
// enforces the data_as_of <= scanned_at invariant. On violation it fails
// closed: it returns a non-nil error and no (partial) vector.
func BuildFeatureVector(input ScanInput) (*FeatureVector, error) {
	dataAsOf := input.ScannedAt
	if n := len(input.Candles); n > 0 {
		dataAsOf = input.Candles[n-1].Timestamp
	}

	if dataAsOf.After(input.ScannedAt) {
		return nil, ErrDataAsOfAfterScannedAt
	}

	closes := closesFromCandles(input.Candles)

	fv := &FeatureVector{
		Ticker:            input.Ticker,
		ScannedAt:         input.ScannedAt,
		Price:             input.Price,
		Rsi14:             ComputeRSI14(closes),
		VolumeRatio:       ComputeVolumeRatio(input),
		AtrPct:            ComputeATRPct(input.Candles, input.Price),
		PriceVs50MA:       ComputePriceVs50MA(closes, input.Price),
		CompressionScore:  ComputeCompressionScore(closes),
		ShortInterest:     input.ShortInterest,
		SectorMomentum10d: input.SectorMomentum10d,
		RegimeTag:         input.RegimeTag,
		DataAsOf:          dataAsOf,
		ScannerVersion:    input.ScannerVersion,
	}

	return fv, nil
}
