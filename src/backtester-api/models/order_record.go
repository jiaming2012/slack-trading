package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderRecord struct {
	gorm.Model
	PlaygroundID    uuid.UUID         `gorm:"column:playground_id;type:uuid;not null;index:idx_playground_order"`
	Playground      PlaygroundSession `gorm:"foreignKey:PlaygroundID;references:ID"`
	ExternalOrderID uint              `gorm:"column:external_id;not null;index:idx_external_order_id"`
	Class           string            `gorm:"column:class;type:text;not null"`
	Symbol          string            `gorm:"column:symbol;type:text;not null"`
	Side            string            `gorm:"column:side;type:text;not null"`
	Quantity        float64           `gorm:"column:quantity;type:numeric;not null"`
	OrderType       string            `gorm:"column:order_type;type:text;not null"`
	Duration        string            `gorm:"column:duration;type:text;not null"`
	Price           *float64          `gorm:"column:price;type:numeric"`
	StopPrice       *float64          `gorm:"column:stop_price;type:numeric"`
	Status          string            `gorm:"column:status;type:text;not null"`
	Tag             string            `gorm:"column:tag;type:text"`
	Timestamp       time.Time         `gorm:"column:timestamp;type:timestamp;not null"`
	Trades          []TradeRecord     `gorm:"foreignKey:OrderID"`
}
