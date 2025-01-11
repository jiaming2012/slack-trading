package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderRecord struct {
	gorm.Model
	PlaygroundID uuid.UUID         `gorm:"type:uuid;not null"`
	Playground   PlaygroundSession `gorm:"foreignKey:PlaygroundID"`
	OrderID      uint              `gorm:"default:nextval('order_id_seq');not null"`
	Class        string            `gorm:"column:class;type:text;not null"`
	Symbol       string            `gorm:"column:symbol;type:text;not null"`
	Side         string            `gorm:"column:side;type:text;not null"`
	Quantity     float64           `gorm:"column:quantity;type:numeric;not null"`
	OrderType    string            `gorm:"column:order_type;type:text;not null"`
	Duration     string            `gorm:"column:duration;type:text;not null"`
	Price        *float64          `gorm:"column:price;type:numeric"`
	StopPrice    *float64          `gorm:"column:stop_price;type:numeric"`
	Status       string            `gorm:"column:status;type:text;not null"`
	Tag          string            `gorm:"column:tag;type:text"`
	CreatedOn    time.Time         `gorm:"column:start_at;type:timestamp;not null"`
}
