package crowding

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateCrowdingDetection creates only the two crowding-detection tables
// (crowding_metrics, crowding_flagged_candidates) and their foreign keys. It
// deliberately does NOT touch any trading-stack-schema table or any existing
// playground table, and is not wired into any other migration entry point —
// callers invoke it explicitly.
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// foreign keys are added inside a DO block that swallows duplicate_object
// errors, so a second run leaves the schema unchanged.
func MigrateCrowdingDetection(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&CrowdingMetric{},
		&CrowdingFlaggedCandidate{},
	); err != nil {
		return fmt.Errorf("MigrateCrowdingDetection: auto-migrate failed: %w", err)
	}

	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE crowding_flagged_candidates
				ADD CONSTRAINT fk_crowding_flagged_candidates_metric
				FOREIGN KEY (crowding_metric_id) REFERENCES crowding_metrics (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE crowding_flagged_candidates
				ADD CONSTRAINT fk_crowding_flagged_candidates_scan_result
				FOREIGN KEY (scan_result_id) REFERENCES scan_results (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("MigrateCrowdingDetection: constraint setup failed: %w", err)
		}
	}

	return nil
}
