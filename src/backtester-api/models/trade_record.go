package models

import (
	"time"

	"gorm.io/gorm"
)

type TradeRecord struct {
	gorm.Model
	OrderID   uint      `gorm:"column:order_id;not null;index:idx_order_id"`
	Timestamp time.Time `gorm:"column:timestamp;type:timestamptz;not null"`
	Quantity  float64   `gorm:"column:quantity;type:numeric;not null"`
	Price     float64   `gorm:"column:price;type:numeric;not null"`
}

func NewTradeRecord(order *OrderRecord, timestamp time.Time, quantity float64, price float64) *TradeRecord {
	return &TradeRecord{
		OrderID:   order.ID,
		Timestamp: timestamp,
		Quantity:  quantity,
		Price:     price,
	}
}
