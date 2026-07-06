package scanneropt

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// syntheticBase anchors the synthetic timeline.
var syntheticBase = time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC)

// SyntheticWeightedRows builds the in-memory weighted training rows used by
// the package tests and the CLI's --synthetic self-check mode. It requires no
// database and no network.
//
// The set contains two regimes:
//
//   - "trending" (healthy): 800 hourly rows in a stationary pattern -- two
//     winners then one loser. Winners live at high volume_ratio (1.5-1.9),
//     low atr_pct (2.0-2.3), and high rsi_14; losers at the opposite corner
//     (volume_ratio < 1.0, atr_pct >= 4.0). Because the pattern is
//     stationary, per-fold re-derivations land on near-identical parameters
//     and out-of-sample performance retains in-sample performance, so a
//     full-set run passes the overfitting gate under its default config.
//   - "mean_reverting" (thin): 12 rows early in the timeline, a third of
//     them breakeven -- far fewer decided samples than the baseline's
//     min_labeled_samples, so the tuner derives no model for it.
//
// All EV weights are 1.0 (the pipeline's neutral default before EV history
// accrues).
func SyntheticWeightedRows() []optvalidation.WeightedTrainingRow {
	rows := make([]optvalidation.WeightedTrainingRow, 0, 812)

	newRow := func(regime, ticker string, at time.Time, pnl, rsi, vr, atr float64) optvalidation.WeightedTrainingRow {
		return optvalidation.WeightedTrainingRow{
			TrainingRow: optvalidation.TrainingRow{
				ScanResultID: uuid.New(),
				SimOutcomeID: uuid.New(),
				StrategyID:   "strategy-synthetic",
				Ticker:       ticker,
				ScannedAt:    at,
				RegimeTag:    regime,
				RSI14:        rsi,
				VolumeRatio:  vr,
				ATRPct:       atr,
				PnlPct:       pnl,
			},
			EVWeight: 1.0,
		}
	}

	winIdx := 0
	for i := 0; i < 800; i++ {
		at := syntheticBase.Add(time.Duration(i) * time.Hour)
		if i%3 == 2 {
			rows = append(rows, newRow("trending", "TRND", at,
				-0.01,
				40+float64(i%5),
				0.8+0.05*float64(i%4),
				4.0+0.1*float64(i%4)))
			continue
		}
		rows = append(rows, newRow("trending", "TRND", at,
			0.015+0.005*float64(winIdx%3),
			60+float64(winIdx%5),
			1.5+0.1*float64(winIdx%5),
			2.0+0.1*float64(winIdx%4)))
		winIdx++
	}

	for j := 0; j < 12; j++ {
		at := syntheticBase.Add(time.Duration(j*30)*time.Hour + 30*time.Minute)
		pnl := 0.0
		switch j % 3 {
		case 1:
			pnl = 0.015
		case 2:
			pnl = -0.02
		}
		rows = append(rows, newRow("mean_reverting", "MNRV", at,
			pnl,
			50+float64(j%7),
			1.2+0.05*float64(j%4),
			2.5+0.1*float64(j%3)))
	}

	return rows
}

// SyntheticBaseline is the baseline payload for the synthetic mode: no
// per-regime models yet (mirroring a first-ever optimizer run) and a
// min_labeled_samples small enough that the walk-forward folds' train
// prefixes re-derive the healthy regime.
func SyntheticBaseline() scannercfg.Payload {
	return scannercfg.Payload{
		Version:      "synthetic-baseline-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{},
		Global: scannercfg.GlobalConfig{
			TopNCandidates:    20,
			MinLabeledSamples: 40,
		},
	}
}

// SyntheticCycleConfig returns a fixed-clock cycle config so repeated
// synthetic runs print identical output (modulo generated ids).
func SyntheticCycleConfig() CycleConfig {
	return CycleConfig{
		Now:        time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		RunID:      "synthetic-run",
		Version:    "synthetic-proposal-v1",
		GateConfig: overfitting.DefaultConfig(),
	}
}
