package feedhealth

import "errors"

// Sentinel errors for the feedhealth domain.
var (
	// ErrInvalidThresholdOrdering is returned when a configured asset class's
	// staleness_threshold is not strictly greater than its expected_interval.
	ErrInvalidThresholdOrdering = errors.New("feedhealth: staleness_threshold must be strictly greater than expected_interval")

	// ErrConfigNotFound is returned when the threshold config file cannot be
	// read from the resolved path.
	ErrConfigNotFound = errors.New("feedhealth: threshold config file not found")
)
