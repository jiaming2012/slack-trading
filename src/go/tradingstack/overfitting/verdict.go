package overfitting

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LibraryVersion is stamped onto every persisted verdict so the audit trail
// records which revision of the gate produced it.
const LibraryVersion = "1.0.0"

// CheckResult is the outcome of one gate check: its name, pass/fail, the
// observed value(s), the threshold(s) they were held to, and a human-readable
// detail naming any failing or skipped condition.
type CheckResult struct {
	Name      string             `json:"name"`
	Passed    bool               `json:"passed"`
	Observed  map[string]float64 `json:"observed"`
	Threshold map[string]float64 `json:"threshold"`
	Detail    string             `json:"detail"`
}

// Verdict is the gate's judgment of one proposal: the proposal identity,
// whether every check passed, and all four per-check results in fixed order.
type Verdict struct {
	ProposalID   uuid.UUID     `json:"proposal_id"`
	ProposalKind ProposalKind  `json:"proposal_kind"`
	Passed       bool          `json:"passed"`
	Checks       []CheckResult `json:"checks"`
}

// CheckResults persists a []CheckResult as a JSON array in a jsonb column so
// per-check numbers round-trip losslessly.
type CheckResults []CheckResult

// Value implements driver.Valuer, marshaling the results to a JSON array. A
// nil slice is stored as an empty JSON array so the column is never NULL.
func (c CheckResults) Value() (driver.Value, error) {
	if c == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]CheckResult(c))
	if err != nil {
		return nil, fmt.Errorf("CheckResults.Value: marshal failed: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner, unmarshaling a jsonb column back into a
// []CheckResult.
func (c *CheckResults) Scan(value interface{}) error {
	if value == nil {
		*c = CheckResults{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("CheckResults.Scan: unsupported type %T", value)
	}

	var out []CheckResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("CheckResults.Scan: unmarshal failed: %w", err)
	}
	*c = CheckResults(out)
	return nil
}

// OverfittingVerdict is the persisted audit record of one gate run -- pass or
// fail. proposal_id is deliberately NOT a foreign key: the proposal tables
// (scanner_config_proposals and the strategy-optimizer's equivalent) live in
// later changes and are two different tables, so the consuming optimizers hold
// the FK in the other direction (their proposal rows reference
// overfitting_verdicts.id).
type OverfittingVerdict struct {
	ID             uuid.UUID    `gorm:"column:id;type:uuid;primaryKey"`
	ProposalID     uuid.UUID    `gorm:"column:proposal_id;type:uuid;not null"`
	ProposalKind   string       `gorm:"column:proposal_kind;type:text;not null"`
	ComputedAt     time.Time    `gorm:"column:computed_at;type:timestamptz;not null"`
	Passed         bool         `gorm:"column:passed;type:boolean;not null"`
	ChecksJSON     CheckResults `gorm:"column:checks_json;type:jsonb;not null"`
	LibraryVersion string       `gorm:"column:library_version;type:text;not null"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (OverfittingVerdict) TableName() string {
	return "overfitting_verdicts"
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, mirroring the tradingstack BaseModel pattern.
func (v *OverfittingVerdict) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

// BeforeSave enforces the proposal-kind vocabulary in Go, in front of the
// database CHECK constraint, so an invalid row is rejected before it reaches
// the wire.
func (v *OverfittingVerdict) BeforeSave(tx *gorm.DB) error {
	if !ProposalKind(v.ProposalKind).Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownProposalKind, v.ProposalKind)
	}
	return nil
}

// NewOverfittingVerdict builds the persistable audit record for a gate
// verdict, assigning a fresh id (so callers hold the verdict id for their own
// FK before persisting), stamping computedAt (supplied by the caller -- the
// gate itself does no clock I/O), and recording the library version.
func NewOverfittingVerdict(v Verdict, computedAt time.Time) OverfittingVerdict {
	return OverfittingVerdict{
		ID:             uuid.New(),
		ProposalID:     v.ProposalID,
		ProposalKind:   string(v.ProposalKind),
		ComputedAt:     computedAt,
		Passed:         v.Passed,
		ChecksJSON:     CheckResults(v.Checks),
		LibraryVersion: LibraryVersion,
	}
}

// MigrateOverfittingCountermeasures creates only the overfitting_verdicts
// table and its proposal-kind CHECK constraint. It deliberately does NOT touch
// any trading-stack-schema table or any existing playground table, and is not
// wired into dbutils.InitPostgres -- callers invoke it explicitly, matching
// the tradingstack migration lifecycle convention.
//
// The call is idempotent: AutoMigrate is a no-op on existing columns, and the
// CHECK constraint is added inside a DO block that swallows duplicate_object
// errors, so a second run leaves the schema unchanged.
func MigrateOverfittingCountermeasures(db *gorm.DB) error {
	if err := db.AutoMigrate(&OverfittingVerdict{}); err != nil {
		return fmt.Errorf("MigrateOverfittingCountermeasures: auto-migrate failed: %w", err)
	}

	constraint := `DO $$ BEGIN
		ALTER TABLE overfitting_verdicts
			ADD CONSTRAINT chk_overfitting_verdicts_proposal_kind
			CHECK (proposal_kind IN ('scanner', 'strategy'));
	EXCEPTION WHEN duplicate_object THEN NULL; END $$;`

	if err := db.Exec(constraint).Error; err != nil {
		return fmt.Errorf("MigrateOverfittingCountermeasures: constraint setup failed: %w", err)
	}

	return nil
}
