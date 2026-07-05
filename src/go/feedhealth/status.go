package feedhealth

// FeedHealthStatus is the three-state health of a Feed for a given asset
// class, derived from elapsed time since the last observed tick relative to
// its configured thresholds.
type FeedHealthStatus int

const (
	// Healthy means elapsed time since the last observed tick is within the
	// asset class's expected_interval.
	Healthy FeedHealthStatus = iota
	// Degraded means elapsed time exceeds expected_interval but is still
	// within staleness_threshold.
	Degraded
	// Stale means elapsed time exceeds staleness_threshold, or the asset
	// class has never been observed.
	Stale
)

// String returns the lowercase string form of the status, matching the
// values persisted via TagScanResult ("healthy", "degraded", "stale").
func (s FeedHealthStatus) String() string {
	switch s {
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Stale:
		return "stale"
	default:
		return "unknown"
	}
}
