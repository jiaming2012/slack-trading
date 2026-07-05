package tradingstack

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateTradingStack creates only the six v4 trading-stack tables and their
// constraints. It deliberately does NOT touch any existing playground table and
// is NOT wired into dbutils.InitPostgres — callers invoke it explicitly so the
// v4 stack owns its own migration lifecycle.
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// CHECK/foreign-key constraints are added inside DO blocks that swallow
// duplicate_object errors, so a second run leaves the schema unchanged.
func MigrateTradingStack(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&ScanResult{},
		&SimOutcome{},
		&SimulatorFidelity{},
		&StrategyEvWeight{},
		&ScannerConfig{},
		&FeatureDistribution{},
	); err != nil {
		return fmt.Errorf("MigrateTradingStack: auto-migrate failed: %w", err)
	}

	// Constraints not expressible via GORM tags without coupling models to
	// associations. Added idempotently via DO blocks that ignore re-adds.
	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE scan_results
				ADD CONSTRAINT chk_scan_results_data_as_of
				CHECK (data_as_of <= scanned_at);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE sim_outcomes
				ADD CONSTRAINT chk_sim_outcomes_exit_reason
				CHECK (exit_reason IN ('stop', 'target', 'timeout', 'signal_exit'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE sim_outcomes
				ADD CONSTRAINT fk_sim_outcomes_scan_result
				FOREIGN KEY (scan_result_id) REFERENCES scan_results (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("MigrateTradingStack: constraint setup failed: %w", err)
		}
	}

	return nil
}
