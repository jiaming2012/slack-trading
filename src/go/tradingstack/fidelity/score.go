package fidelity

import "math"

// FidelityConfig holds the deterministic scoring parameters. Weights sum to 1
// and the normalization constants bound the magnitude components into [0, 1].
// All values are documented defaults, not tuned against live data (which does
// not yet exist).
type FidelityConfig struct {
	// WeightPnL, WeightFill, WeightExitReason are the composite weights. They
	// MUST sum to 1.0 so that the weighted sum of the (already [0,1]) normalized
	// components stays in [0, 1].
	WeightPnL        float64
	WeightFill       float64
	WeightExitReason float64
	// PnLNormK and FillNormK are the k constants in the bounded normalization
	// x/(x+k) applied to the mean absolute PnL and fill deltas. Larger k means a
	// given raw drift maps to a smaller normalized value.
	PnLNormK  float64
	FillNormK float64
	// ToleranceThreshold is the inclusive upper bound on drift_score for a
	// strategy to be considered within tolerance.
	ToleranceThreshold float64
}

// DefaultConfig returns the documented default scoring configuration. Weights
// sum to 1.0; the tolerance threshold is 0.2.
func DefaultConfig() FidelityConfig {
	return FidelityConfig{
		WeightPnL:          0.4,
		WeightFill:         0.3,
		WeightExitReason:   0.3,
		PnLNormK:           100.0,
		FillNormK:          1.0,
		ToleranceThreshold: 0.2,
	}
}

// Composite is the reduced per-strategy scoring output.
type Composite struct {
	// DriftPnL is the aggregated (summed) simulator-minus-live PnL delta.
	DriftPnL float64
	// DriftFill is the average per-pair fill delta.
	DriftFill float64
	// DriftScore is the composite drift score, clamped to [0.0, 1.0].
	DriftScore float64
}

// boundedNorm squashes a non-negative magnitude x into [0, 1] via x/(x+k). It is
// monotonic non-decreasing in x for x >= 0 and k > 0, returns 0 at x = 0, and
// returns exactly 1 in the limit of arbitrarily large (infinite) drift so the
// composite reaches its clamp bound rather than merely approaching it.
func boundedNorm(x, k float64) float64 {
	if x <= 0 {
		return 0
	}
	if k <= 0 || math.IsInf(x, 1) {
		return 1
	}
	return x / (x + k)
}

// clamp01 constrains v to the closed interval [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Score reduces a strategy's per-pair deltas into a Composite. The composite
// drift_score is a fixed weighted sum of three normalized components — mean
// absolute PnL drift, mean absolute fill drift, and exit-reason mismatch rate —
// each in [0, 1]. Because the weights sum to 1 the sum is already in [0, 1]; it
// is clamped as a defensive guarantee. The score is 0.0 for perfect fidelity,
// clamped to at most 1.0 under arbitrarily large drift, and monotonic
// non-decreasing in the magnitude of each dimension. An empty delta slice scores
// 0.0 (nothing compared means no observed drift).
func Score(deltas []PairDelta, cfg FidelityConfig) Composite {
	if len(deltas) == 0 {
		return Composite{}
	}

	var sumPnL, sumAbsPnL, sumFill, sumAbsFill float64
	var mismatches int
	for _, d := range deltas {
		sumPnL += d.PnLDelta
		sumAbsPnL += math.Abs(d.PnLDelta)
		sumFill += d.FillDelta
		sumAbsFill += math.Abs(d.FillDelta)
		if !d.ExitReasonMatch {
			mismatches++
		}
	}

	n := float64(len(deltas))
	meanAbsPnL := sumAbsPnL / n
	meanAbsFill := sumAbsFill / n
	mismatchRate := float64(mismatches) / n

	// The composite blends the drift magnitude on each dimension; magnitudes use
	// mean absolute deltas so positive and negative pair drifts do not cancel.
	nPnL := boundedNorm(meanAbsPnL, cfg.PnLNormK)
	nFill := boundedNorm(meanAbsFill, cfg.FillNormK)
	nExit := clamp01(mismatchRate)

	score := cfg.WeightPnL*nPnL + cfg.WeightFill*nFill + cfg.WeightExitReason*nExit

	return Composite{
		// DriftPnL keeps the signed sum (sim minus live) so its sign is
		// meaningful; DriftFill keeps the signed average.
		DriftPnL:   sumPnL,
		DriftFill:  sumFill / n,
		DriftScore: clamp01(score),
	}
}
