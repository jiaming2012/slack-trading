package overfitting

import (
	"time"

	"github.com/google/uuid"
)

// Fixed fixture identities so repeated synthetic runs are byte-identical.
var (
	syntheticPassProposalID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	syntheticFailProposalID = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

// SyntheticEvidencePass builds an in-memory Evidence fixture that clears all
// four checks under DefaultConfig: 600 labeled samples (>= 500) with a
// 60-sample out-of-sample window (>= 50); four chronologically ordered,
// non-lookahead walk-forward folds; out-of-sample mean 0.015 retaining 75% of
// the in-sample mean 0.02 (>= 50%); a 10% proposed parameter step (<= 25%)
// with near-constant per-fold parameter values (CV well under 0.5); and an
// out-of-sample t-statistic of ~2.90 against the deflated bar of 2.0 (4
// trials: sqrt(2*ln 4) ~= 1.67, floored by MinTStat 2.0). It requires no
// database and no network, and is shared by the package tests and the
// overfitting-check command's --synthetic self-check mode.
func SyntheticEvidencePass() Evidence {
	month := func(m time.Month) time.Time {
		return time.Date(2024, m, 1, 0, 0, 0, 0, time.UTC)
	}

	// Monthly buckets Jan..Jun: fold i trains on month i and tests on month
	// i+1, so TestStart >= TrainEnd holds by construction and the test
	// windows are chronologically ordered and non-overlapping.
	folds := make([]FoldResult, 0, 4)
	stopPct := []float64{0.050, 0.052, 0.049, 0.051}
	targetPct := []float64{0.100, 0.098, 0.102, 0.100}
	testMetric := []float64{0.018, 0.016, 0.014, 0.017}
	for i := 0; i < 4; i++ {
		trainStart := month(time.Month(i + 1))
		trainEnd := month(time.Month(i + 2))
		folds = append(folds, FoldResult{
			Index:      i + 1,
			TrainStart: trainStart,
			TrainEnd:   trainEnd,
			TestStart:  trainEnd,
			TestEnd:    month(time.Month(i + 3)),
			Params: map[string]float64{
				"stop_pct":   stopPct[i],
				"target_pct": targetPct[i],
			},
			TestMetric: testMetric[i],
		})
	}

	return Evidence{
		ProposalID:   syntheticPassProposalID,
		ProposalKind: ProposalKindStrategy,
		SampleSize:   600,
		TrialsCount:  4,
		InSample: Performance{
			WindowStart: month(time.January),
			WindowEnd:   month(time.June),
			Mean:        0.02,
			StdDev:      0.05,
			SampleSize:  540,
		},
		OutOfSample: Performance{
			WindowStart: month(time.June),
			WindowEnd:   month(time.July),
			Mean:        0.015,
			StdDev:      0.04,
			SampleSize:  60,
		},
		Folds: folds,
		ProposedParams: map[string]float64{
			"stop_pct":   0.055, // +10% step from baseline, inside the 25% bound
			"target_pct": 0.100, // unchanged
		},
		BaselineParams: map[string]float64{
			"stop_pct":   0.050,
			"target_pct": 0.100,
		},
	}
}

// SyntheticEvidenceFail builds an in-memory Evidence fixture that fails all
// four checks under DefaultConfig, demonstrating every deficiency in one
// verdict: 120 labeled samples with a 30-sample out-of-sample window (both
// below minimum); only two folds, one of them with lookahead, and an
// out-of-sample collapse (mean 0.004 retains only ~13% of the in-sample mean
// 0.03); an 80% proposed parameter step with wildly fold-unstable fitted
// values (CV ~0.64) plus a proposal-only parameter that is noted; and an
// out-of-sample t-statistic of ~0.44 against a 200-trial deflated bar of
// ~3.26.
func SyntheticEvidenceFail() Evidence {
	month := func(m time.Month) time.Time {
		return time.Date(2024, m, 1, 0, 0, 0, 0, time.UTC)
	}

	folds := []FoldResult{
		{
			Index:      1,
			TrainStart: month(time.January),
			TrainEnd:   month(time.March),
			TestStart:  month(time.February), // lookahead: test starts inside the train window
			TestEnd:    month(time.April),
			Params:     map[string]float64{"stop_pct": 0.020},
			TestMetric: 0.010,
		},
		{
			Index:      2,
			TrainStart: month(time.February),
			TrainEnd:   month(time.April),
			TestStart:  month(time.April),
			TestEnd:    month(time.May),
			Params:     map[string]float64{"stop_pct": 0.090},
			TestMetric: -0.005,
		},
	}

	return Evidence{
		ProposalID:   syntheticFailProposalID,
		ProposalKind: ProposalKindScanner,
		SampleSize:   120,
		TrialsCount:  200,
		InSample: Performance{
			WindowStart: month(time.January),
			WindowEnd:   month(time.May),
			Mean:        0.03,
			StdDev:      0.06,
			SampleSize:  90,
		},
		OutOfSample: Performance{
			WindowStart: month(time.May),
			WindowEnd:   month(time.June),
			Mean:        0.004,
			StdDev:      0.05,
			SampleSize:  30,
		},
		Folds: folds,
		ProposedParams: map[string]float64{
			"stop_pct": 0.090, // +80% step from baseline, far outside the 25% bound
			"new_knob": 1.500, // present only in the proposal: noted, not failed
		},
		BaselineParams: map[string]float64{
			"stop_pct": 0.050,
		},
	}
}
