package crowding

import (
	"errors"
	"time"
)

// ErrEmptyCandidates is returned by DetectCrowding when called with an empty
// candidate slice, since an overlap percentage over zero candidates is
// undefined.
var ErrEmptyCandidates = errors.New("crowding: candidates must not be empty")

// CrowdingResult is the output of DetectCrowding for a single scan cycle (all
// candidates sharing one ScannedAt).
type CrowdingResult struct {
	ScannedAt                   time.Time
	TotalCandidates             int
	OverlappingCandidates       int
	OverlapPct                  float64
	ThresholdPct                float64
	Flagged                     bool
	OverlappingCandidateDetails []ScanCandidate
}

// DetectCrowding computes, for a single scan cycle, how many candidates were
// claimed by two or more distinct strategies (via sim_outcomes.strategy_id)
// and whether that overlap percentage exceeds thresholdPct. It is a pure
// function: it performs no I/O and depends only on its arguments, so it is
// exhaustively testable with fixture data alone.
//
// A candidate is "overlapping" when it has two or more distinct entries in
// StrategyIDs. Candidates with zero or one strategy count toward
// TotalCandidates but never OverlappingCandidates. Flagged is true only when
// OverlapPct is strictly greater than thresholdPct; an exact match is not
// flagged.
func DetectCrowding(candidates []ScanCandidate, thresholdPct float64) (CrowdingResult, error) {
	if len(candidates) == 0 {
		return CrowdingResult{}, ErrEmptyCandidates
	}

	var scannedAt time.Time
	overlapping := 0
	details := make([]ScanCandidate, 0)

	for _, c := range candidates {
		if scannedAt.IsZero() {
			scannedAt = c.ScannedAt
		}

		if distinctCount(c.StrategyIDs) >= 2 {
			overlapping++
			details = append(details, c)
		}
	}

	total := len(candidates)
	overlapPct := float64(overlapping) / float64(total) * 100.0

	return CrowdingResult{
		ScannedAt:                   scannedAt,
		TotalCandidates:             total,
		OverlappingCandidates:       overlapping,
		OverlapPct:                  overlapPct,
		ThresholdPct:                thresholdPct,
		Flagged:                     overlapPct > thresholdPct,
		OverlappingCandidateDetails: details,
	}, nil
}

// distinctCount returns the number of distinct, non-empty strategy ids.
func distinctCount(strategyIDs []string) int {
	seen := make(map[string]struct{}, len(strategyIDs))
	for _, id := range strategyIDs {
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
	}
	return len(seen)
}
