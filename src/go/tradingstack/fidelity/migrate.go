package fidelity

import (
	"fmt"

	"gorm.io/gorm"
)

// fidelityPeriodUniqueIndex is the unique index backing Persist's upsert key.
// Exported behavior, not name: re-persisting the same (strategy_id,
// period_start, period_end) updates the existing simulator_fidelity row.
const fidelityPeriodUniqueIndex = "idx_simulator_fidelity_strategy_period"

// MigrateFidelityMonitoring idempotently creates the unique index on
// simulator_fidelity (strategy_id, period_start, period_end) that makes
// Persist's per-period upsert possible. It is additive: it does not create,
// alter, or drop any other table, column, row, or constraint — following the
// costmodel.MigrateNetEvCostModel in-package precedent.
//
// It returns ErrSimulatorFidelityTableMissing when simulator_fidelity does not
// exist (trading-stack-schema's tradingstack.MigrateTradingStack has not run).
// Running it twice in succession is a no-op on the second call.
//
// The indexed columns are nullable in the schema; the fidelity monitor always
// populates all three, and Postgres treating NULLs as distinct is acceptable
// for hand-inserted rows (design D4).
func MigrateFidelityMonitoring(db *gorm.DB) error {
	if !db.Migrator().HasTable("simulator_fidelity") {
		return ErrSimulatorFidelityTableMissing
	}

	stmt := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON simulator_fidelity (strategy_id, period_start, period_end);`,
		fidelityPeriodUniqueIndex,
	)
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("fidelity: MigrateFidelityMonitoring: failed to create unique index: %w", err)
	}
	return nil
}
