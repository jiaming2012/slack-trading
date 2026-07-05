package evtracker

import "errors"

// Sentinel errors for the EV tracker. Empty trade input is deliberately NOT an
// error: it yields zero results. These sentinels guard the persistence entry
// point's required dependencies.
var (
	// ErrNilDB is returned by Recompute when the gorm handle is nil.
	ErrNilDB = errors.New("evtracker: db handle is nil")

	// ErrNilRepo is returned by Recompute when the trade-outcome repository is
	// nil.
	ErrNilRepo = errors.New("evtracker: trade outcome repository is nil")
)
