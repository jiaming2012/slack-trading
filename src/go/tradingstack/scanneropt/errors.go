// Package scanneropt is the scanner optimizer: a deterministic,
// statistics-based tuner (v1) that consumes the clean weighted dataset
// produced by the optimizer-validation-pipeline and emits gated
// scannercfg.Payload proposals for operator review. Nothing in this package
// auto-applies a configuration: the only code path that writes to
// scanner_configs is the operator promote command.
package scanneropt

import "errors"

// Sentinel errors for the scanner optimizer.
var (
	// ErrNoTrainingRows is returned by RunCycle when the weighted training
	// row set is empty -- there is nothing to tune.
	ErrNoTrainingRows = errors.New("scanneropt: no weighted training rows to tune on")

	// ErrInsufficientRows is returned when the row set is too small to form
	// the chronological in-sample/out-of-sample split and walk-forward folds.
	ErrInsufficientRows = errors.New("scanneropt: too few rows to build walk-forward evidence")

	// ErrProposalNotFound is returned by ProposalStore when no proposal
	// exists for the requested id.
	ErrProposalNotFound = errors.New("scanneropt: proposal not found")

	// ErrProposalNotPending is returned by Promote when the proposal's
	// status is not pending_review -- gate-rejected and already-promoted
	// proposals can never be applied.
	ErrProposalNotPending = errors.New("scanneropt: proposal is not pending review")

	// ErrInvalidProposalStatus is returned when a proposal row carries a
	// status outside {pending_review, rejected_by_gate, promoted}.
	ErrInvalidProposalStatus = errors.New("scanneropt: invalid proposal status")

	// ErrMissingVerdictTable is returned by MigrateScannerOptimizer when the
	// overfitting_verdicts table (the verdict_id foreign-key target) does not
	// exist -- run overfitting.MigrateOverfittingCountermeasures first.
	ErrMissingVerdictTable = errors.New("scanneropt: required table overfitting_verdicts is missing (run overfitting.MigrateOverfittingCountermeasures first)")
)
