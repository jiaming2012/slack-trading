package optvalidation

import (
	"math"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// DistributionReport is the advisory output of the distribution-check stage:
// which named features (of those the baseline declares) drifted more than
// cfg.DistributionDriftStdDevs standard deviations from their
// training-window baseline mean, and which did not.
type DistributionReport struct {
	Drifted    []string
	NotDrifted []string
}

// featureAccessors maps a feature_distributions.feature_name value to a
// function extracting that feature's numeric value from a TrainingRow. The
// names mirror the scan_results columns the features are computed from.
var featureAccessors = map[string]func(TrainingRow) float64{
	"price":         func(r TrainingRow) float64 { return r.Price },
	"volume_ratio":  func(r TrainingRow) float64 { return r.VolumeRatio },
	"rsi_14":        func(r TrainingRow) float64 { return r.RSI14 },
	"atr_pct":       func(r TrainingRow) float64 { return r.ATRPct },
	"scanner_score": func(r TrainingRow) float64 { return r.ScannerScore },
}

// DistributionCheck computes, for each named feature present in baseline,
// the mean of that feature across rows and flags it as drifted when the
// absolute difference between the current mean and the baseline mean
// exceeds cfg.DistributionDriftStdDevs times the baseline's std dev. This
// stage is advisory only: it never removes or mutates any TrainingRow. An
// empty baseline (the expected state before feature_distributions has
// accrued any history) yields an empty, no-drift report.
func DistributionCheck(rows []TrainingRow, baseline []tradingstack.FeatureDistribution, cfg Config) DistributionReport {
	report := DistributionReport{
		Drifted:    []string{},
		NotDrifted: []string{},
	}

	for _, b := range baseline {
		if b.FeatureName == nil || b.Mean == nil || b.StdDev == nil {
			continue
		}

		accessor, ok := featureAccessors[*b.FeatureName]
		if !ok {
			continue
		}

		currentMean := meanOf(rows, accessor)
		drift := math.Abs(currentMean - *b.Mean)
		threshold := cfg.DistributionDriftStdDevs * *b.StdDev

		if drift > threshold {
			report.Drifted = append(report.Drifted, *b.FeatureName)
		} else {
			report.NotDrifted = append(report.NotDrifted, *b.FeatureName)
		}
	}

	return report
}

func meanOf(rows []TrainingRow, accessor func(TrainingRow) float64) float64 {
	if len(rows) == 0 {
		return 0
	}

	var sum float64
	for _, r := range rows {
		sum += accessor(r)
	}

	return sum / float64(len(rows))
}
