package evtracker

import "sort"

// RankByEV sorts results by descending all-time EV and assigns each a 1-based
// Rank in that order. Ties break deterministically by StrategyID then Regime
// (both ascending). The input slice is sorted in place and returned.
func RankByEV(results []StrategyEVResult) []StrategyEVResult {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.EvAllTime != b.EvAllTime {
			return a.EvAllTime > b.EvAllTime
		}
		if a.StrategyID != b.StrategyID {
			return a.StrategyID < b.StrategyID
		}
		return a.Regime < b.Regime
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	return results
}
