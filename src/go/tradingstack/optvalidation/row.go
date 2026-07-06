package optvalidation

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// TrainingRow is the in-memory join of a tradingstack.ScanResult with its
// linked tradingstack.SimOutcome -- the actual unit the Scanner Optimizer
// trains on. Every pipeline stage operates on slices of TrainingRow (or a
// stage's own output type) and performs no database I/O.
type TrainingRow struct {
	ScanResultID uuid.UUID
	SimOutcomeID uuid.UUID

	StrategyID       string
	Ticker           string
	ScannedAt        time.Time
	DataAsOf         time.Time
	RegimeTag        string
	RegimeConfidence float64
	Price            float64
	VolumeRatio      float64
	RSI14            float64
	ATRPct           float64
	ScannerScore     float64

	// PnlPct and OutcomeLabel are carried from the joined SimOutcome for the
	// scanner optimizer's win/loss labeling. No pipeline stage reads or
	// alters them. A nil source PnlPct maps to 0 (landing in the optimizer's
	// excluded breakeven class); a nil source OutcomeLabel maps to the empty
	// string.
	PnlPct       float64
	OutcomeLabel string
}

// WeightedTrainingRow is a TrainingRow with the ev_weight attached by the
// EV-weight-join stage (stage 5).
type WeightedTrainingRow struct {
	TrainingRow
	EVWeight float64
}

// NewTrainingRow joins a ScanResult with its linked SimOutcome into a
// TrainingRow. Nullable source fields default to their Go zero value when
// nil -- in particular a nil DataAsOf becomes the zero time.Time, which is
// always <= ScannedAt and so never trips the timestamp-audit stage, mirroring
// tradingstack.ScanResult's own "NULL data_as_of is allowed" semantics. A nil
// RegimeConfidence becomes 0.0, which is strictly less than the regime-
// confidence-filter stage's default threshold (0.7) and so the row is
// dropped by that stage rather than by this constructor -- a scan result
// with no recorded regime confidence is treated as untrusted, not as
// confidently in-regime.

func NewTrainingRow(sr tradingstack.ScanResult, so tradingstack.SimOutcome) TrainingRow {
	return TrainingRow{
		ScanResultID: sr.ID,
		SimOutcomeID: so.ID,

		StrategyID:       derefString(so.StrategyID),
		Ticker:           sr.Ticker,
		ScannedAt:        sr.ScannedAt,
		DataAsOf:         derefTime(sr.DataAsOf),
		RegimeTag:        derefString(sr.RegimeTag),
		RegimeConfidence: derefFloat64(sr.RegimeConfidence),
		Price:            derefFloat64(sr.Price),
		VolumeRatio:      derefFloat64(sr.VolumeRatio),
		RSI14:            derefFloat64(sr.Rsi14),
		ATRPct:           derefFloat64(sr.AtrPct),
		ScannerScore:     derefFloat64(sr.ScannerScore),

		PnlPct:       derefFloat64(so.PnlPct),
		OutcomeLabel: derefString(so.OutcomeLabel),
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}
