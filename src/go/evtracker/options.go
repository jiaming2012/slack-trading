package evtracker

// DefaultBucketDays is the default equal-length time-bucket width (in days) used
// when partitioning a group's trades for the EV trend slope.
const DefaultBucketDays = 30

// Options tunes a computation/recompute run. The zero value is valid: BucketDays
// falls back to DefaultBucketDays and a nil Retired set retires nothing.
type Options struct {
	// BucketDays is the slope bucket width in days. Values <= 0 fall back to
	// DefaultBucketDays.
	BucketDays int
	// Retired is the set of strategy ids to exclude entirely. A retired strategy
	// yields no result and no persisted row (effective weight 0).
	Retired map[string]bool
}

// bucketDays returns the effective bucket width, applying the default when unset
// or non-positive.
func (o Options) bucketDays() int {
	if o.BucketDays <= 0 {
		return DefaultBucketDays
	}
	return o.BucketDays
}

// isRetired reports whether the strategy id is in the retired set.
func (o Options) isRetired(strategyID string) bool {
	return o.Retired != nil && o.Retired[strategyID]
}
