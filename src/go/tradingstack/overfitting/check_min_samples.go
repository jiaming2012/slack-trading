package overfitting

import (
	"fmt"
	"strings"
)

// CheckNameMinSamples names the minimum-sample gate's CheckResult.
const CheckNameMinSamples = "minimum_samples"

// checkMinSamples passes only when the evidence's total decided labeled
// sample count and its out-of-sample window size both meet their configured
// minimums. Values exactly at a threshold pass. The result reports both
// observed counts and both thresholds.
func checkMinSamples(e Evidence, cfg Config) CheckResult {
	var failures []string

	if e.SampleSize < cfg.MinSamples {
		failures = append(failures, fmt.Sprintf(
			"total labeled samples %d below minimum %d", e.SampleSize, cfg.MinSamples))
	}
	if e.OutOfSample.SampleSize < cfg.MinOOSSamples {
		failures = append(failures, fmt.Sprintf(
			"out-of-sample samples %d below minimum %d", e.OutOfSample.SampleSize, cfg.MinOOSSamples))
	}

	detail := "sample counts meet both minimums"
	if len(failures) > 0 {
		detail = strings.Join(failures, "; ")
	}

	return CheckResult{
		Name:   CheckNameMinSamples,
		Passed: len(failures) == 0,
		Observed: map[string]float64{
			"total_samples":         float64(e.SampleSize),
			"out_of_sample_samples": float64(e.OutOfSample.SampleSize),
		},
		Threshold: map[string]float64{
			"min_samples":     float64(cfg.MinSamples),
			"min_oos_samples": float64(cfg.MinOOSSamples),
		},
		Detail: detail,
	}
}
