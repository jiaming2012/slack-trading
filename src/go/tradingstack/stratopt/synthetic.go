package stratopt

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
)

// syntheticBase anchors the synthetic timeline. Each group gets its own
// minute offset so timestamps never collide across groups.
var syntheticBase = time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC)

// syntheticBuilder accumulates paired pipeline rows and sim outcomes.
type syntheticBuilder struct {
	rows     []optvalidation.TrainingRow
	outcomes []tradingstack.SimOutcome
}

// add appends one paired (TrainingRow, SimOutcome) with a shared outcome id.
// RegimeConfidence 0.9 clears the pipeline's default regime filter; the zero
// DataAsOf never trips the timestamp audit.
func (b *syntheticBuilder) add(strategyID, regime, ticker string, at time.Time, pnl float64, holdDays int, exitReason string) {
	scanResultID := uuid.New()
	outcomeID := uuid.New()
	simulatedAt := at.Add(time.Hour)

	pnlCopy := pnl
	holdCopy := holdDays
	strategyCopy := strategyID

	b.rows = append(b.rows, optvalidation.TrainingRow{
		ScanResultID:     scanResultID,
		SimOutcomeID:     outcomeID,
		StrategyID:       strategyID,
		Ticker:           ticker,
		ScannedAt:        at,
		RegimeTag:        regime,
		RegimeConfidence: 0.9,
		PnlPct:           pnl,
	})
	b.outcomes = append(b.outcomes, tradingstack.SimOutcome{
		BaseModel:    tradingstack.BaseModel{ID: outcomeID},
		ScanResultID: scanResultID,
		SimulatedAt:  simulatedAt,
		StrategyID:   &strategyCopy,
		PnlPct:       &pnlCopy,
		HoldDays:     &holdCopy,
		ExitReason:   exitReason,
	})
}

// SyntheticDataset builds the in-memory fixture set used by the package
// tests and the CLI's --synthetic self-check mode. It requires no database
// and no network. All EV weights are the pipeline's neutral 1.0 default (no
// StrategyEvWeight reference rows), no fidelity periods exist (zero fidelity
// drops), and every row clears the timestamp and regime stages -- so the
// gated set equals the input set and the interesting variation is in the
// groups themselves:
//
//   - "strat-alpha"/"trend" (gate-passing stop-churn): 240 hourly outcomes in
//     a stationary 2-wins-then-1-quick-stop-loss pattern. Every loss is a
//     stop exit within 1 day, so the stop-churn rule fires; the pattern is
//     stationary across all five chronological windows, so out-of-sample
//     performance retains in-sample performance and the candidate passes the
//     gate -> pending_review.
//   - "strat-alpha"/"range" (multi-regime, insufficient): 20 outcomes --
//     below the 50-decided pre-filter, reported as insufficient-data.
//   - "strat-beta"/"chop" (gate-failing timeout-drag): 200 outcomes whose
//     losses all exit via timeout (the timeout-drag rule fires), positive
//     through the first four windows, collapsing to pure losses in the most
//     recent window -- the gate's walk-forward retention check fails on the
//     out-of-sample collapse -> rejected_by_gate.
//   - "strat-gamma"/"trend" (eligible, below rule thresholds): 60 decided
//     outcomes (plus 5 breakevens) whose quick-stop share among losses is
//     0.4 and whose other shares are 0 -- no rule fires, zero candidates.
//   - "strat-delta"/"squeeze" (insufficient despite extreme churn): 49
//     decided outcomes, all losses quick stop-outs -- one short of the
//     pre-filter, so no candidate is generated and the group is reported.
func SyntheticDataset() (optvalidation.Input, []tradingstack.SimOutcome) {
	b := &syntheticBuilder{}

	// --- strat-alpha / trend: gate-passing stop-churn group (240 rows) ---
	alphaBase := syntheticBase
	for i := 0; i < 240; i++ {
		at := alphaBase.Add(time.Duration(i) * time.Hour)
		if i%3 == 2 {
			b.add("strat-alpha", "trend", "ALPH", at, -0.01, 1, "stop")
			continue
		}
		b.add("strat-alpha", "trend", "ALPH", at, 0.02, 3, "target")
	}

	// --- strat-alpha / range: multi-regime, insufficient (20 rows) ---
	rangeBase := syntheticBase.Add(10 * time.Minute)
	for i := 0; i < 20; i++ {
		at := rangeBase.Add(time.Duration(i) * time.Hour)
		if i%2 == 0 {
			b.add("strat-alpha", "range", "ALPH", at, 0.015, 4, "target")
			continue
		}
		b.add("strat-alpha", "range", "ALPH", at, -0.012, 1, "stop")
	}

	// --- strat-beta / chop: gate-failing timeout-drag group (200 rows) ---
	betaBase := syntheticBase.Add(20 * time.Minute)
	for i := 0; i < 160; i++ {
		at := betaBase.Add(time.Duration(i) * time.Hour)
		if i%5 < 3 {
			b.add("strat-beta", "chop", "BETA", at, 0.02, 3, "target")
			continue
		}
		b.add("strat-beta", "chop", "BETA", at, -0.01, 9, "timeout")
	}
	for j := 0; j < 40; j++ {
		at := betaBase.Add(time.Duration(160+j) * time.Hour)
		b.add("strat-beta", "chop", "BETA", at, -0.015-0.0002*float64(j%5), 10, "timeout")
	}

	// --- strat-gamma / trend: eligible but below every rule threshold ---
	gammaBase := syntheticBase.Add(30 * time.Minute)
	for cycle := 0; cycle < 6; cycle++ {
		for k := 0; k < 10; k++ {
			at := gammaBase.Add(time.Duration(cycle*11+k) * time.Hour)
			switch {
			case k < 5:
				b.add("strat-gamma", "trend", "GAMA", at, 0.015, 4, "target")
			case k < 7:
				b.add("strat-gamma", "trend", "GAMA", at, -0.01, 1, "stop")
			default:
				b.add("strat-gamma", "trend", "GAMA", at, -0.012, 6, "signal_exit")
			}
		}
		// One breakeven per cycle (excluded from decided counts), plus a
		// final one below, totals 60 decided + 5 breakevens... cycles 1-5.
		if cycle < 5 {
			at := gammaBase.Add(time.Duration(cycle*11+10) * time.Hour)
			b.add("strat-gamma", "trend", "GAMA", at, 0, 3, "signal_exit")
		}
	}

	// --- strat-delta / squeeze: 49 decided, extreme stop churn ---
	deltaBase := syntheticBase.Add(40 * time.Minute)
	for i := 0; i < 49; i++ {
		at := deltaBase.Add(time.Duration(i) * time.Hour)
		if i%5 < 3 {
			b.add("strat-delta", "squeeze", "DLTA", at, -0.01, 1, "stop")
			continue
		}
		b.add("strat-delta", "squeeze", "DLTA", at, 0.02, 5, "target")
	}

	return optvalidation.Input{Rows: b.rows}, b.outcomes
}

// SyntheticRunConfig returns a fixed-clock run config so repeated synthetic
// runs print identical output (modulo generated ids).
func SyntheticRunConfig() RunConfig {
	return RunConfig{
		Now:            time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		Optimizer:      DefaultOptimizerConfig(),
		PipelineConfig: optvalidation.DefaultConfig(),
	}
}
