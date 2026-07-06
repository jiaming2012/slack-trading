package overfitting

import (
	"fmt"
	"math"
)

// CheckNameDeflated names the deflated-performance check's CheckResult.
const CheckNameDeflated = "deflated_performance"

// checkDeflated computes the out-of-sample t-statistic
//
//	t = OutOfSample.Mean / (OutOfSample.StdDev / sqrt(OutOfSample.SampleSize))
//
// and passes only when t >= max(cfg.MinTStat, sqrt(2*ln(max(TrialsCount, 1)))).
// sqrt(2*ln(N)) is the asymptotic expected maximum of N i.i.d. standard
// normals -- the bar an N-way search "wins" by luck alone -- so the bar rises
// with the number of parameterizations searched, deflating selection bias;
// MinTStat keeps a floor of plain statistical significance when TrialsCount is
// small (ln(1) = 0). The check fails closed when OutOfSample.StdDev <= 0 or
// OutOfSample.SampleSize < 2: without honest dispersion evidence there is no
// t-statistic to trust.
func checkDeflated(e Evidence, cfg Config) CheckResult {
	trials := e.TrialsCount
	if trials < 1 {
		trials = 1
	}
	bar := math.Max(cfg.MinTStat, math.Sqrt(2*math.Log(float64(trials))))

	observed := map[string]float64{
		"t_stat":                0,
		"trials_count":          float64(trials),
		"out_of_sample_std_dev": e.OutOfSample.StdDev,
		"out_of_sample_samples": float64(e.OutOfSample.SampleSize),
	}
	threshold := map[string]float64{
		"effective_t_stat_bar": bar,
		"min_t_stat":           cfg.MinTStat,
	}

	if e.OutOfSample.StdDev <= 0 {
		return CheckResult{
			Name:      CheckNameDeflated,
			Passed:    false,
			Observed:  observed,
			Threshold: threshold,
			Detail: fmt.Sprintf(
				"no honest dispersion evidence: out-of-sample std dev %.6f is not strictly positive",
				e.OutOfSample.StdDev),
		}
	}
	if e.OutOfSample.SampleSize < 2 {
		return CheckResult{
			Name:      CheckNameDeflated,
			Passed:    false,
			Observed:  observed,
			Threshold: threshold,
			Detail: fmt.Sprintf(
				"no honest dispersion evidence: out-of-sample sample size %d is below 2",
				e.OutOfSample.SampleSize),
		}
	}

	tStat := e.OutOfSample.Mean / (e.OutOfSample.StdDev / math.Sqrt(float64(e.OutOfSample.SampleSize)))
	observed["t_stat"] = tStat

	passed := tStat >= bar
	detail := fmt.Sprintf(
		"out-of-sample t-statistic %.4f vs deflated bar %.4f (max of min t-stat %.4f and sqrt(2*ln(%d)) = %.4f)",
		tStat, bar, cfg.MinTStat, trials, math.Sqrt(2*math.Log(float64(trials))))

	return CheckResult{
		Name:      CheckNameDeflated,
		Passed:    passed,
		Observed:  observed,
		Threshold: threshold,
		Detail:    detail,
	}
}
