package overfitting

import "fmt"

// RunGate runs the four overfitting checks in the fixed order
// minimum-sample -> walk-forward -> parameter-stability -> deflated-performance
// over the caller-supplied evidence and returns a Verdict whose Passed is true
// only when every check passed. All four checks always run (no short-circuit),
// so a failing proposal's verdict shows every deficiency at once.
//
// RunGate is deterministic: the same evidence and config always produce an
// identical verdict. It performs no I/O. An evidence value whose ProposalKind
// is neither "scanner" nor "strategy" returns ErrUnknownProposalKind and no
// verdict.
func RunGate(evidence Evidence, cfg Config) (Verdict, error) {
	if !evidence.ProposalKind.Valid() {
		return Verdict{}, fmt.Errorf("%w: %q", ErrUnknownProposalKind, evidence.ProposalKind)
	}

	checks := []CheckResult{
		checkMinSamples(evidence, cfg),
		checkWalkForward(evidence, cfg),
		checkParamStability(evidence, cfg),
		checkDeflated(evidence, cfg),
	}

	passed := true
	for _, c := range checks {
		passed = passed && c.Passed
	}

	return Verdict{
		ProposalID:   evidence.ProposalID,
		ProposalKind: evidence.ProposalKind,
		Passed:       passed,
		Checks:       checks,
	}, nil
}
