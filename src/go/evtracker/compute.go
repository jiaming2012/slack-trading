package evtracker

import (
	"sort"
	"time"
)

// StrategyEVResult is the computed EV summary for one (strategy_id, regime)
// group. EvSlope is nil when the group has fewer than two non-empty buckets. Rank
// is assigned by RankByEV on the full result set.
type StrategyEVResult struct {
	StrategyID string
	Regime     string
	Ev30d      float64
	Ev90d      float64
	EvAllTime  float64
	EvSlope    *float64
	EvWeight   float64
	Status     DecayStatus
	Rank       int
}

// groupKey identifies a (strategy_id, regime) computation group.
type groupKey struct {
	StrategyID string
	Regime     string
}

// Compute produces one StrategyEVResult per (strategy_id, regime) group present
// in trades. For each group it computes ev_30d, ev_90d, and ev_all_time relative
// to asOf, fits an EV trend slope over equal-length buckets, classifies the
// weight/status, and ranks the groups by descending all-time EV. Trades closing
// after asOf are excluded from every window and from the slope. Compute is a
// pure function of trades, asOf, and opts.BucketDays — no wall clock, no
// randomness — so identical inputs yield identical outputs.
func Compute(trades []TradeOutcome, asOf time.Time, opts Options) []StrategyEVResult {
	groups := make(map[groupKey][]TradeOutcome)
	for _, t := range trades {
		key := groupKey{StrategyID: t.StrategyID, Regime: t.Regime}
		groups[key] = append(groups[key], t)
	}

	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].StrategyID != keys[j].StrategyID {
			return keys[i].StrategyID < keys[j].StrategyID
		}
		return keys[i].Regime < keys[j].Regime
	})

	results := make([]StrategyEVResult, 0, len(keys))
	for _, key := range keys {
		groupTrades := groups[key]

		allTime := AllTime(groupTrades, asOf)
		ev30 := ComputeEV(Within(groupTrades, asOf, 30)).EV
		ev90 := ComputeEV(Within(groupTrades, asOf, 90)).EV
		evAll := ComputeEV(allTime).EV

		var slopePtr *float64
		if slope, ok := OLSSlope(BucketEVs(allTime, opts.bucketDays())); ok {
			slopePtr = &slope
		}
		weight, status := ClassifyWeight(slopePtr)

		results = append(results, StrategyEVResult{
			StrategyID: key.StrategyID,
			Regime:     key.Regime,
			Ev30d:      ev30,
			Ev90d:      ev90,
			EvAllTime:  evAll,
			EvSlope:    slopePtr,
			EvWeight:   weight,
			Status:     status,
		})
	}

	return RankByEV(results)
}
