package models

import (
	"time"

	"gorm.io/gorm"
)

type LiveAccountPlot struct {
	gorm.Model
	LiveAccountID uint      `gorm:"column:live_account_id;index:idx_live_account_id"`
	Timestamp     time.Time `gorm:"column:timestamp;type:timestamptz;not null"`
	Equity        *float64  `gorm:"column:equity;type:numeric"`
}
