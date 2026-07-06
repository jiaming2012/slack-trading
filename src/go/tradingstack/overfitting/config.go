package overfitting

// Config carries every gate threshold. All values are configurable so each
// consuming optimizer can supply per-kind thresholds (e.g. the
// strategy-optimizer's thinner per-group data volumes) without code change;
// DefaultConfig returns the conservative starting points signed off in the
// overfitting-countermeasures change.
type Config struct {
	// MinSamples is the minimum count of decided labeled samples
	// (Evidence.SampleSize) required by the minimum-sample gate.
	MinSamples int

	// MinOOSSamples is the minimum out-of-sample window size
	// (Evidence.OutOfSample.SampleSize) required by the minimum-sample gate.
	MinOOSSamples int

	// MinFolds is the minimum number of walk-forward folds required by the
	// walk-forward check.
	MinFolds int

	// OOSRetentionFraction is the fraction of in-sample performance that
	// out-of-sample performance must retain (OutOfSample.Mean >=
	// OOSRetentionFraction * InSample.Mean).
	OOSRetentionFraction float64

	// MaxRelativeStep bounds how far a proposed parameter may move from its
	// non-zero baseline: |proposed - baseline| <= MaxRelativeStep * |baseline|.
	MaxRelativeStep float64

	// MaxFoldParamCV bounds the coefficient of variation (stddev / |mean|) of
	// a parameter's fitted values across walk-forward folds.
	MaxFoldParamCV float64

	// MinTStat is the floor of the deflated-performance bar: the out-of-sample
	// t-statistic must clear max(MinTStat, sqrt(2*ln(max(TrialsCount, 1)))).
	MinTStat float64
}

// DefaultConfig returns the drafted default thresholds: 500 labeled samples
// (the architecture doc's min_labeled_samples), 50 out-of-sample rows, 3
// walk-forward folds, 0.5 out-of-sample retention, a 25% maximum relative
// parameter step, a 0.5 cross-fold coefficient-of-variation bound, and a
// minimum t-statistic of 2.0.
func DefaultConfig() Config {
	return Config{
		MinSamples:           500,
		MinOOSSamples:        50,
		MinFolds:             3,
		OOSRetentionFraction: 0.5,
		MaxRelativeStep:      0.25,
		MaxFoldParamCV:       0.5,
		MinTStat:             2.0,
	}
}
