package stratopt

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// triggerStat recomputes a rule's trigger statistic over an arbitrary window
// of gated outcomes -- the per-fold parameter the gate's cross-fold stability
// check verifies, so the condition driving a proposal must be temporally
// stable, not an artifact of one period.
func triggerStat(rule string, rows []GatedOutcome, cfg OptimizerConfig) float64 {
	switch rule {
	case RuleStopChurn:
		return quickStopShareOfLosses(rows, cfg.StopChurnMaxHoldDays)
	case RuleTimeoutDrag:
		return timeoutShareOfLosses(rows)
	case RuleFastTarget:
		return fastTargetShareOfWins(rows, cfg.FastTargetMaxHoldDays)
	}
	return 0
}

// BuildEvidence constructs the overfitting-gate evidence for one candidate
// from the same gated outcome rows the candidate was derived from
// (group.Outcomes, chronologically sorted by SimulatedAt):
//
//   - The rows are partitioned chronologically into cfg.FoldCount+1
//     contiguous windows of (near-)equal size. Fold i (1..FoldCount) trains
//     on window i and tests on window i+1, so TestStart >= TrainEnd holds by
//     construction. Each fold's Params carry the rule's trigger statistic
//     recomputed on the fold's train window; TestMetric is the test window's
//     ev-weighted mean PnlPct.
//   - InSample is all but the most recent window; OutOfSample is the most
//     recent window (ev-weighted mean, weighted population std dev, and row
//     count of PnlPct each).
//   - The parameter step uses the multiplier encoding of the candidate's
//     relative adjustment: BaselineParams = {parameter: 1.0}, ProposedParams
//     = {parameter: 1.0 + adjustment}, so the gate's relative-step bound
//     bounds the adjustment magnitude directly.
//   - TrialsCount is 1: rule-based generation is a direct deterministic
//     derivation, not a search.
//   - SampleSize is the group's decided (non-breakeven) outcome count -- the
//     samples the candidate was derived from, re-verifying the same bar as
//     the candidate-generation pre-filter.
//
// Deterministic: identical group, candidate, proposalID, and config always
// produce identical evidence. No clock, randomness, database, or network.
func BuildEvidence(group GroupStats, candidate Candidate, proposalID uuid.UUID, cfg OptimizerConfig) (overfitting.Evidence, error) {
	rows := group.Outcomes
	windowCount := cfg.FoldCount + 1
	if cfg.FoldCount < 1 {
		return overfitting.Evidence{}, fmt.Errorf("%w: fold count %d must be at least 1", ErrInsufficientRows, cfg.FoldCount)
	}
	if len(rows) < windowCount {
		return overfitting.Evidence{}, fmt.Errorf("%w: %d gated outcomes for %d chronological windows",
			ErrInsufficientRows, len(rows), windowCount)
	}

	windows := splitWindows(rows, windowCount)
	statName := triggerStatName(candidate.Rule)

	folds := make([]overfitting.FoldResult, 0, cfg.FoldCount)
	for i := 1; i <= cfg.FoldCount; i++ {
		train := windows[i-1]
		test := windows[i]

		testMetric, _ := weightedMeanStdDev(test)

		folds = append(folds, overfitting.FoldResult{
			Index:      i,
			TrainStart: train[0].SimulatedAt,
			TrainEnd:   train[len(train)-1].SimulatedAt,
			TestStart:  test[0].SimulatedAt,
			TestEnd:    test[len(test)-1].SimulatedAt,
			Params:     map[string]float64{statName: triggerStat(candidate.Rule, train, cfg)},
			TestMetric: testMetric,
		})
	}

	inSampleRows := rows[:windowStart(len(rows), windowCount, windowCount-1)]
	outOfSampleRows := windows[windowCount-1]

	inMean, inStdDev := weightedMeanStdDev(inSampleRows)
	oosMean, oosStdDev := weightedMeanStdDev(outOfSampleRows)

	return overfitting.Evidence{
		ProposalID:   proposalID,
		ProposalKind: overfitting.ProposalKindStrategy,
		SampleSize:   group.DecidedCount,
		TrialsCount:  1,
		InSample: overfitting.Performance{
			WindowStart: inSampleRows[0].SimulatedAt,
			WindowEnd:   inSampleRows[len(inSampleRows)-1].SimulatedAt,
			Mean:        inMean,
			StdDev:      inStdDev,
			SampleSize:  len(inSampleRows),
		},
		OutOfSample: overfitting.Performance{
			WindowStart: outOfSampleRows[0].SimulatedAt,
			WindowEnd:   outOfSampleRows[len(outOfSampleRows)-1].SimulatedAt,
			Mean:        oosMean,
			StdDev:      oosStdDev,
			SampleSize:  len(outOfSampleRows),
		},
		Folds:          folds,
		ProposedParams: map[string]float64{candidate.Parameter: 1.0 + candidate.AdjustmentPct},
		BaselineParams: map[string]float64{candidate.Parameter: 1.0},
	}, nil
}

// windowStart returns the start index of window i (0-based over windowCount
// contiguous chronological windows of n rows).
func windowStart(n, windowCount, i int) int {
	return n * i / windowCount
}

// splitWindows splits chronologically sorted rows into windowCount contiguous
// windows of (near-)equal size. Every window is non-empty when
// len(rows) >= windowCount.
func splitWindows(rows []GatedOutcome, windowCount int) [][]GatedOutcome {
	windows := make([][]GatedOutcome, windowCount)
	for i := 0; i < windowCount; i++ {
		start := windowStart(len(rows), windowCount, i)
		end := windowStart(len(rows), windowCount, i+1)
		windows[i] = rows[start:end]
	}
	return windows
}
