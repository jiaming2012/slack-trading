package shadowdeploy

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// syntheticScannedAt anchors the synthetic observation batch in time.
var syntheticScannedAt = time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

// SyntheticFixture is the built-in fixture pair shared by the package tests
// and the CLI's --synthetic self-check mode: an active payload, a divergent
// shadow payload, one observation batch, and pre-existing sim outcomes for a
// subset of the tickers (so the coverage-explicit outcome comparison is
// demonstrated with less-than-full coverage). It requires no database and no
// network.
type SyntheticFixture struct {
	ActivePayload scannercfg.Payload
	ShadowPayload scannercfg.Payload
	Observations  []Observation

	// OutcomesByScanResult joins a subset of the observations to synthetic
	// sim outcomes, keyed the way RunShadow's loader keys real rows.
	OutcomesByScanResult map[uuid.UUID][]tradingstack.SimOutcome
}

// syntheticScanResultID derives a deterministic scan result id per ticker so
// repeated synthetic runs print identical output.
func syntheticScanResultID(ticker string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("shadowdeploy-synthetic-"+ticker))
}

// Synthetic returns the built-in fixture. Hand-computed expectations (pinned
// by synthetic_test.go):
//
// Observations (regime "trending" unless noted), (volume_ratio, rsi_14, atr_pct):
//
//	AAPL 2.0, 55, 2.0    MSFT 1.8, 60, 3.0    NVDA 1.6, 85, 5.0
//	AMZN 1.4, 75, 3.5    GOOG 1.1, 90, 2.5    META 1.0, 40, 6.0
//	TSLA 0.9, 95, 7.0    AMD  2.2, 30, 4.5    NFLX nil, 70, 3.0
//	XOM  1.5, 50, 2.0 (regime "choppy": no model in either payload -> no_model)
//
// Active payload (volume-led: weights vr 0.7 / rsi 0.3, floor 1.0,
// threshold 0.35, top-4) selects {AAPL, AMD, MSFT, NVDA}.
// Shadow payload (momentum-led: weights rsi 0.7 / vr 0.3, floor 1.2,
// atr ceiling 6.0, threshold 0.35, top-4) selects {AAPL, AMZN, MSFT, NVDA}.
//
// Divergence: active-only {AMD}, shadow-only {AMZN}, both 3 -> 2 divergent
// over a union of 5 = 40%.
//
// Outcomes exist for AAPL (+2.5), MSFT (-1.0), NVDA (+4.0), AMD (+1.5), and
// GOOG (-0.5); AMZN is uncovered. Active side: coverage 4/4, win rate 3/4,
// mean pnl 1.75. Shadow side: coverage 3/4 (75%), win rate 2/3, mean pnl
// (2.5 - 1.0 + 4.0) / 3.
func Synthetic() SyntheticFixture {
	trending := "trending"
	choppy := "choppy"

	obs := func(ticker string, regime *string, vr, rsi, atr *float64) Observation {
		return Observation{
			ScanResultID: syntheticScanResultID(ticker),
			Ticker:       ticker,
			ScannedAt:    syntheticScannedAt,
			RegimeTag:    regime,
			VolumeRatio:  vr,
			RSI14:        rsi,
			ATRPct:       atr,
		}
	}
	f := func(v float64) *float64 { return &v }

	observations := []Observation{
		obs("AAPL", &trending, f(2.0), f(55), f(2.0)),
		obs("MSFT", &trending, f(1.8), f(60), f(3.0)),
		obs("NVDA", &trending, f(1.6), f(85), f(5.0)),
		obs("AMZN", &trending, f(1.4), f(75), f(3.5)),
		obs("GOOG", &trending, f(1.1), f(90), f(2.5)),
		obs("META", &trending, f(1.0), f(40), f(6.0)),
		obs("TSLA", &trending, f(0.9), f(95), f(7.0)),
		obs("AMD", &trending, f(2.2), f(30), f(4.5)),
		obs("NFLX", &trending, nil, f(70), f(3.0)),
		obs("XOM", &choppy, f(1.5), f(50), f(2.0)),
	}

	active := scannercfg.Payload{
		Version: "synthetic-active-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights: map[string]float64{
					"volume_ratio": 0.7,
					"rsi_14":       0.3,
				},
				HardFilterOverrides: scannercfg.HardFilterOverrides{
					VolumeRatioFloor: f(1.0),
				},
				ScoreThreshold: f(0.35),
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 4, MinLabeledSamples: 40},
	}

	shadow := scannercfg.Payload{
		Version: "synthetic-shadow-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights: map[string]float64{
					"rsi_14":       0.7,
					"volume_ratio": 0.3,
				},
				HardFilterOverrides: scannercfg.HardFilterOverrides{
					VolumeRatioFloor: f(1.2),
					ATRPctCeiling:    f(6.0),
				},
				ScoreThreshold: f(0.35),
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 4, MinLabeledSamples: 40},
	}

	simulatedAt := syntheticScannedAt.Add(3 * time.Hour)
	outcome := func(ticker string, pnl float64, exitReason string) (uuid.UUID, tradingstack.SimOutcome) {
		id := syntheticScanResultID(ticker)
		return id, tradingstack.SimOutcome{
			BaseModel:    tradingstack.BaseModel{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("shadowdeploy-synthetic-outcome-"+ticker))},
			ScanResultID: id,
			SimulatedAt:  simulatedAt,
			PnlPct:       &pnl,
			ExitReason:   exitReason,
		}
	}

	outcomes := map[uuid.UUID][]tradingstack.SimOutcome{}
	for _, o := range []struct {
		ticker string
		pnl    float64
		exit   string
	}{
		{"AAPL", 2.5, "target"},
		{"MSFT", -1.0, "stop"},
		{"NVDA", 4.0, "target"},
		{"AMD", 1.5, "target"},
		{"GOOG", -0.5, "timeout"},
	} {
		id, row := outcome(o.ticker, o.pnl, o.exit)
		outcomes[id] = append(outcomes[id], row)
	}

	return SyntheticFixture{
		ActivePayload:        active,
		ShadowPayload:        shadow,
		Observations:         observations,
		OutcomesByScanResult: outcomes,
	}
}
