package fidelity

import "errors"

// Sentinel errors for the fidelity checker.
var (
	// ErrNoLiveTrades is returned when a fidelity check is requested for a
	// period whose live-trade set is empty. It signals a clean no-input outcome
	// (not a failure): there is nothing to compare, so no fidelity records are
	// produced.
	ErrNoLiveTrades = errors.New("fidelity: no live trades available for the period")
)
