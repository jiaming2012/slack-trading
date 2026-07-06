package scanneropt

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

func fptr(v float64) *float64 { return &v }

// testRow builds one weighted training row for tuner fixtures.
func testRow(regime string, scannedAt time.Time, pnl, ev, rsi, vr, atr float64) optvalidation.WeightedTrainingRow {
	return optvalidation.WeightedTrainingRow{
		TrainingRow: optvalidation.TrainingRow{
			ScanResultID: uuid.New(),
			StrategyID:   "strategy-test",
			Ticker:       "TICK",
			ScannedAt:    scannedAt,
			RegimeTag:    regime,
			RSI14:        rsi,
			VolumeRatio:  vr,
			ATRPct:       atr,
			PnlPct:       pnl,
		},
		EVWeight: ev,
	}
}

// tunerBaseline is the baseline payload shared by the tuner tests:
// min_labeled_samples 4 so the five-decided-row fixture regime derives and a
// three-decided-row regime is thin.
func tunerBaseline() scannercfg.Payload {
	return scannercfg.Payload{
		Version: "base-v1",
		RegimeModels: map[string]scannercfg.RegimeModel{
			"trending": {
				FeatureWeights: map[string]float64{"rsi_14": 0.5, "volume_ratio": 0.3, "atr_pct": 0.2},
				DropFeatures:   []string{"pe_ratio"},
				HardFilterOverrides: scannercfg.HardFilterOverrides{
					VolumeRatioFloor: fptr(1.2),
					ATRPctCeiling:    fptr(3.5),
				},
				ScoreThreshold: fptr(0.65),
			},
			"mean_reverting": {
				HardFilterOverrides: scannercfg.HardFilterOverrides{
					VolumeRatioFloor: fptr(1.0),
				},
				ScoreThreshold: fptr(0.7),
			},
		},
		Global: scannercfg.GlobalConfig{TopNCandidates: 20, MinLabeledSamples: 4},
	}
}

// fixtureRows returns the hand-computed "trending" regime group: three wins
// (EV weights 1, 2, 1), two losses (EV weight 1 each), and one breakeven row
// with extreme feature values that must be excluded from every
// label-conditioned statistic.
func fixtureRows(base time.Time) []optvalidation.WeightedTrainingRow {
	return []optvalidation.WeightedTrainingRow{
		testRow("trending", base.Add(1*time.Hour), 0.02, 1.0, 60, 1.5, 2.0),
		testRow("trending", base.Add(2*time.Hour), 0.03, 2.0, 62, 1.8, 2.4),
		testRow("trending", base.Add(3*time.Hour), 0.01, 1.0, 64, 2.0, 2.2),
		testRow("trending", base.Add(4*time.Hour), -0.02, 1.0, 40, 0.9, 4.0),
		testRow("trending", base.Add(5*time.Hour), -0.01, 1.0, 44, 1.1, 3.0),
		// Breakeven: PnlPct == 0 -- excluded. Its extreme values would wreck
		// every statistic if it leaked in.
		testRow("trending", base.Add(6*time.Hour), 0.0, 5.0, 999, 99, 99),
	}
}

// TestTune_HandComputedWeightsAndOverrides pins the derivation against hand
// arithmetic.
//
// EV-weighted win statistics (weights 1, 2, 1; total 4):
//
//	rsi_14:       mean (60+2*62+64)/4 = 62,     var (4+0+4)/4 = 2
//	volume_ratio: mean (1.5+3.6+2.0)/4 = 1.775, var 0.1275/4 = 0.031875
//	atr_pct:      mean (2.0+4.8+2.2)/4 = 2.25,  var 0.11/4 = 0.0275
//
// EV-weighted loss statistics (weights 1, 1; total 2):
//
//	rsi_14:       mean 42,  var 4
//	volume_ratio: mean 1.0, var 0.01
//	atr_pct:      mean 3.5, var 0.25
//
// Pooled std dev sqrt((4*varWin + 2*varLoss)/6) and effect size
// |meanWin - meanLoss| / pooled:
//
//	rsi_14:       sqrt(16/6),     e = 20/sqrt(16/6)
//	volume_ratio: sqrt(0.1475/6), e = 0.775/sqrt(0.1475/6)
//	atr_pct:      sqrt(0.61/6),   e = 1.25/sqrt(0.61/6)
//
// Weights are each effect size over the sum of all three.
//
// Overrides from the winners only (weights 1, 2, 1; total 4):
//
//	volume_ratio_floor: p25 -- target cumulative weight 1.0; sorted values
//	  1.5 (w 1), 1.8 (w 2), 2.0 (w 1); 1.5 reaches 1.0 -> floor 1.5
//	atr_pct_ceiling: p75 -- target 3.0; sorted 2.0 (w 1), 2.2 (w 1),
//	  2.4 (w 2); 2.4 reaches 4.0 >= 3.0 -> ceiling 2.4
func TestTune_HandComputedWeightsAndOverrides(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

	proposal, err := Tune(fixtureRows(base), tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	eRSI := 20.0 / math.Sqrt(16.0/6.0)
	eVR := 0.775 / math.Sqrt(0.1475/6.0)
	eATR := 1.25 / math.Sqrt(0.61/6.0)
	total := eRSI + eVR + eATR

	model, ok := proposal.RegimeModels["trending"]
	require.True(t, ok)

	require.Len(t, model.FeatureWeights, 3)
	assert.InDelta(t, eRSI/total, model.FeatureWeights["rsi_14"], 1e-9)
	assert.InDelta(t, eVR/total, model.FeatureWeights["volume_ratio"], 1e-9)
	assert.InDelta(t, eATR/total, model.FeatureWeights["atr_pct"], 1e-9)

	weightSum := model.FeatureWeights["rsi_14"] + model.FeatureWeights["volume_ratio"] + model.FeatureWeights["atr_pct"]
	assert.InDelta(t, 1.0, weightSum, 1e-9, "proposed weights must sum to 1")

	require.NotNil(t, model.HardFilterOverrides.VolumeRatioFloor)
	assert.InDelta(t, 1.5, *model.HardFilterOverrides.VolumeRatioFloor, 1e-9)
	require.NotNil(t, model.HardFilterOverrides.ATRPctCeiling)
	assert.InDelta(t, 2.4, *model.HardFilterOverrides.ATRPctCeiling, 1e-9)

	require.NotNil(t, model.ScoreThreshold)
	assert.Equal(t, 0.65, *model.ScoreThreshold, "score_threshold must be carried forward unchanged")
	assert.Equal(t, []string{"pe_ratio"}, model.DropFeatures, "drop_features must be carried forward")

	assert.Equal(t, "proposed-v2", proposal.Version)
	assert.Equal(t, tunerBaseline().Global, proposal.Global, "global section is carried from the baseline")
}

// TestTune_ThinRegimeCarriesBaselineModel: a regime with fewer decided rows
// than min_labeled_samples produces no derived model; the baseline's model is
// carried forward verbatim.
func TestTune_ThinRegimeCarriesBaselineModel(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	rows := append(fixtureRows(base),
		// Only 3 decided mean_reverting rows -- below min_labeled_samples 4.
		testRow("mean_reverting", base.Add(7*time.Hour), 0.02, 1.0, 55, 1.4, 1.9),
		testRow("mean_reverting", base.Add(8*time.Hour), -0.01, 1.0, 45, 1.0, 2.5),
		testRow("mean_reverting", base.Add(9*time.Hour), 0.01, 1.0, 58, 1.6, 2.1),
	)

	proposal, err := Tune(rows, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	assert.Equal(t, tunerBaseline().RegimeModels["mean_reverting"], proposal.RegimeModels["mean_reverting"],
		"thin regime must carry the baseline model forward")
}

// TestTune_EmptyRegimeTagExcluded: rows with an empty regime tag never
// produce a model and never contaminate other regimes.
func TestTune_EmptyRegimeTagExcluded(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	withTagless := append(fixtureRows(base),
		testRow("", base.Add(10*time.Hour), 0.5, 9.0, 1, 0.1, 50),
		testRow("", base.Add(11*time.Hour), -0.5, 9.0, 99, 9.0, 0.1),
	)

	withProposal, err := Tune(withTagless, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)
	withoutProposal, err := Tune(fixtureRows(base), tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	assert.Equal(t, withoutProposal, withProposal, "empty-tag rows must not affect the proposal")
	_, hasEmpty := withProposal.RegimeModels[""]
	assert.False(t, hasEmpty)
}

// TestTune_NoLossesCarriesBaselineWeightsButDerivesOverrides: with no losing
// rows the win/loss contrast is undefined, so every effect size is 0 and the
// baseline's feature weights are carried forward -- but the winner
// percentiles are still well-defined, so overrides are derived.
func TestTune_NoLossesCarriesBaselineWeightsButDerivesOverrides(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	rows := []optvalidation.WeightedTrainingRow{
		testRow("trending", base.Add(1*time.Hour), 0.02, 1.0, 60, 1.5, 2.0),
		testRow("trending", base.Add(2*time.Hour), 0.03, 1.0, 62, 1.8, 2.4),
		testRow("trending", base.Add(3*time.Hour), 0.01, 1.0, 64, 2.0, 2.2),
		testRow("trending", base.Add(4*time.Hour), 0.02, 1.0, 61, 1.6, 2.1),
	}

	proposal, err := Tune(rows, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	model := proposal.RegimeModels["trending"]
	assert.Equal(t, tunerBaseline().RegimeModels["trending"].FeatureWeights, model.FeatureWeights,
		"all-zero effect sizes must carry the baseline feature weights forward")
	require.NotNil(t, model.HardFilterOverrides.VolumeRatioFloor)
	assert.InDelta(t, 1.5, *model.HardFilterOverrides.VolumeRatioFloor, 1e-9,
		"p25 of winners' volume_ratio (1.5, 1.6, 1.8, 2.0; equal weights)")
	require.NotNil(t, model.HardFilterOverrides.ATRPctCeiling)
	assert.InDelta(t, 2.2, *model.HardFilterOverrides.ATRPctCeiling, 1e-9,
		"p75 of winners' atr_pct (2.0, 2.1, 2.2, 2.4; equal weights)")
}

// TestTune_NoWinnersCarriesBaselineModel: with no winning rows the winner
// percentiles are undefined, so no model is derived at all.
func TestTune_NoWinnersCarriesBaselineModel(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	rows := []optvalidation.WeightedTrainingRow{
		testRow("trending", base.Add(1*time.Hour), -0.02, 1.0, 40, 0.9, 4.0),
		testRow("trending", base.Add(2*time.Hour), -0.01, 1.0, 44, 1.1, 3.0),
		testRow("trending", base.Add(3*time.Hour), -0.03, 1.0, 42, 1.0, 3.5),
		testRow("trending", base.Add(4*time.Hour), -0.02, 1.0, 41, 0.95, 3.8),
	}

	proposal, err := Tune(rows, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	assert.Equal(t, tunerBaseline().RegimeModels["trending"], proposal.RegimeModels["trending"])
}

// TestTune_Deterministic: two runs over the same rows and baseline produce
// identical payloads, byte-for-byte.
func TestTune_Deterministic(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	rows := append(fixtureRows(base),
		testRow("mean_reverting", base.Add(7*time.Hour), 0.02, 1.0, 55, 1.4, 1.9),
	)

	first, err := Tune(rows, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)
	second, err := Tune(rows, tunerBaseline(), "proposed-v2")
	require.NoError(t, err)

	assert.Equal(t, first, second)

	firstJSON, err := first.Marshal()
	require.NoError(t, err)
	secondJSON, err := second.Marshal()
	require.NoError(t, err)
	assert.Equal(t, string(firstJSON), string(secondJSON))
}

// TestTune_DoesNotMutateBaseline guards the carry-forward path: tuning must
// never write through to the baseline payload.
func TestTune_DoesNotMutateBaseline(t *testing.T) {
	base := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	baseline := tunerBaseline()

	_, err := Tune(fixtureRows(base), baseline, "proposed-v2")
	require.NoError(t, err)

	assert.Equal(t, tunerBaseline(), baseline, "baseline must be unchanged after tuning")
}

// --- weighted statistics helpers --------------------------------------------

func TestWeightedMean(t *testing.T) {
	assert.InDelta(t, 62.0, weightedMean([]float64{60, 62, 64}, []float64{1, 2, 1}), 1e-12)
	assert.Equal(t, 0.0, weightedMean(nil, nil), "empty input yields 0")
	assert.Equal(t, 0.0, weightedMean([]float64{5}, []float64{0}), "zero total weight yields 0")
}

func TestPooledWeightedStdDev(t *testing.T) {
	// Hand computation matching the tuner fixture's rsi_14 numbers.
	got := pooledWeightedStdDev(
		[]float64{60, 62, 64}, []float64{1, 2, 1},
		[]float64{40, 44}, []float64{1, 1},
	)
	assert.InDelta(t, math.Sqrt(16.0/6.0), got, 1e-12)

	assert.Equal(t, 0.0, pooledWeightedStdDev(nil, nil, nil, nil))
}

func TestWeightedPercentile(t *testing.T) {
	values := []float64{2.0, 1.5, 1.8} // deliberately unsorted
	weights := []float64{1, 1, 2}

	// total weight 4; p25 target 1.0: sorted 1.5 (cum 1.0) -> 1.5
	assert.InDelta(t, 1.5, weightedPercentile(values, weights, 0.25), 1e-12)
	// p75 target 3.0: sorted 1.5 (1.0), 1.8 (3.0) -> 1.8
	assert.InDelta(t, 1.8, weightedPercentile(values, weights, 0.75), 1e-12)
	// p100 -> maximum
	assert.InDelta(t, 2.0, weightedPercentile(values, weights, 1.0), 1e-12)

	assert.Equal(t, 0.0, weightedPercentile(nil, nil, 0.5), "empty input yields 0")
	assert.Equal(t, 0.0, weightedPercentile([]float64{3}, []float64{0}, 0.5), "zero total weight yields 0")
}
