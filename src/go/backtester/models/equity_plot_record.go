package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EquityPlotRecord struct {
	gorm.Model
	PlaygroundID uuid.UUID `gorm:"column:playground_session_id;type:uuid;not null;index:idx_playground_session_id"`
	Timestamp    time.Time `gorm:"column:timestamp;type:timestamptz;not null"`
	Equity       float64   `gorm:"column:equity;type:numeric;not null"`
}
