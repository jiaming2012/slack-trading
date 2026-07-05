package tradingstack

import "errors"

// Sentinel errors for the trading-stack domain-model validation hooks.
var (
	// ErrScanDataAsOfAfterScannedAt is returned when a ScanResult's data_as_of
	// timestamp is strictly after its scanned_at timestamp, violating the
	// data_as_of <= scanned_at invariant.
	ErrScanDataAsOfAfterScannedAt = errors.New("tradingstack: scan_results data_as_of must be <= scanned_at")

	// ErrInvalidExitReason is returned when a SimOutcome's exit_reason is not
	// one of the allowed values {stop, target, timeout, signal_exit}.
	ErrInvalidExitReason = errors.New("tradingstack: sim_outcomes exit_reason must be one of stop, target, timeout, signal_exit")
)
