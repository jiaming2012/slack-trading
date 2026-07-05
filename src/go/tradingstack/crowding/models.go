package crowding

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONArray persists a []string as a JSON array in a jsonb column, so
// CrowdingFlaggedCandidate.StrategyIDs round-trips without introducing a
// dependency on postgres-specific array types.
type JSONArray []string

// Value implements driver.Valuer, marshaling the slice to a JSON array. A nil
// slice is stored as an empty JSON array so the column is never NULL.
func (a JSONArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(a))
	if err != nil {
		return nil, fmt.Errorf("JSONArray.Value: marshal failed: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner, unmarshaling a jsonb column back into a
// []string.
func (a *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*a = JSONArray{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("JSONArray.Scan: unsupported type %T", value)
	}

	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("JSONArray.Scan: unmarshal failed: %w", err)
	}
	*a = JSONArray(out)
	return nil
}

// baseModel provides the shared UUID primary key and a BeforeCreate hook that
// assigns a fresh UUID when the id is unset, mirroring the pattern already
// established in tradingstack/ids.go. It is intentionally not shared with the
// tradingstack package's BaseModel so this package stays self-contained: it
// only reads tradingstack's ScanResult / SimOutcome types, and owns its own
// persistence primitives.
type baseModel struct {
	ID uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
}

// BeforeCreate populates the primary key with a new UUID when it is the zero
// value, so callers may leave ID unset and still get a stable, generated id.
func (b *baseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// CrowdingMetric is one row per scan-cycle crowding summary.
type CrowdingMetric struct {
	baseModel
	ScannedAt             time.Time `gorm:"column:scanned_at;type:timestamptz;not null"`
	ComputedAt            time.Time `gorm:"column:computed_at;type:timestamptz;not null"`
	TotalCandidates       int       `gorm:"column:total_candidates;type:integer;not null"`
	OverlappingCandidates int       `gorm:"column:overlapping_candidates;type:integer;not null"`
	OverlapPct            float64   `gorm:"column:overlap_pct;type:numeric;not null"`
	ThresholdPct          float64   `gorm:"column:threshold_pct;type:numeric;not null"`
	Flagged               bool      `gorm:"column:flagged;type:boolean;not null"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (CrowdingMetric) TableName() string {
	return "crowding_metrics"
}

// CrowdingFlaggedCandidate is one row per crowded candidate within a flagged
// scan cycle, naming the overlapping strategies.
type CrowdingFlaggedCandidate struct {
	baseModel
	CrowdingMetricID uuid.UUID `gorm:"column:crowding_metric_id;type:uuid;not null"`
	ScanResultID     uuid.UUID `gorm:"column:scan_result_id;type:uuid;not null"`
	Ticker           string    `gorm:"column:ticker;type:text;not null"`
	StrategyIDs      JSONArray `gorm:"column:strategy_ids;type:jsonb;not null"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (CrowdingFlaggedCandidate) TableName() string {
	return "crowding_flagged_candidates"
}
