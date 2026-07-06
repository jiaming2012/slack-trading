package shadowdeploy

import (
	"sort"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// scoreableFeatures is the fixed payload feature-name vocabulary the engine
// can resolve against an Observation. A weighted feature outside this set
// contributes 0 to the score and is noted on the decision, exactly like a nil
// feature value.
var scoreableFeatures = []string{"atr_pct", "rsi_14", "scanner_score", "volume_ratio"}

// featureValue resolves one named feature on an observation; nil means the
// value is absent (NULL column or unknown feature name).
func featureValue(o Observation, feature string) *float64 {
	switch feature {
	case "volume_ratio":
		return o.VolumeRatio
	case "rsi_14":
		return o.RSI14
	case "atr_pct":
		return o.ATRPct
	case "scanner_score":
		return o.ScannerScore
	default:
		return nil
	}
}

// Decision is the engine's verdict for one observation under one payload.
type Decision struct {
	// ScanResultID and Ticker identify the observation the decision is
	// about; decisions are returned in input (observation) order.
	ScanResultID uuid.UUID
	Ticker       string

	// Regime is the resolved regime tag ("" when the row carried none).
	Regime string

	// NoModel is true when the payload has no regime_models entry for the
	// observation's regime: admitted by default, unscored, never selected --
	// visible in the output rather than silently dropped.
	NoModel bool

	// Admitted reports the regime's hard_filter_overrides verdict. An
	// absent override or a nil feature value does not reject: overrides
	// tighten Layer 1, they do not re-run it.
	Admitted bool

	// Scored is true when a regime model existed and a score was computed
	// (even for non-admitted observations, so divergence detail can show
	// both sides' scores). Score is meaningful only when Scored is true.
	Scored bool
	Score  float64

	// Selected is true when the observation survived admission, the
	// regime's score threshold, and the global top-N cut.
	Selected bool

	// MissingFeatures lists weighted features that contributed 0 to the
	// score because the observation's value was nil (or the feature name is
	// outside the engine's vocabulary), in ascending order.
	MissingFeatures []string
}

// ScorePtr returns the decision's score as a pointer, nil when unscored --
// the shape divergence detail and reports use.
func (d Decision) ScorePtr() *float64 {
	if !d.Scored {
		return nil
	}
	s := d.Score
	return &s
}

// featureRange holds one feature's batch min/max over non-nil values.
type featureRange struct {
	min, max float64
	seen     bool
}

// normalize maps a raw value into [0, 1] by batch min-max. A feature whose
// batch range is empty or degenerate (min == max: no contrast, therefore no
// information) normalizes to 0 -- pinned so evaluation is deterministic.
func (r featureRange) normalize(v float64) float64 {
	if !r.seen || r.max == r.min {
		return 0
	}
	return (v - r.min) / (r.max - r.min)
}

// batchRanges computes each scoreable feature's min-max over the non-nil
// values of the whole observation batch. Normalizing per batch makes
// heterogeneous feature scales (RSI 0-100 vs volume_ratio around 1)
// commensurable; scores are therefore only comparable within a run, never
// across runs.
func batchRanges(obs []Observation) map[string]featureRange {
	ranges := make(map[string]featureRange, len(scoreableFeatures))
	for _, feature := range scoreableFeatures {
		var r featureRange
		for _, o := range obs {
			v := featureValue(o, feature)
			if v == nil {
				continue
			}
			if !r.seen || *v < r.min {
				r.min = *v
			}
			if !r.seen || *v > r.max {
				r.max = *v
			}
			r.seen = true
		}
		ranges[feature] = r
	}
	return ranges
}

// EvaluateConfig evaluates one scanner-config payload over one observation
// batch, returning one Decision per observation in input order. It is pure
// (no I/O, no clock, no randomness) and deterministic: identical inputs
// always produce identical output, with top-N ties broken by ticker
// ascending (then scan result id ascending).
//
// Per observation:
//
//  1. Regime resolution: no regime_models entry -> no_model (admitted by
//     default, unscored, never selected).
//  2. Admission: the regime's hard_filter_overrides -- volume_ratio >=
//     volume_ratio_floor and atr_pct <= atr_pct_ceiling. An absent override
//     or a nil feature value does not reject.
//  3. Scoring: the weighted sum of the regime's feature_weights (excluding
//     drop_features) over features min-max normalized to [0, 1] per feature
//     across the batch. A nil feature contributes 0 and is noted on the
//     decision.
//  4. Selection: admitted AND score >= the regime's score_threshold (a model
//     without a threshold selects all admitted), then global top-N by score
//     per global.top_n_candidates.
func EvaluateConfig(payload scannercfg.Payload, obs []Observation) []Decision {
	ranges := batchRanges(obs)

	decisions := make([]Decision, len(obs))
	for i, o := range obs {
		regime := o.Regime()
		d := Decision{
			ScanResultID: o.ScanResultID,
			Ticker:       o.Ticker,
			Regime:       regime,
		}

		model, ok := payload.RegimeModels[regime]
		if !ok {
			d.NoModel = true
			d.Admitted = true
			decisions[i] = d
			continue
		}

		d.Admitted = admit(model.HardFilterOverrides, o)
		d.Score, d.MissingFeatures = score(model, o, ranges)
		d.Scored = true
		decisions[i] = d
	}

	selectTopN(payload, decisions)
	return decisions
}

// admit applies the regime's hard-filter overrides. Overrides tighten Layer 1
// admission: an absent override imposes nothing, and a nil feature value is
// never grounds for rejection (Layer 1 already ran when the row persisted).
func admit(overrides scannercfg.HardFilterOverrides, o Observation) bool {
	if f := overrides.VolumeRatioFloor; f != nil && o.VolumeRatio != nil && *o.VolumeRatio < *f {
		return false
	}
	if c := overrides.ATRPctCeiling; c != nil && o.ATRPct != nil && *o.ATRPct > *c {
		return false
	}
	return true
}

// score computes the weighted sum of the model's feature weights (minus
// drop_features) over batch-normalized features, iterating features in
// ascending name order so floating-point summation order is deterministic.
// Nil (or unknown) features contribute 0 and are reported.
func score(model scannercfg.RegimeModel, o Observation, ranges map[string]featureRange) (float64, []string) {
	dropped := make(map[string]bool, len(model.DropFeatures))
	for _, f := range model.DropFeatures {
		dropped[f] = true
	}

	features := make([]string, 0, len(model.FeatureWeights))
	for f := range model.FeatureWeights {
		if !dropped[f] {
			features = append(features, f)
		}
	}
	sort.Strings(features)

	var total float64
	var missing []string
	for _, f := range features {
		v := featureValue(o, f)
		if v == nil {
			missing = append(missing, f)
			continue
		}
		total += model.FeatureWeights[f] * ranges[f].normalize(*v)
	}
	return total, missing
}

// selectTopN marks Selected on the decisions that survive their regime's
// score threshold and the global top-N cut. Candidates are ordered by score
// descending with ties broken by ticker ascending, then scan result id
// ascending, so the boundary is deterministic. no_model decisions are never
// candidates.
func selectTopN(payload scannercfg.Payload, decisions []Decision) {
	candidates := make([]int, 0, len(decisions))
	for i, d := range decisions {
		if d.NoModel || !d.Admitted || !d.Scored {
			continue
		}
		threshold := payload.RegimeModels[d.Regime].ScoreThreshold
		if threshold != nil && d.Score < *threshold {
			continue
		}
		candidates = append(candidates, i)
	}

	sort.Slice(candidates, func(a, b int) bool {
		da, db := decisions[candidates[a]], decisions[candidates[b]]
		if da.Score != db.Score {
			return da.Score > db.Score
		}
		if da.Ticker != db.Ticker {
			return da.Ticker < db.Ticker
		}
		return da.ScanResultID.String() < db.ScanResultID.String()
	})

	n := payload.Global.TopNCandidates
	if n > len(candidates) {
		n = len(candidates)
	}
	if n < 0 {
		n = 0
	}
	for _, idx := range candidates[:n] {
		decisions[idx].Selected = true
	}
}
