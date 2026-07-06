package stratopt

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Proposal status vocabulary. A new proposal's status is set solely by its
// gate verdict (passed -> pending_review, failed -> rejected_by_gate); the
// two terminal states (accepted, dismissed) are reachable only through the
// operator decide command, and only from pending_review.
//
// The shared status core (rejected_by_gate, pending_review) matches
// scanner-optimizer; the terminal states deliberately differ ("accepted", not
// "promoted") because this optimizer has no apply target -- accepting a
// proposal records the operator's judgment and nothing else.
const (
	StatusRejectedByGate = "rejected_by_gate"
	StatusPendingReview  = "pending_review"
	StatusAccepted       = "accepted"
	StatusDismissed      = "dismissed"
)

// validProposalStatus reports whether s is one of the four allowed statuses.
func validProposalStatus(s string) bool {
	switch s {
	case StatusRejectedByGate, StatusPendingReview, StatusAccepted, StatusDismissed:
		return true
	}
	return false
}

// Decision vocabulary for DecideProposal.
const (
	DecisionAccepted  = "accepted"
	DecisionDismissed = "dismissed"
)

// validDecision reports whether d is an allowed operator decision.
func validDecision(d string) bool {
	return d == DecisionAccepted || d == DecisionDismissed
}

// StrategyProposal is one persisted strategy-parameter recommendation.
// verdict_id references overfitting_verdicts.id (not null -- every persisted
// proposal carries its gate verdict, pass or fail; the proposal holds the FK
// toward the verdict, per the overfitting package's design).
//
// NOTHING reads this table to modify any strategy configuration or trading
// behavior: a proposal is a recommendation requiring operator action, and
// recording a decision changes only this row.
type StrategyProposal struct {
	ID            uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	RunID         uuid.UUID  `gorm:"column:run_id;type:uuid;not null"`
	StrategyID    string     `gorm:"column:strategy_id;type:text;not null"`
	Regime        string     `gorm:"column:regime;type:text;not null"`
	Parameter     string     `gorm:"column:parameter;type:text;not null"`
	AdjustmentPct float64    `gorm:"column:adjustment_pct;type:numeric;not null"`
	Rule          string     `gorm:"column:rule;type:text;not null"`
	Rationale     string     `gorm:"column:rationale;type:text;not null"`
	EvidenceJSON  []byte     `gorm:"column:evidence_json;type:jsonb;not null"`
	VerdictID     uuid.UUID  `gorm:"column:verdict_id;type:uuid;not null"`
	Status        string     `gorm:"column:status;type:text;not null"`
	DecidedAt     *time.Time `gorm:"column:decided_at;type:timestamptz"`
	DecidedVia    *string    `gorm:"column:decided_via;type:text"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (StrategyProposal) TableName() string {
	return "strategy_proposals"
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, mirroring the tradingstack BaseModel pattern.
func (p *StrategyProposal) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// BeforeSave enforces the status vocabulary in Go, in front of the database
// CHECK constraint, so an invalid row is rejected before it reaches the wire.
func (p *StrategyProposal) BeforeSave(tx *gorm.DB) error {
	if !validProposalStatus(p.Status) {
		return fmt.Errorf("%w: %q", ErrInvalidProposalStatus, p.Status)
	}
	return nil
}

// MigrateStrategyOptimizer creates only the strategy_proposals table, its
// status CHECK constraint, and its verdict_id foreign key. It requires
// overfitting.MigrateOverfittingCountermeasures to have run first (the FK
// targets overfitting_verdicts.id) and returns a clear error naming that
// dependency when the target table is missing. It deliberately does NOT
// create, alter, or drop any trading-stack-schema table, the
// overfitting_verdicts table, or any playground table, and is not wired into
// dbutils.InitPostgres -- callers invoke it explicitly, matching the
// tradingstack migration lifecycle convention (precedent:
// costmodel.MigrateNetEvCostModel, scanneropt.MigrateScannerOptimizer).
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// constraints are added inside DO blocks that swallow duplicate_object
// errors, so a second run leaves the schema unchanged.
func MigrateStrategyOptimizer(db *gorm.DB) error {
	if !db.Migrator().HasTable("overfitting_verdicts") {
		return fmt.Errorf("MigrateStrategyOptimizer: %w", ErrMissingVerdictTable)
	}

	if err := db.AutoMigrate(&StrategyProposal{}); err != nil {
		return fmt.Errorf("MigrateStrategyOptimizer: auto-migrate failed: %w", err)
	}

	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE strategy_proposals
				ADD CONSTRAINT chk_strategy_proposals_status
				CHECK (status IN ('rejected_by_gate', 'pending_review', 'accepted', 'dismissed'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE strategy_proposals
				ADD CONSTRAINT fk_strategy_proposals_verdict
				FOREIGN KEY (verdict_id) REFERENCES overfitting_verdicts (id);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("MigrateStrategyOptimizer: constraint setup failed: %w", err)
		}
	}

	return nil
}
