package shadowdeploy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

func floatPtr(v float64) *float64 { return &v }

func strPtr(v string) *string { return &v }

// testObservation builds a "trending" observation with the given features
// (nil pointers allowed).
func testObservation(ticker string, vr, rsi, atr *float64) Observation {
	return Observation{
		ScanResultID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("engine-test-"+ticker)),
		Ticker:       ticker,
		ScannedAt:    time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC),
		RegimeTag:    strPtr("trending"),
		VolumeRatio:  vr,
		RSI14:        rsi,
		ATRPct:       atr,
	}
}

// enginePayload is the hand-computed fixture payload: one "trending" model
// weighting volume_ratio 0.6 / rsi_14 0.4 (atr_pct dropped), a volume floor
// of 1.0, a score threshold of 0.5, and top-2 selection.
func enginePayload() scannercfg.Payload {
	return scannercfg.Payload{
		Version: "engine-fixture-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights: map[string]float64{
					"volume_ratio": 0.6,
					"rsi_14":       0.4,
					"atr_pct":      0.9, // dropped below -- must not influence scores
				},
				DropFeatures: []string{"atr_pct"},
				HardFilterOverrides: scannercfg.HardFilterOverrides{
					VolumeRatioFloor: floatPtr(1.0),
				},
				ScoreThreshold: floatPtr(0.5),
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 2, MinLabeledSamples: 1},
	}
}

// engineObservations is the six-row batch the hand computation below covers.
//
// volume_ratio batch (non-nil): 2.0, 1.5, 1.0, 0.5, 1.2 -> min 0.5, max 2.0
// rsi_14 batch:                 80, 60, 40, 90, 50, 70  -> min 40, max 90
//
// Normalized scores (0.6*vr_norm + 0.4*rsi_norm):
//
//	AAA: 0.6*1.0     + 0.4*0.8 = 0.92  admitted, >= 0.5, top-2 -> selected
//	BBB: 0.6*(2/3)   + 0.4*0.4 = 0.56  admitted, >= 0.5, top-2 -> selected
//	CCC: 0.6*(1/3)   + 0.4*0.0 = 0.20  admitted, below threshold
//	DDD: 0.6*0.0     + 0.4*1.0 = 0.40  vr 0.5 < floor 1.0 -> not admitted
//	EEE: regime "unknown" -> no_model, admitted by default, unscored
//	FFF: 0.6*0(nil)  + 0.4*0.6 = 0.24  nil vr: noted, does not reject
func engineObservations() []Observation {
	eee := testObservation("EEE", floatPtr(1.2), floatPtr(50), nil)
	eee.RegimeTag = strPtr("unknown")

	return []Observation{
		testObservation("AAA", floatPtr(2.0), floatPtr(80), floatPtr(5.0)),
		testObservation("BBB", floatPtr(1.5), floatPtr(60), nil),
		testObservation("CCC", floatPtr(1.0), floatPtr(40), nil),
		testObservation("DDD", floatPtr(0.5), floatPtr(90), nil),
		eee,
		testObservation("FFF", nil, floatPtr(70), nil),
	}
}

// TestEvaluateConfig_HandComputedFixture pins the full decision vector --
// admission, normalized scores, threshold, and top-N -- against the values
// computed by hand in the engineObservations comment.
func TestEvaluateConfig_HandComputedFixture(t *testing.T) {
	decisions := EvaluateConfig(enginePayload(), engineObservations())
	require.Len(t, decisions, 6)

	byTicker := map[string]Decision{}
	for _, d := range decisions {
		byTicker[d.Ticker] = d
	}

	const tolerance = 1e-9

	aaa := byTicker["AAA"]
	assert.True(t, aaa.Admitted)
	assert.True(t, aaa.Scored)
	assert.InDelta(t, 0.92, aaa.Score, tolerance)
	assert.True(t, aaa.Selected)

	bbb := byTicker["BBB"]
	assert.True(t, bbb.Admitted)
	assert.InDelta(t, 0.56, bbb.Score, tolerance)
	assert.True(t, bbb.Selected)

	ccc := byTicker["CCC"]
	assert.True(t, ccc.Admitted)
	assert.InDelta(t, 0.20, ccc.Score, tolerance)
	assert.False(t, ccc.Selected, "below the 0.5 score threshold")

	ddd := byTicker["DDD"]
	assert.False(t, ddd.Admitted, "volume_ratio 0.5 is below the 1.0 floor")
	assert.True(t, ddd.Scored, "scores are computed even for non-admitted rows")
	assert.InDelta(t, 0.40, ddd.Score, tolerance)
	assert.False(t, ddd.Selected)

	fff := byTicker["FFF"]
	assert.True(t, fff.Admitted, "a nil volume_ratio never rejects")
	assert.InDelta(t, 0.24, fff.Score, tolerance)
	assert.Equal(t, []string{"volume_ratio"}, fff.MissingFeatures)
	assert.False(t, fff.Selected)

	// Decisions come back in input order.
	for i, want := range []string{"AAA", "BBB", "CCC", "DDD", "EEE", "FFF"} {
		assert.Equal(t, want, decisions[i].Ticker)
	}
}

// TestEvaluateConfig_HardFilterRejectionRegardlessOfScore: DDD carries the
// batch's highest RSI, but the volume floor rejects it -- admission is
// independent of the score.
func TestEvaluateConfig_HardFilterRejectionRegardlessOfScore(t *testing.T) {
	payload := enginePayload()
	// Remove the threshold and widen top-N so only admission can exclude.
	model := payload.RegimeModels["trending"]
	model.ScoreThreshold = nil
	payload.RegimeModels["trending"] = model
	payload.Global.TopNCandidates = 100

	decisions := EvaluateConfig(payload, engineObservations())
	for _, d := range decisions {
		if d.Ticker != "DDD" {
			continue
		}
		assert.False(t, d.Admitted)
		assert.False(t, d.Selected)
		assert.Greater(t, d.Score, 0.0, "the rejected row still carries its score")
	}
}

// TestEvaluateConfig_NoModelVisibleNeverSelected: EEE's "unknown" regime has
// no regime_models entry -- it stays in the output, admitted by default,
// unscored, and never selected even with an unbounded top-N.
func TestEvaluateConfig_NoModelVisibleNeverSelected(t *testing.T) {
	payload := enginePayload()
	payload.Global.TopNCandidates = 100

	decisions := EvaluateConfig(payload, engineObservations())
	require.Len(t, decisions, 6, "no_model rows must not be dropped")

	var eee *Decision
	for i := range decisions {
		if decisions[i].Ticker == "EEE" {
			eee = &decisions[i]
		}
	}
	require.NotNil(t, eee)
	assert.True(t, eee.NoModel)
	assert.True(t, eee.Admitted, "no_model rows are admitted by default")
	assert.False(t, eee.Scored)
	assert.Nil(t, eee.ScorePtr())
	assert.False(t, eee.Selected, "no_model rows are never selected")
}

// TestEvaluateConfig_DeterministicIncludingTopNTies: two identically scored
// tickers sit on the top-1 boundary; the tie breaks by ticker ascending, and
// repeated evaluation is byte-identical.
func TestEvaluateConfig_DeterministicIncludingTopNTies(t *testing.T) {
	payload := scannercfg.Payload{
		Version: "tie-fixture-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights:      map[string]float64{"volume_ratio": 1.0},
				HardFilterOverrides: scannercfg.HardFilterOverrides{},
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 1, MinLabeledSamples: 1},
	}

	obs := []Observation{
		testObservation("ZZZ", floatPtr(2.0), nil, nil),
		testObservation("MMM", floatPtr(2.0), nil, nil),
		testObservation("AAA", floatPtr(1.0), nil, nil),
	}

	first := EvaluateConfig(payload, obs)
	second := EvaluateConfig(payload, obs)
	assert.Equal(t, first, second, "identical inputs must produce identical decisions")

	selected := []string{}
	for _, d := range first {
		if d.Selected {
			selected = append(selected, d.Ticker)
		}
	}
	assert.Equal(t, []string{"MMM"}, selected, "the top-1 tie between MMM and ZZZ breaks by ticker ascending")
}

// TestEvaluateConfig_NilFeatureContributesZeroAndIsNoted covers a model
// weighting a feature the whole batch lacks (scanner_score): every decision
// notes it and the missing feature adds nothing to any score.
func TestEvaluateConfig_NilFeatureContributesZeroAndIsNoted(t *testing.T) {
	payload := scannercfg.Payload{
		Version: "nil-fixture-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights: map[string]float64{
					"volume_ratio":  1.0,
					"scanner_score": 5.0,
				},
				HardFilterOverrides: scannercfg.HardFilterOverrides{},
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 10, MinLabeledSamples: 1},
	}

	obs := []Observation{
		testObservation("AAA", floatPtr(2.0), nil, nil),
		testObservation("BBB", floatPtr(1.0), nil, nil),
	}

	decisions := EvaluateConfig(payload, obs)
	require.Len(t, decisions, 2)

	assert.InDelta(t, 1.0, decisions[0].Score, 1e-9, "AAA: vr normalizes to 1.0; scanner_score contributes 0")
	assert.InDelta(t, 0.0, decisions[1].Score, 1e-9)
	for _, d := range decisions {
		assert.Equal(t, []string{"scanner_score"}, d.MissingFeatures)
	}
}
