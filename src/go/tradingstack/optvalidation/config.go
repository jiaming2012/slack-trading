package optvalidation

// Config holds the tunable thresholds for the optimizer validation pipeline.
type Config struct {
	// RegimeConfidenceThreshold is the minimum RegimeConfidence a row must
	// have to survive the regime-confidence-filter stage. The filter drops
	// values strictly less than this threshold; a row exactly at the
	// threshold survives.
	RegimeConfidenceThreshold float64

	// DistributionDriftStdDevs is the number of baseline standard deviations
	// a feature's current batch mean may deviate from the baseline mean
	// before the distribution-check stage flags it as drifted.
	DistributionDriftStdDevs float64
}

// DefaultConfig returns the pipeline's documented default thresholds:
// RegimeConfidenceThreshold 0.7, DistributionDriftStdDevs 2.0.
func DefaultConfig() Config {
	return Config{
		RegimeConfidenceThreshold: 0.7,
		DistributionDriftStdDevs:  2.0,
	}
}
