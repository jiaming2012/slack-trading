package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlaygroundSession struct {
	gorm.Model
	ID                uuid.UUID              `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	StartAt           time.Time              `gorm:"column:start_at;type:timestamptz;not null"`
	EndAt             *time.Time             `gorm:"column:end_at;type:timestamptz"`
	CurrentTime       time.Time              `gorm:"column:current_time;type:timestamptz;not null"`
	Balance           float64                `gorm:"column:balance;type:numeric;not null"`
	StartingBalance   float64                `gorm:"column:starting_balance;type:numeric;not null"`
	Env               string                 `gorm:"column:environment;type:text;not null"`
	Broker            *string                `gorm:"column:broker;type:text"`
	AccountID         *string                `gorm:"column:account_id;type:text"`
	ApiKeyName        *string                `gorm:"column:api_key;type:text"`
	Orders            []OrderRecord          `gorm:"foreignKey:PlaygroundID"`
	EquityPlotRecords []EquityPlotRecord     `gorm:"foreignKey:PlaygroundID;references:ID"`
	Repositories      CandleRepositoryRecord `gorm:"type:json;not null"`
}

func (s PlaygroundSession) ToPlayground() (IPlayground, error) {
	if s.Env == "simulator" {
		return &Playground{}, nil
	} else if s.Env == "live" {
		return &LivePlayground{}, nil
	} else {
		return nil, fmt.Errorf("unknown environment: %s", s.Env)
	}
}
