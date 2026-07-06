package fidelity

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// Persist writes one simulator_fidelity row per Result through GORM, stamped
// with computedAt. It upserts on the unique key (strategy_id, period_start,
// period_end) — created by MigrateFidelityMonitoring — so re-persisting the
// same strategy and period updates the existing row's computed_at, drift
// fields, and within_tolerance instead of inserting a duplicate. Distinct
// periods accumulate history. It is a thin shell around the pure engine;
// callers that only need the in-memory results need not call it.
func Persist(db *gorm.DB, results []Result, computedAt time.Time) error {
	for _, r := range results {
		record := r.ToRecord()
		stamp := computedAt
		record.ComputedAt = &stamp
		err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "strategy_id"}, {Name: "period_start"}, {Name: "period_end"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"computed_at", "drift_pnl", "drift_fill", "drift_score", "within_tolerance",
			}),
		}).Create(&record).Error
		if err != nil {
			return fmt.Errorf("fidelity: persist simulator_fidelity for strategy %q: %w", r.StrategyID, err)
		}
	}
	return nil
}
