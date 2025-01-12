package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TradeRecord struct {
	gorm.Model
	Order        OrderRecord       `gorm:"foreignKey:OrderID;references:ID"`
	OrderID      uint              `gorm:"column:order_id;not null;index:idx_order_id"`
	TradeID      uint              `gorm:"column:id;not null;index:idx_trade_id"`
	Timestamp    time.Time         `gorm:"column:timestamp;type:timestamp;not null"`
	Quantity     float64           `gorm:"column:quantity;type:numeric;not null"`
	Price        float64           `gorm:"column:price;type:numeric;not null"`
}

func NewTradeRecord(playgroundID uuid.UUID, orderID uint, tradeID uint, timestamp time.Time, quantity float64, price float64) *TradeRecord {
	return &TradeRecord{
		OrderID:      orderID,
		TradeID:      tradeID,
		Timestamp:    timestamp,
		Quantity:     quantity,
		Price:        price,
	}
}