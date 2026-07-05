package fidelity

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// RunFidelityCheck orchestrates the full pipeline for a period: pair the live
// and simulator trades, compare and aggregate per strategy, and derive the gate
// decisions (raising an alert on each breach via the injected Alerter). It is a
// pure function of its inputs apart from the alert side effect. It returns
// ErrNoLiveTrades when the live set is empty (a clean no-input outcome, not a
// failure). When a is nil the default LogAlerter is used.
func RunFidelityCheck(ctx context.Context, set TradeSet, period Period, cfg FidelityConfig, a Alerter) ([]Result, map[string]Decision, error) {
	if len(set.Live) == 0 {
		return nil, nil, ErrNoLiveTrades
	}
	if a == nil {
		a = LogAlerter{}
	}

	pairs, _ := PairTrades(set.Live, set.Sim)
	results := Aggregate(pairs, period, cfg)
	decisions := Gate(ctx, results, a)
	return results, decisions, nil
}

// Persist writes one simulator_fidelity row per Result through GORM. It is a
// thin shell around the pure engine; callers that only need the in-memory
// results need not call it.
func Persist(db *gorm.DB, results []Result) error {
	for _, r := range results {
		record := r.ToRecord()
		if err := db.Create(&record).Error; err != nil {
			return fmt.Errorf("fidelity: persist simulator_fidelity for strategy %q: %w", r.StrategyID, err)
		}
	}
	return nil
}
