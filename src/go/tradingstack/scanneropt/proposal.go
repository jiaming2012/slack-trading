package scanneropt

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Proposal status vocabulary. Status is set from the gate verdict at insert
// (passed -> pending_review, failed -> rejected_by_gate) and flips to
// promoted only via the operator promote command.
const (
	StatusPendingReview  = "pending_review"
	StatusRejectedByGate = "rejected_by_gate"
	StatusPromoted       = "promoted"
)

// validProposalStatus reports whether s is one of the three allowed statuses.
func validProposalStatus(s string) bool {
	return s == StatusPendingReview || s == StatusRejectedByGate || s == StatusPromoted
}

// ScannerConfigProposal is one persisted optimizer proposal awaiting (or
// having finished) operator review. verdict_id references
// overfitting_verdicts.id -- the proposal holds the FK toward the verdict, per
// the overfitting package's design.
type ScannerConfigProposal struct {
	ID                 uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	CreatedAt          time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	OptimizerRunID     string    `gorm:"column:optimizer_run_id;type:text;not null"`
	ProposedConfigJSON []byte    `gorm:"column:proposed_config_json;type:jsonb;not null"`
	EvidenceJSON       []byte    `gorm:"column:evidence_json;type:jsonb;not null"`
	VerdictID          uuid.UUID `gorm:"column:verdict_id;type:uuid;not null"`
	Status             string    `gorm:"column:status;type:text;not null"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (ScannerConfigProposal) TableName() string {
	return "scanner_config_proposals"
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, mirroring the tradingstack BaseModel pattern.
func (p *ScannerConfigProposal) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// BeforeSave enforces the status vocabulary in Go, in front of the database
// CHECK constraint, so an invalid row is rejected before it reaches the wire.
func (p *ScannerConfigProposal) BeforeSave(tx *gorm.DB) error {
	if !validProposalStatus(p.Status) {
		return fmt.Errorf("%w: %q", ErrInvalidProposalStatus, p.Status)
	}
	return nil
}

// MigrateScannerOptimizer creates only the scanner_config_proposals table,
// its status CHECK constraint, and its verdict_id foreign key. It requires
// MigrateOverfittingCountermeasures to have run first (the FK targets
// overfitting_verdicts.id) and returns a clear error naming that dependency
// when the target table is missing. It deliberately does NOT create, alter,
// or drop any trading-stack-schema table, the overfitting_verdicts table, or
// any playground table, and is not wired into dbutils.InitPostgres -- callers
// invoke it explicitly, matching the tradingstack migration lifecycle
// convention.
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// constraints are added inside DO blocks that swallow duplicate_object
// errors, so a second run leaves the schema unchanged.
func MigrateScannerOptimizer(db *gorm.DB) error {
	if !db.Migrator().HasTable("overfitting_verdicts") {
		return fmt.Errorf("MigrateScannerOptimizer: %w", ErrMissingVerdictTable)
	}

	if err := db.AutoMigrate(&ScannerConfigProposal{}); err != nil {
		return fmt.Errorf("MigrateScannerOptimizer: auto-migrate failed: %w", err)
	}

	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE scanner_config_proposals
				ADD CONSTRAINT chk_scanner_config_proposals_status
				CHECK (status IN ('pending_review', 'rejected_by_gate', 'promoted'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE scanner_config_proposals
				ADD CONSTRAINT fk_scanner_config_proposals_verdict
				FOREIGN KEY (verdict_id) REFERENCES overfitting_verdicts (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("MigrateScannerOptimizer: constraint setup failed: %w", err)
		}
	}

	return nil
}
