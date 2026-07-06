package scanneropt

import (
	"fmt"
	"sort"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// tunedFeatures are the numeric TrainingRow features the v1 tuner derives
// effect-size weights over, in the fixed payload feature-name vocabulary.
var tunedFeatures = []string{"atr_pct", "rsi_14", "volume_ratio"}

// featureValue returns a row's value for one of the tuned features.
func featureValue(row optvalidation.WeightedTrainingRow, feature string) float64 {
	switch feature {
	case "rsi_14":
		return row.RSI14
	case "volume_ratio":
		return row.VolumeRatio
	case "atr_pct":
		return row.ATRPct
	default:
		return 0
	}
}

// Tune derives a complete proposed scannercfg.Payload from the clean weighted
// training rows, per the v1 deterministic method:
//
//   - Rows are grouped by RegimeTag; rows with an empty tag are excluded.
//   - A row with PnlPct > 0 is a win, PnlPct < 0 a loss; PnlPct == 0
//     (breakeven) is excluded from label-conditioned statistics. All
//     statistics are weighted by the row's EVWeight.
//   - Feature weights over rsi_14/volume_ratio/atr_pct: per-feature effect
//     size |weightedMean(win) - weightedMean(loss)| / pooledWeightedStdDev,
//     normalized to sum 1. Zero pooled dispersion gives weight 0; if every
//     effect size is 0 (including the no-losses case, where the win/loss
//     contrast is undefined) the baseline's feature weights are carried
//     forward.
//   - Hard-filter overrides: volume_ratio_floor = EV-weighted 25th percentile
//     of winners' volume_ratio; atr_pct_ceiling = EV-weighted 75th percentile
//     of winners' atr_pct.
//   - score_threshold and drop_features are carried forward from the baseline
//     model unchanged (score_threshold is not tuned in this change).
//   - A regime with fewer decided rows than the baseline's global
//     min_labeled_samples -- or with no winners, or with zero total decided
//     weight, so the winner percentiles are undefined -- produces no derived
//     model; the baseline's model for that regime, if any, is carried forward
//     so the proposal is always a complete payload.
//
// The derivation is deterministic: identical rows and baseline always produce
// an identical payload. The baseline is never mutated.
func Tune(rows []optvalidation.WeightedTrainingRow, baseline scannercfg.Payload, version string) (scannercfg.Payload, error) {
	proposal := baseline.Clone()
	proposal.Version = version
	if proposal.RegimeModels == nil {
		proposal.RegimeModels = map[string]scannercfg.RegimeModel{}
	}

	groups := groupByRegime(rows)
	for _, regime := range sortedRegimes(groups) {
		model, derived := deriveRegimeModel(groups[regime], baseline.RegimeModels[regime], baseline.Global.MinLabeledSamples)
		if derived {
			proposal.RegimeModels[regime] = model
		}
	}

	if err := proposal.Validate(); err != nil {
		return scannercfg.Payload{}, fmt.Errorf("scanneropt: tuned payload failed validation: %w", err)
	}
	return proposal, nil
}

// deriveRegimeModel derives one regime's model from its rows. The boolean
// reports whether a model was derived; false means the caller must leave the
// baseline's model (if any) in place.
func deriveRegimeModel(rows []optvalidation.WeightedTrainingRow, baselineModel scannercfg.RegimeModel, minLabeledSamples int) (scannercfg.RegimeModel, bool) {
	var wins, losses []optvalidation.WeightedTrainingRow
	for _, r := range rows {
		switch {
		case r.PnlPct > 0:
			wins = append(wins, r)
		case r.PnlPct < 0:
			losses = append(losses, r)
		}
	}

	decided := len(wins) + len(losses)
	if decided < minLabeledSamples || len(wins) == 0 {
		return scannercfg.RegimeModel{}, false
	}

	winWeights := evWeights(wins)
	lossWeights := evWeights(losses)
	if sumFloats(winWeights)+sumFloats(lossWeights) <= 0 {
		return scannercfg.RegimeModel{}, false
	}

	model := baselineModel.Clone()

	// --- Feature weights: normalized EV-weighted effect sizes ---
	effectSizes := make(map[string]float64, len(tunedFeatures))
	var totalEffect float64
	for _, feature := range tunedFeatures {
		winValues := featureValues(wins, feature)
		lossValues := featureValues(losses, feature)

		effect := 0.0
		if len(losses) > 0 {
			pooled := pooledWeightedStdDev(winValues, winWeights, lossValues, lossWeights)
			if pooled > 0 {
				diff := weightedMean(winValues, winWeights) - weightedMean(lossValues, lossWeights)
				effect = abs(diff) / pooled
			}
		}
		effectSizes[feature] = effect
		totalEffect += effect
	}

	if totalEffect > 0 {
		weights := make(map[string]float64, len(tunedFeatures))
		for _, feature := range tunedFeatures {
			weights[feature] = effectSizes[feature] / totalEffect
		}
		model.FeatureWeights = weights
	}
	// totalEffect == 0: the baseline's feature weights (already on model via
	// the clone) are carried forward.

	// --- Hard-filter overrides: winner percentiles ---
	floor := weightedPercentile(featureValues(wins, "volume_ratio"), winWeights, 0.25)
	ceiling := weightedPercentile(featureValues(wins, "atr_pct"), winWeights, 0.75)
	model.HardFilterOverrides = scannercfg.HardFilterOverrides{
		VolumeRatioFloor: &floor,
		ATRPctCeiling:    &ceiling,
	}

	// score_threshold and drop_features stay as cloned from the baseline.
	return model, true
}

// groupByRegime buckets rows by RegimeTag, excluding rows with an empty tag.
func groupByRegime(rows []optvalidation.WeightedTrainingRow) map[string][]optvalidation.WeightedTrainingRow {
	groups := make(map[string][]optvalidation.WeightedTrainingRow)
	for _, r := range rows {
		if r.RegimeTag == "" {
			continue
		}
		groups[r.RegimeTag] = append(groups[r.RegimeTag], r)
	}
	return groups
}

func sortedRegimes(groups map[string][]optvalidation.WeightedTrainingRow) []string {
	regimes := make([]string, 0, len(groups))
	for regime := range groups {
		regimes = append(regimes, regime)
	}
	sort.Strings(regimes)
	return regimes
}

func featureValues(rows []optvalidation.WeightedTrainingRow, feature string) []float64 {
	out := make([]float64, len(rows))
	for i, r := range rows {
		out[i] = featureValue(r, feature)
	}
	return out
}

func evWeights(rows []optvalidation.WeightedTrainingRow) []float64 {
	out := make([]float64, len(rows))
	for i, r := range rows {
		out[i] = r.EVWeight
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
