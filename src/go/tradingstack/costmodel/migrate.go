package costmodel

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateNetEvCostModel idempotently adds gross_ev, net_ev, and
// capacity_shares NUMERIC columns to the existing strategy_ev_weights table
// (created by the trading-stack-schema migration, tradingstack.MigrateTradingStack).
// It does not alter, drop, or recreate any existing column, row, or
// constraint on that table, and does not touch any other table.
//
// It returns ErrStrategyEvWeightsTableMissing if strategy_ev_weights does
// not yet exist — i.e. trading-stack-schema's migration has not run. The
// call is idempotent: running it twice in succession is a no-op on the
// second call.
func MigrateNetEvCostModel(db *gorm.DB) error {
	if !db.Migrator().HasTable("strategy_ev_weights") {
		return ErrStrategyEvWeightsTableMissing
	}

	statements := []string{
		`ALTER TABLE strategy_ev_weights ADD COLUMN IF NOT EXISTS gross_ev NUMERIC;`,
		`ALTER TABLE strategy_ev_weights ADD COLUMN IF NOT EXISTS net_ev NUMERIC;`,
		`ALTER TABLE strategy_ev_weights ADD COLUMN IF NOT EXISTS capacity_shares NUMERIC;`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("costmodel: MigrateNetEvCostModel: failed to add column: %w", err)
		}
	}

	return nil
}
