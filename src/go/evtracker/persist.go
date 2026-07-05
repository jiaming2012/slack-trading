package evtracker

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// Recompute fetches closed-trade outcomes through repo, drops retired strategies
// before computing, runs the EV computation as of asOf, and persists one
// strategy_ev_weights row per resulting (strategy_id, regime) group with
// computed_at = asOf. A null EV slope is stored as SQL NULL.
//
// Persistence is reproducible for a given asOf: any prior rows written for the
// same computed_at are removed before the new rows are inserted, so a re-run
// with changed inputs never leaves stale or duplicated weights. The delete and
// inserts run in a single transaction. Retired strategies produce no row.
func Recompute(ctx context.Context, db *gorm.DB, repo TradeOutcomeRepository, asOf time.Time, opts Options) ([]StrategyEVResult, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	if repo == nil {
		return nil, ErrNilRepo
	}

	trades, err := repo.FetchTradeOutcomes(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("evtracker.Recompute: fetch trade outcomes: %w", err)
	}

	// Drop retired strategies before compute so they yield no result and no row.
	filtered := trades[:0:0]
	for _, t := range trades {
		if opts.isRetired(t.StrategyID) {
			continue
		}
		filtered = append(filtered, t)
	}

	results := Compute(filtered, asOf, opts)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove any prior run at this exact as-of so re-runs replace rather than
		// accumulate, keeping the latest computed_at rows consistent with inputs.
		if delErr := tx.Where("computed_at = ?", asOf).Delete(&tradingstack.StrategyEvWeight{}).Error; delErr != nil {
			return fmt.Errorf("delete prior rows: %w", delErr)
		}

		for _, r := range results {
			row := toWeightRow(r, asOf)
			if createErr := tx.Create(row).Error; createErr != nil {
				return fmt.Errorf("insert row for %s/%s: %w", r.StrategyID, r.Regime, createErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("evtracker.Recompute: persist: %w", err)
	}

	return results, nil
}

// toWeightRow maps a StrategyEVResult onto a persistable StrategyEvWeight row,
// taking fresh pointer copies so no two rows alias the same backing value. A nil
// EvSlope is left nil to persist as SQL NULL.
func toWeightRow(r StrategyEVResult, asOf time.Time) *tradingstack.StrategyEvWeight {
	computedAt := asOf
	strategyID := r.StrategyID
	regime := r.Regime
	ev30 := r.Ev30d
	ev90 := r.Ev90d
	weight := r.EvWeight

	row := &tradingstack.StrategyEvWeight{
		ComputedAt: &computedAt,
		StrategyID: &strategyID,
		Regime:     &regime,
		Ev30d:      &ev30,
		Ev90d:      &ev90,
		EvWeight:   &weight,
	}
	if r.EvSlope != nil {
		slope := *r.EvSlope
		row.EvSlope = &slope
	}
	return row
}
