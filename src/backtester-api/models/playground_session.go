package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type PlaygroundSession struct {
	gorm.Model
	ID                uuid.UUID              `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ClientID          *string                `gorm:"column:client_id;type:text;unique"`
	StartAt           time.Time              `gorm:"column:start_at;type:timestamptz;not null"`
	EndAt             *time.Time             `gorm:"column:end_at;type:timestamptz"`
	CurrentTime       time.Time              `gorm:"column:current_time;type:timestamptz;not null"`
	Balance           float64                `gorm:"column:balance;type:numeric;not null"`
	StartingBalance   float64                `gorm:"column:starting_balance;type:numeric;not null"`
	Env               string                 `gorm:"column:environment;type:text;not null"`
	LiveAccount       *LiveAccount           `gorm:"foreignKey:LiveAccountID"`
	LiveAccountID     *uint                  `gorm:"column:live_account_id;index:idx_live_account_id"`
	Broker            *string                `gorm:"column:broker;type:text"`
	AccountID         *string                `gorm:"column:account_id;type:text"`
	LiveAccountType   *string                `gorm:"column:live_account_type;type:text"`
	Orders            []OrderRecord          `gorm:"foreignKey:PlaygroundID"`
	EquityPlotRecords []EquityPlotRecord     `gorm:"foreignKey:PlaygroundID;references:ID"`
	Tags              pq.StringArray         `gorm:"type:text[]"`
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
