package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type TradeRecord struct {
	gorm.Model
	Order     OrderRecord `gorm:"foreignKey:OrderID;references:ID"`
	OrderID   uint        `gorm:"column:order_id;not null;index:idx_order_id"`
	Timestamp time.Time   `gorm:"column:timestamp;type:timestamptz;not null"`
	Quantity  float64     `gorm:"column:quantity;type:numeric;not null"`
	Price     float64     `gorm:"column:price;type:numeric;not null"`
}

func (t *TradeRecord) ToBacktesterTrade(symbol eventmodels.Instrument) (*BacktesterTrade, error) {
	return NewBacktesterTrade(symbol, t.Timestamp, t.Quantity, t.Price), nil
}

func NewTradeRecord(playgroundID uuid.UUID, orderID uint, timestamp time.Time, quantity float64, price float64) *TradeRecord {
	return &TradeRecord{
		OrderID:   orderID,
		Timestamp: timestamp,
		Quantity:  quantity,
		Price:     price,
	}
}
