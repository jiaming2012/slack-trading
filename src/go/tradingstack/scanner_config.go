package tradingstack

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScannerConfig mirrors the scanner_configs table. config_json is stored as
// JSONB (raw bytes), enabling verbatim versioning and rollback by id.
type ScannerConfig struct {
	BaseModel
	CreatedAt      *time.Time `gorm:"column:created_at;type:timestamptz"`
	Regime         *string    `gorm:"column:regime;type:text"`
	ConfigJSON     []byte     `gorm:"column:config_json;type:jsonb"`
	OptimizerRunID *string    `gorm:"column:optimizer_run_id;type:text"`
}

// TableName pins the table name so GORM pluralization cannot alter it.
func (ScannerConfig) TableName() string {
	return "scanner_configs"
}

// FetchScannerConfigByID fetches a prior scanner config verbatim by its id,
// supporting rollback to an earlier configuration version.
func FetchScannerConfigByID(db *gorm.DB, id uuid.UUID) (*ScannerConfig, error) {
	var cfg ScannerConfig
	if err := db.First(&cfg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}
