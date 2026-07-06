package shadowdeploy

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// Observation is one per-ticker scan snapshot lifted from a persisted
// scan_results row -- the unit of input the decision engine evaluates.
// Nullable columns stay pointers so NULL never masquerades as a zero value:
// admission and scoring treat a nil feature explicitly (see EvaluateConfig).
type Observation struct {
	// ScanResultID is the source scan_results row id, kept for outcome
	// joins (sim_outcomes.scan_result_id) and divergence drill-down.
	ScanResultID uuid.UUID

	Ticker    string
	ScannedAt time.Time

	// RegimeTag selects the payload's regime model. A nil tag resolves to
	// the empty regime name, which no payload models -- the decision is
	// marked no_model.
	RegimeTag *string

	// The scoreable / filterable features, in the fixed payload
	// feature-name vocabulary: volume_ratio, rsi_14, atr_pct,
	// scanner_score.
	VolumeRatio  *float64
	RSI14        *float64
	ATRPct       *float64
	ScannerScore *float64
}

// Regime returns the observation's regime tag, or "" when the row has none.
func (o Observation) Regime() string {
	if o.RegimeTag == nil {
		return ""
	}
	return *o.RegimeTag
}

// ObservationFromScanResult lifts a persisted scan_results row into the
// engine's input shape. It copies pointer values so the engine can never
// alias (and therefore never mutate) the loaded row.
func ObservationFromScanResult(sr tradingstack.ScanResult) Observation {
	return Observation{
		ScanResultID: sr.ID,
		Ticker:       sr.Ticker,
		ScannedAt:    sr.ScannedAt,
		RegimeTag:    copyStringPtr(sr.RegimeTag),
		VolumeRatio:  copyFloatPtr(sr.VolumeRatio),
		RSI14:        copyFloatPtr(sr.Rsi14),
		ATRPct:       copyFloatPtr(sr.AtrPct),
		ScannerScore: copyFloatPtr(sr.ScannerScore),
	}
}

func copyFloatPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func copyStringPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
