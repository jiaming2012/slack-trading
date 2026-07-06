package stratopt

import (
	"math"
	"sort"
)

// GroupStats are the deterministic ev-weight-weighted statistics of one
// (strategy_id, regime) group of gated outcomes. Every statistic uses each
// outcome's EVWeight as its row weight, so rows the pipeline trusts less
// influence proposals less. Breakeven outcomes (PnlPct == 0) are excluded
// from win/loss counts and rates, and AvgLossPct is a positive magnitude --
// both matching the ev-computation conventions.
type GroupStats struct {
	StrategyID string
	Regime     string

	// Outcomes are the group's gated outcomes, chronologically sorted
	// (SimulatedAt, then SimOutcomeID), including breakevens. Gate evidence
	// is built from exactly these rows.
	Outcomes []GatedOutcome

	// DecidedCount counts non-breakeven outcomes -- the sample count the
	// minimum-sample pre-filter and the gate's evidence SampleSize use.
	DecidedCount int

	// WeightedEV is the architecture formula
	// (win_rate x avg_win) - (loss_rate x avg_loss) over weighted rates and
	// weighted mean magnitudes.
	WeightedEV float64

	WinRate    float64 // weighted share of decided outcomes with PnlPct > 0
	LossRate   float64 // weighted share of decided outcomes with PnlPct < 0
	AvgWinPct  float64 // weighted mean PnlPct among wins
	AvgLossPct float64 // weighted mean |PnlPct| among losses (positive magnitude)

	// Rule trigger statistics: weighted exit-reason shares among losses and
	// among wins, with the configured hold-day cutoffs applied.
	QuickStopShareOfLosses float64
	TimeoutShareOfLosses   float64
	FastTargetShareOfWins  float64
}

// ComputeGroupStats groups gated outcomes by (strategy_id, regime) and
// computes each group's weighted statistics independently -- no outcome from
// one group influences another. Pure function: no clock, randomness,
// database, or network. The result is sorted by (strategy_id, regime) so
// identical inputs always produce identical output.
func ComputeGroupStats(outcomes []GatedOutcome, cfg OptimizerConfig) []GroupStats {
	type key struct{ strategyID, regime string }

	grouped := make(map[key][]GatedOutcome)
	for _, o := range outcomes {
		k := key{strategyID: o.StrategyID, regime: o.Regime}
		grouped[k] = append(grouped[k], o)
	}

	keys := make([]key, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].strategyID != keys[j].strategyID {
			return keys[i].strategyID < keys[j].strategyID
		}
		return keys[i].regime < keys[j].regime
	})

	groups := make([]GroupStats, 0, len(keys))
	for _, k := range keys {
		rows := grouped[k]
		sort.SliceStable(rows, func(i, j int) bool {
			if !rows[i].SimulatedAt.Equal(rows[j].SimulatedAt) {
				return rows[i].SimulatedAt.Before(rows[j].SimulatedAt)
			}
			return rows[i].SimOutcomeID.String() < rows[j].SimOutcomeID.String()
		})

		g := GroupStats{
			StrategyID: k.strategyID,
			Regime:     k.regime,
			Outcomes:   rows,
		}

		var winWeight, lossWeight float64
		var winPnlWeighted, lossPnlWeighted float64
		for _, o := range rows {
			switch {
			case o.PnlPct > 0:
				g.DecidedCount++
				winWeight += o.EVWeight
				winPnlWeighted += o.EVWeight * o.PnlPct
			case o.PnlPct < 0:
				g.DecidedCount++
				lossWeight += o.EVWeight
				lossPnlWeighted += o.EVWeight * math.Abs(o.PnlPct)
			}
			// PnlPct == 0: breakeven, excluded from win and loss counts.
		}

		decidedWeight := winWeight + lossWeight
		if decidedWeight > 0 {
			g.WinRate = winWeight / decidedWeight
			g.LossRate = lossWeight / decidedWeight
		}
		if winWeight > 0 {
			g.AvgWinPct = winPnlWeighted / winWeight
		}
		if lossWeight > 0 {
			g.AvgLossPct = lossPnlWeighted / lossWeight
		}
		g.WeightedEV = g.WinRate*g.AvgWinPct - g.LossRate*g.AvgLossPct

		g.QuickStopShareOfLosses = quickStopShareOfLosses(rows, cfg.StopChurnMaxHoldDays)
		g.TimeoutShareOfLosses = timeoutShareOfLosses(rows)
		g.FastTargetShareOfWins = fastTargetShareOfWins(rows, cfg.FastTargetMaxHoldDays)

		groups = append(groups, g)
	}

	return groups
}

// quickStopShareOfLosses is the weighted share of losses that exited via
// "stop" with HoldDays <= maxHoldDays. Zero when the window has no losses.
func quickStopShareOfLosses(rows []GatedOutcome, maxHoldDays int) float64 {
	var matched, total float64
	for _, o := range rows {
		if o.PnlPct >= 0 {
			continue
		}
		total += o.EVWeight
		if o.ExitReason == "stop" && o.HoldDays <= maxHoldDays {
			matched += o.EVWeight
		}
	}
	if total == 0 {
		return 0
	}
	return matched / total
}

// timeoutShareOfLosses is the weighted share of losses that exited via
// "timeout". Zero when the window has no losses.
func timeoutShareOfLosses(rows []GatedOutcome) float64 {
	var matched, total float64
	for _, o := range rows {
		if o.PnlPct >= 0 {
			continue
		}
		total += o.EVWeight
		if o.ExitReason == "timeout" {
			matched += o.EVWeight
		}
	}
	if total == 0 {
		return 0
	}
	return matched / total
}

// fastTargetShareOfWins is the weighted share of wins that exited via
// "target" with HoldDays <= maxHoldDays. Zero when the window has no wins.
func fastTargetShareOfWins(rows []GatedOutcome, maxHoldDays int) float64 {
	var matched, total float64
	for _, o := range rows {
		if o.PnlPct <= 0 {
			continue
		}
		total += o.EVWeight
		if o.ExitReason == "target" && o.HoldDays <= maxHoldDays {
			matched += o.EVWeight
		}
	}
	if total == 0 {
		return 0
	}
	return matched / total
}

// weightedMeanStdDev returns the ev-weighted mean and weighted population
// standard deviation of PnlPct over rows. An empty window or zero total
// weight returns (0, 0).
func weightedMeanStdDev(rows []GatedOutcome) (mean, stdDev float64) {
	var totalWeight, weightedSum float64
	for _, o := range rows {
		totalWeight += o.EVWeight
		weightedSum += o.EVWeight * o.PnlPct
	}
	if totalWeight == 0 {
		return 0, 0
	}
	mean = weightedSum / totalWeight

	var weightedSumSq float64
	for _, o := range rows {
		d := o.PnlPct - mean
		weightedSumSq += o.EVWeight * d * d
	}
	return mean, math.Sqrt(weightedSumSq / totalWeight)
}
