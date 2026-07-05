package crowding

import (
	"strconv"

	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// DefaultOverlapThresholdPct is the default overlap percentage above which a
// scan cycle is flagged as crowded, per the crowding-detection spec.
const DefaultOverlapThresholdPct = 30.0

// crowdingOverlapThresholdEnvVar is the optional env var override for the
// default overlap threshold.
const crowdingOverlapThresholdEnvVar = "CROWDING_OVERLAP_THRESHOLD_PCT"

// ResolveOverlapThresholdPct returns the configured overlap threshold
// percentage: the value of CROWDING_OVERLAP_THRESHOLD_PCT if set and parses
// as a float, otherwise DefaultOverlapThresholdPct.
func ResolveOverlapThresholdPct() float64 {
	raw, err := utils.GetEnv(crowdingOverlapThresholdEnvVar)
	if err != nil {
		return DefaultOverlapThresholdPct
	}

	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return DefaultOverlapThresholdPct
	}

	return parsed
}
