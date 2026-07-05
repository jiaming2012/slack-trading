// Package scanner implements Layers 1-2 of the AI Scanner: deterministic hard
// filters (Layer 1) and feature extraction (Layer 2). It operates over a
// pre-assembled ScanInput contract and is deliberately decoupled from any live
// Feed integration, fundamentals provider, or regime classifier -- assembling
// a real ScanInput from those sources is a future integration change. Layer 3
// (ML ranking) is also out of scope here.
package scanner

import "time"

// Candle is a single OHLCV bar with its timestamp. Callers supply candles in
// ascending time order. The last element may represent the current, still-
// in-progress trading session (a partial "today" bar) whose Close mirrors the
// current price and whose Volume is the volume observed so far in the
// session; this is what makes pace-adjusted intraday features (volume_ratio)
// possible without a dedicated live-feed adapter.
type Candle struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// ScanInput carries everything Layer 1 and Layer 2 need for a single ticker.
// It is the seam between this engine and whatever assembles it (a future
// change wiring in the live Feed, a fundamentals source, and a regime
// classifier).
type ScanInput struct {
	// Ticker is the symbol being scanned.
	Ticker string

	// ScannedAt is the timestamp this scan cycle is evaluating the ticker as
	// of. Every derived data_as_of value must be <= ScannedAt.
	ScannedAt time.Time

	// Candles is the ticker's OHLCV history in ascending time order. The last
	// entry may be a partial "today" bar (see Candle).
	Candles []Candle

	// Price is the current (or latest available) trade price.
	Price float64

	// AvgDailyVolume20d is the 20-day average daily (full-session) volume.
	AvgDailyVolume20d float64

	// MarketCap is the ticker's current market capitalization in dollars.
	MarketCap float64

	// DaysToNextEarnings is the signed number of calendar days to the next
	// scheduled earnings announcement (negative if the most recent earnings
	// announcement was that many days ago). Nil means no earnings date is
	// known/scheduled.
	DaysToNextEarnings *int

	// RecentHalt is true when the ticker has a recent trading halt flagged.
	RecentHalt bool

	// HasOptionsChain is true when an options chain is confirmed to exist for
	// the ticker.
	HasOptionsChain bool

	// ShortInterest is recorded verbatim on the feature vector (pass-through).
	ShortInterest *float64

	// SectorMomentum10d is the ticker's sector ETF's 10-day return, recorded
	// verbatim on the feature vector (pass-through).
	SectorMomentum10d *float64

	// RegimeTag classifies the current market regime ("trending",
	// "mean_reverting", "high_vol") and gates the regime-conditional filter.
	// Empty or unrecognized values make the regime filter a no-op.
	RegimeTag string

	// ScannerVersion identifies the Layer 1+2 code version producing this
	// scan, stamped onto every persisted scan_results row.
	ScannerVersion string
}
