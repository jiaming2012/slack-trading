package shadowdeploy

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ShadowRun is one persisted shadow evaluation: which configs were compared,
// over which scanned_at window, the selection totals, the divergence
// percentage, and both sides' outcome comparison. The evidence survives for
// the operator's promotion decision and later audit ("what did we know when
// we promoted?").
type ShadowRun struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`

	// ActiveConfigID is the scanner_configs row that served as the active
	// baseline; NULL means the table was empty and the built-in default
	// payload was the baseline.
	ActiveConfigID *uuid.UUID `gorm:"column:active_config_id;type:uuid"`

	// ProposalID references scanner_config_proposals.id -- the shadow
	// candidate (FK added in MigrateShadowDeployment).
	ProposalID uuid.UUID `gorm:"column:proposal_id;type:uuid;not null"`

	// WindowStart/WindowEnd bound the half-open scanned_at window
	// [start, end) that was replayed.
	WindowStart time.Time `gorm:"column:window_start;type:timestamptz;not null"`
	WindowEnd   time.Time `gorm:"column:window_end;type:timestamptz;not null"`

	TotalObservations int `gorm:"column:total_observations;type:integer;not null"`
	SelectedActive    int `gorm:"column:selected_active;type:integer;not null"`
	SelectedShadow    int `gorm:"column:selected_shadow;type:integer;not null"`
	SelectedBoth      int `gorm:"column:selected_both;type:integer;not null"`

	DivergencePct float64 `gorm:"column:divergence_pct;type:numeric;not null"`

	// OutcomeSummaryJSON is the serialized OutcomeComparison (both sides,
	// coverage-explicit, limitation included).
	OutcomeSummaryJSON []byte `gorm:"column:outcome_summary_json;type:jsonb"`

	// Synthetic marks fixture-driven runs. The CLI's --synthetic mode never
	// persists at all; the flag keeps any accidental persistence honest.
	Synthetic bool `gorm:"column:synthetic;not null"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (ShadowRun) TableName() string {
	return "shadow_runs"
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, mirroring the tradingstack BaseModel pattern.
func (r *ShadowRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// ShadowDivergence is one diverging ticker of a shadow run: selected by
// exactly one side, with both sides' scores and a loose scan_results
// reference for drill-down.
type ShadowDivergence struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	ShadowRunID uuid.UUID `gorm:"column:shadow_run_id;type:uuid;not null"`
	Ticker      string    `gorm:"column:ticker;type:text;not null"`

	// Kind is constrained to {shadow_only, active_only} by Go validation
	// (BeforeSave) and a database CHECK constraint.
	Kind string `gorm:"column:kind;type:text;not null"`

	ActiveScore *float64 `gorm:"column:active_score;type:numeric"`
	ShadowScore *float64 `gorm:"column:shadow_score;type:numeric"`

	// ScanResultID is a loose (non-FK) reference to the scan_results row
	// behind the diverging decision.
	ScanResultID uuid.UUID `gorm:"column:scan_result_id;type:uuid"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (ShadowDivergence) TableName() string {
	return "shadow_divergences"
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value.
func (d *ShadowDivergence) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// BeforeSave enforces the kind vocabulary in Go, in front of the database
// CHECK constraint, so an invalid row is rejected before it reaches the wire.
func (d *ShadowDivergence) BeforeSave(tx *gorm.DB) error {
	if !validDivergenceKind(d.Kind) {
		return fmt.Errorf("%w: %q", ErrInvalidDivergenceKind, d.Kind)
	}
	return nil
}

// MigrateShadowDeployment creates only the shadow_runs and shadow_divergences
// tables, their CHECK constraint, and their foreign keys. It requires
// scanneropt.MigrateScannerOptimizer to have run first (shadow_runs
// FKs scanner_config_proposals.id) and returns a clear error naming that
// dependency when the target table is missing. It deliberately does NOT
// create, alter, or drop any other table and is not wired into
// dbutils.InitPostgres -- callers invoke it explicitly, matching the
// tradingstack migration lifecycle convention.
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// constraints are added inside DO blocks that swallow duplicate_object
// errors, so a second run leaves the schema unchanged.
func MigrateShadowDeployment(db *gorm.DB) error {
	if !db.Migrator().HasTable("scanner_config_proposals") {
		return fmt.Errorf("MigrateShadowDeployment: %w", ErrMissingProposalTable)
	}

	if err := db.AutoMigrate(&ShadowRun{}, &ShadowDivergence{}); err != nil {
		return fmt.Errorf("MigrateShadowDeployment: auto-migrate failed: %w", err)
	}

	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE shadow_runs
				ADD CONSTRAINT fk_shadow_runs_proposal
				FOREIGN KEY (proposal_id) REFERENCES scanner_config_proposals (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE shadow_divergences
				ADD CONSTRAINT fk_shadow_divergences_run
				FOREIGN KEY (shadow_run_id) REFERENCES shadow_runs (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE shadow_divergences
				ADD CONSTRAINT chk_shadow_divergences_kind
				CHECK (kind IN ('shadow_only', 'active_only'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("MigrateShadowDeployment: constraint setup failed: %w", err)
		}
	}

	return nil
}
