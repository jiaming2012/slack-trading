package overfitting

import (
	"fmt"
	"strings"
)

// CheckNameWalkForward names the walk-forward check's CheckResult.
const CheckNameWalkForward = "walk_forward"

// checkWalkForward passes only when the walk-forward fold geometry is sound
// (enough folds, strictly increasing indices, no lookahead within a fold,
// chronologically ordered test windows across folds) AND out-of-sample
// performance is positive and retains at least the configured fraction of a
// strictly positive in-sample performance. A non-positive in-sample mean fails
// the check outright: a proposal that cannot even fit its training window has
// no business being proposed, and a retention ratio against a non-positive
// base is meaningless.
func checkWalkForward(e Evidence, cfg Config) CheckResult {
	var failures []string

	// --- Fold geometry ---
	if len(e.Folds) < cfg.MinFolds {
		failures = append(failures, fmt.Sprintf(
			"fold count %d below minimum %d", len(e.Folds), cfg.MinFolds))
	}

	for i, fold := range e.Folds {
		if i > 0 && fold.Index <= e.Folds[i-1].Index {
			failures = append(failures, fmt.Sprintf(
				"fold indices not strictly increasing at position %d (index %d after %d)",
				i, fold.Index, e.Folds[i-1].Index))
		}
		if fold.TestStart.Before(fold.TrainEnd) {
			failures = append(failures, fmt.Sprintf(
				"lookahead in fold %d: test window starts %s before train window ends %s",
				fold.Index, fold.TestStart.Format(timeLayout), fold.TrainEnd.Format(timeLayout)))
		}
		if i > 0 && e.Folds[i].TestStart.Before(e.Folds[i-1].TestEnd) {
			failures = append(failures, fmt.Sprintf(
				"test windows out of chronological order: fold %d test starts %s before fold %d test ends %s",
				fold.Index, fold.TestStart.Format(timeLayout),
				e.Folds[i-1].Index, e.Folds[i-1].TestEnd.Format(timeLayout)))
		}
	}

	// --- Performance retention ---
	retentionRatio := 0.0
	switch {
	case e.InSample.Mean <= 0:
		failures = append(failures, fmt.Sprintf(
			"in-sample mean %.6f is not strictly positive", e.InSample.Mean))
	default:
		retentionRatio = e.OutOfSample.Mean / e.InSample.Mean
		if e.OutOfSample.Mean <= 0 {
			failures = append(failures, fmt.Sprintf(
				"out-of-sample mean %.6f is not strictly positive", e.OutOfSample.Mean))
		} else if e.OutOfSample.Mean < cfg.OOSRetentionFraction*e.InSample.Mean {
			failures = append(failures, fmt.Sprintf(
				"out-of-sample retention %.4f below required fraction %.4f of in-sample performance",
				retentionRatio, cfg.OOSRetentionFraction))
		}
	}

	detail := "fold geometry sound; out-of-sample performance positive and retained"
	if len(failures) > 0 {
		detail = strings.Join(failures, "; ")
	}

	return CheckResult{
		Name:   CheckNameWalkForward,
		Passed: len(failures) == 0,
		Observed: map[string]float64{
			"fold_count":          float64(len(e.Folds)),
			"in_sample_mean":      e.InSample.Mean,
			"out_of_sample_mean":  e.OutOfSample.Mean,
			"oos_retention_ratio": retentionRatio,
		},
		Threshold: map[string]float64{
			"min_folds":              float64(cfg.MinFolds),
			"oos_retention_fraction": cfg.OOSRetentionFraction,
		},
		Detail: detail,
	}
}

// timeLayout formats fold window timestamps in check detail text.
const timeLayout = "2006-01-02T15:04:05Z07:00"
