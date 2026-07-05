package optvalidation

import "errors"

// Sentinel errors for the optimizer validation pipeline.
var (
	// ErrEmptyBatch is returned by Run when the input training row batch is
	// empty -- there is nothing to validate.
	ErrEmptyBatch = errors.New("optvalidation: input training row batch is empty")
)
