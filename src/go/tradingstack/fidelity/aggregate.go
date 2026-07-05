package fidelity

import (
	"sort"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// Result is the per-strategy fidelity outcome for a period. It carries the
// aggregated drift figures, the composite score, and the tolerance verdict, and
// maps field-for-field onto tradingstack.SimulatorFidelity.
type Result struct {
	StrategyID      string
	Period          Period
	DriftPnL        float64
	DriftFill       float64
	DriftScore      float64
	WithinTolerance bool
}

// Aggregate groups matched pairs by strategy id and produces exactly one Result
// per strategy. Pairs are compared into per-pair deltas, reduced into a
// composite via Score, and the tolerance verdict is drift_score <= threshold.
// Results are returned sorted by strategy id so the output is deterministic.
func Aggregate(pairs []Pair, period Period, cfg FidelityConfig) []Result {
	deltasByStrategy := map[string][]PairDelta{}
	for _, p := range pairs {
		sid := p.Live.StrategyID
		deltasByStrategy[sid] = append(deltasByStrategy[sid], ComparePair(p))
	}

	strategyIDs := make([]string, 0, len(deltasByStrategy))
	for sid := range deltasByStrategy {
		strategyIDs = append(strategyIDs, sid)
	}
	sort.Strings(strategyIDs)

	results := make([]Result, 0, len(strategyIDs))
	for _, sid := range strategyIDs {
		comp := Score(deltasByStrategy[sid], cfg)
		results = append(results, Result{
			StrategyID:      sid,
			Period:          period,
			DriftPnL:        comp.DriftPnL,
			DriftFill:       comp.DriftFill,
			DriftScore:      comp.DriftScore,
			WithinTolerance: comp.DriftScore <= cfg.ToleranceThreshold,
		})
	}
	return results
}

// ToRecord maps a Result field-for-field onto a tradingstack.SimulatorFidelity
// so it can be persisted through the trading-stack-schema. The pointer fields on
// the model are populated from copies of the Result's values.
func (r Result) ToRecord() tradingstack.SimulatorFidelity {
	strategyID := r.StrategyID
	periodStart := r.Period.Start
	periodEnd := r.Period.End
	driftPnL := r.DriftPnL
	driftFill := r.DriftFill
	driftScore := r.DriftScore
	return tradingstack.SimulatorFidelity{
		StrategyID:      &strategyID,
		PeriodStart:     &periodStart,
		PeriodEnd:       &periodEnd,
		DriftPnl:        &driftPnL,
		DriftFill:       &driftFill,
		DriftScore:      &driftScore,
		WithinTolerance: r.WithinTolerance,
	}
}
