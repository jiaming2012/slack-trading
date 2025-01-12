package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlaygroundSession struct {
	gorm.Model
	ID              uuid.UUID     `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	StartAt         time.Time     `gorm:"column:start_at;type:timestamp;not null"`
	EndAt           *time.Time    `gorm:"column:end_at;type:timestamp"`
	StartingBalance float64       `gorm:"column:starting_balance;type:numeric;not null"`
	Env             string        `gorm:"column:environment;type:text;not null"`
	Orders          []OrderRecord `gorm:"foreignKey:PlaygroundID"`
}
