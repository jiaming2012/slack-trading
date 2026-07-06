// Package overfitting is the shared, deterministic anti-overfitting gate that
// every optimizer proposal (strategy-optimizer, scanner-optimizer) must pass
// through before it can be presented for operator review.
//
// The gate runs four checks in a fixed order over a caller-supplied Evidence
// value -- minimum samples, walk-forward/out-of-sample retention, parameter
// stability, and a deflated (multiple-testing-adjusted) t-statistic bar -- and
// returns a single Verdict. Every check is a pure function of
// (Evidence, Config): no clock, no database, no network, no randomness, so the
// same evidence and config always produce an identical verdict.
package overfitting

import "errors"

// Sentinel errors for the overfitting gate.
var (
	// ErrUnknownProposalKind is returned by RunGate when the evidence's
	// ProposalKind is neither ProposalKindScanner nor ProposalKindStrategy.
	ErrUnknownProposalKind = errors.New("overfitting: unknown proposal kind")

	// ErrVerdictNotFound is returned by VerdictStore.FetchLatestByProposal
	// when no verdict has been persisted for the requested proposal id.
	ErrVerdictNotFound = errors.New("overfitting: no verdict found for proposal")
)
