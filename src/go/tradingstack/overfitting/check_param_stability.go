package overfitting

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// CheckNameParamStability names the parameter-stability check's CheckResult.
const CheckNameParamStability = "parameter_stability"

// checkParamStability passes only when no evaluated parameter violates either
// stability condition:
//
//	(a) step bound -- for every parameter present in both ProposedParams and
//	    BaselineParams with a non-zero baseline, |proposed - baseline| must
//	    not exceed cfg.MaxRelativeStep * |baseline|. A zero-valued baseline
//	    parameter, or a parameter present in only one of the two maps, is
//	    skipped by this condition and noted in the detail rather than failing
//	    it -- new/removed knobs are a review topic, not an automatic failure.
//	(b) cross-fold stability -- for every parameter appearing in the folds'
//	    Params, the population coefficient of variation of its per-fold values
//	    (stddev / |mean|) must not exceed cfg.MaxFoldParamCV; a parameter
//	    whose cross-fold mean is zero is skipped by this condition. A
//	    parameter whose fitted value swings wildly fold-to-fold is fitting
//	    noise.
func checkParamStability(e Evidence, cfg Config) CheckResult {
	var failures, notes []string

	// --- (a) Step bound vs baseline ---
	maxStepObserved := 0.0
	for _, name := range sortedKeys(e.ProposedParams) {
		baseline, inBaseline := e.BaselineParams[name]
		if !inBaseline {
			notes = append(notes, fmt.Sprintf("parameter %q present only in proposal; skipped by step bound", name))
			continue
		}
		if baseline == 0 {
			notes = append(notes, fmt.Sprintf("parameter %q has zero baseline; skipped by step bound", name))
			continue
		}
		step := math.Abs(e.ProposedParams[name]-baseline) / math.Abs(baseline)
		if step > maxStepObserved {
			maxStepObserved = step
		}
		if step > cfg.MaxRelativeStep {
			failures = append(failures, fmt.Sprintf(
				"parameter %q step %.4f exceeds max relative step %.4f (baseline %.6f, proposed %.6f)",
				name, step, cfg.MaxRelativeStep, baseline, e.ProposedParams[name]))
		}
	}
	for _, name := range sortedKeys(e.BaselineParams) {
		if _, inProposed := e.ProposedParams[name]; !inProposed {
			notes = append(notes, fmt.Sprintf("parameter %q present only in baseline; skipped by step bound", name))
		}
	}

	// --- (b) Cross-fold coefficient of variation ---
	foldValues := make(map[string][]float64)
	for _, fold := range e.Folds {
		for name, value := range fold.Params {
			foldValues[name] = append(foldValues[name], value)
		}
	}

	maxCVObserved := 0.0
	for _, name := range sortedKeys(foldValues) {
		mean, stdDev := meanStdDev(foldValues[name])
		if mean == 0 {
			notes = append(notes, fmt.Sprintf("parameter %q has zero cross-fold mean; skipped by cross-fold stability", name))
			continue
		}
		cv := stdDev / math.Abs(mean)
		if cv > maxCVObserved {
			maxCVObserved = cv
		}
		if cv > cfg.MaxFoldParamCV {
			failures = append(failures, fmt.Sprintf(
				"parameter %q cross-fold coefficient of variation %.4f exceeds maximum %.4f",
				name, cv, cfg.MaxFoldParamCV))
		}
	}

	var parts []string
	if len(failures) > 0 {
		parts = append(parts, strings.Join(failures, "; "))
	} else {
		parts = append(parts, "all evaluated parameters within step and cross-fold stability bounds")
	}
	if len(notes) > 0 {
		parts = append(parts, "notes: "+strings.Join(notes, "; "))
	}

	return CheckResult{
		Name:   CheckNameParamStability,
		Passed: len(failures) == 0,
		Observed: map[string]float64{
			"max_relative_step_observed": maxStepObserved,
			"max_fold_param_cv_observed": maxCVObserved,
		},
		Threshold: map[string]float64{
			"max_relative_step": cfg.MaxRelativeStep,
			"max_fold_param_cv": cfg.MaxFoldParamCV,
		},
		Detail: strings.Join(parts, " | "),
	}
}

// sortedKeys returns the map's keys in ascending order so parameter iteration
// (and therefore detail text) is deterministic.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// meanStdDev returns the mean and population standard deviation of values. An
// empty slice returns (0, 0).
func meanStdDev(values []float64) (mean, stdDev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	var sumSq float64
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return mean, math.Sqrt(sumSq / float64(len(values)))
}
