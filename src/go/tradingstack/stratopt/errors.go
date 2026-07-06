// Package stratopt is the strategy optimizer: a deterministic, rule-based
// recommendation engine (v1) that turns validation-pipeline-gated sim-outcome
// history into per-(strategy_id, regime) execution-parameter proposals for
// operator review. Every generated candidate is submitted to the shared
// overfitting gate (src/go/tradingstack/overfitting, proposal_kind =
// "strategy") before it can enter the review queue.
//
// Proposals are recommendations only: no code path in this package -- or
// anywhere else -- reads strategy_proposals to modify any strategy
// configuration, playground, order flow, or trading behavior. Recording a
// decision changes only the proposal row itself.
package stratopt

import "errors"

// Sentinel errors for the strategy optimizer.
var (
	// ErrPipelineFailed is returned by GenerateRun when the
	// optimizer-validation-pipeline returns an error -- the run aborts with
	// zero proposals.
	ErrPipelineFailed = errors.New("stratopt: optimizer validation pipeline failed")

	// ErrGateFailed is returned when the overfitting gate run or its verdict
	// persistence fails internally (NOT when a proposal merely fails the
	// gate's checks -- that is a normal rejected_by_gate outcome).
	ErrGateFailed = errors.New("stratopt: overfitting gate submission failed")

	// ErrInsufficientRows is returned by BuildEvidence when a group has too
	// few gated outcomes to form the chronological fold partition.
	ErrInsufficientRows = errors.New("stratopt: too few gated outcomes to build walk-forward evidence")

	// ErrProposalNotFound is returned when no proposal exists for the
	// requested id.
	ErrProposalNotFound = errors.New("stratopt: proposal not found")

	// ErrProposalNotDecidable is returned by DecideProposal when the
	// proposal's status is not pending_review -- gate-rejected and
	// already-decided proposals can never be decided (again).
	ErrProposalNotDecidable = errors.New("stratopt: proposal is not pending review")

	// ErrInvalidProposalStatus is returned when a proposal row carries a
	// status outside {rejected_by_gate, pending_review, accepted, dismissed}.
	ErrInvalidProposalStatus = errors.New("stratopt: invalid proposal status")

	// ErrInvalidDecision is returned by DecideProposal when the decision is
	// neither "accepted" nor "dismissed".
	ErrInvalidDecision = errors.New("stratopt: invalid decision (want accepted or dismissed)")

	// ErrMissingVerdictTable is returned by MigrateStrategyOptimizer when the
	// overfitting_verdicts table (the verdict_id foreign-key target) does not
	// exist -- run overfitting.MigrateOverfittingCountermeasures first.
	ErrMissingVerdictTable = errors.New("stratopt: required table overfitting_verdicts is missing (run overfitting.MigrateOverfittingCountermeasures first)")
)
