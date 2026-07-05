package evtracker

// DecayStatus classifies a strategy/regime group's edge trend.
type DecayStatus string

const (
	// StatusImproving marks a group whose EV slope exceeds the scale-up threshold.
	StatusImproving DecayStatus = "improving"
	// StatusStable marks a group whose EV slope is within the stable band.
	StatusStable DecayStatus = "stable"
	// StatusDecaying marks a group whose EV slope is negative (flag for review).
	StatusDecaying DecayStatus = "decaying"
	// StatusInsufficientData marks a group with a null (undefined) slope.
	StatusInsufficientData DecayStatus = "insufficient_data"
)

// EV weight bands. The slope thresholds are inclusive at their stated
// boundaries: exactly 0.1 and exactly 0.0 both fall in the stable band.
const (
	// SlopeScaleUpThreshold is the slope above which a strategy scales up.
	SlopeScaleUpThreshold = 0.1

	// WeightScaleUp is assigned to improving strategies (slope > 0.1).
	WeightScaleUp = 1.0
	// WeightStable is assigned to stable strategies (0 <= slope <= 0.1) and to
	// insufficient-data strategies (null slope).
	WeightStable = 0.7
	// WeightDecay is assigned to decaying strategies (slope < 0).
	WeightDecay = 0.3
)

// ClassifyWeight maps an EV trend slope to an EV weight and decay status. A nil
// slope (fewer than two non-empty buckets) is insufficient data and defaults to
// the stable weight. The band boundaries are inclusive: slope > 0.1 improves;
// 0 <= slope <= 0.1 is stable; slope < 0 decays.
func ClassifyWeight(slope *float64) (float64, DecayStatus) {
	if slope == nil {
		return WeightStable, StatusInsufficientData
	}
	switch {
	case *slope > SlopeScaleUpThreshold:
		return WeightScaleUp, StatusImproving
	case *slope < 0:
		return WeightDecay, StatusDecaying
	default:
		return WeightStable, StatusStable
	}
}
