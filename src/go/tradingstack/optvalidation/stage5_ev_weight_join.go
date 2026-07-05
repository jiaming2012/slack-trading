package optvalidation

import (
	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// defaultEVWeight is the neutral weight assigned to a surviving TrainingRow
// when no StrategyEvWeight record matches its (strategy, regime) -- the
// normal state before ev-tracker has accrued any history. It is never zero,
// so an unmatched row is neither dropped nor silently down-weighted to
// nothing.
const defaultEVWeight = 1.0

type evWeightKey struct {
	strategyID string
	regime     string
}

// EVWeightJoin attaches the most recently computed matching ev_weight
// (matched by strategy id and regime) from weights to each row, producing a
// WeightedTrainingRow. When no matching StrategyEvWeight record exists, the
// row receives defaultEVWeight (1.0) rather than being dropped or
// zero-weighted.
func EVWeightJoin(rows []TrainingRow, weights []tradingstack.StrategyEvWeight) []WeightedTrainingRow {
	latest := make(map[evWeightKey]tradingstack.StrategyEvWeight)

	for _, w := range weights {
		if w.StrategyID == nil || w.Regime == nil || w.EvWeight == nil {
			continue
		}

		key := evWeightKey{strategyID: *w.StrategyID, regime: *w.Regime}
		existing, ok := latest[key]
		if !ok || isMoreRecentlyComputed(w, existing) {
			latest[key] = w
		}
	}

	out := make([]WeightedTrainingRow, 0, len(rows))
	for _, r := range rows {
		weight := defaultEVWeight
		if w, ok := latest[evWeightKey{strategyID: r.StrategyID, regime: r.RegimeTag}]; ok {
			weight = *w.EvWeight
		}
		out = append(out, WeightedTrainingRow{TrainingRow: r, EVWeight: weight})
	}

	return out
}

// isMoreRecentlyComputed reports whether candidate's ComputedAt is strictly
// after existing's. A nil ComputedAt is treated as older than any concrete
// timestamp.
func isMoreRecentlyComputed(candidate, existing tradingstack.StrategyEvWeight) bool {
	if candidate.ComputedAt == nil {
		return false
	}
	if existing.ComputedAt == nil {
		return true
	}
	return candidate.ComputedAt.After(*existing.ComputedAt)
}
