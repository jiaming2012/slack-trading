package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type TradeRecord struct {
	gorm.Model
	OrderRecord *OrderRecord     `gorm:"foreignKey:OrderID;references:ID"`
	Order       *BacktesterOrder `gorm:"-"`
	OrderID     uint             `gorm:"column:order_id;not null;index:idx_order_id"`
	Timestamp   time.Time        `gorm:"column:timestamp;type:timestamptz;not null"`
	Quantity    float64          `gorm:"column:quantity;type:numeric;not null"`
	Price       float64          `gorm:"column:price;type:numeric;not null"`
}

func (t *TradeRecord) GetSymbol() eventmodels.Instrument {
	if t.Order != nil {
		return t.Order.Symbol
	} else if t.OrderRecord != nil {
		switch t.OrderRecord.Class {
		case "equity":
			return eventmodels.NewStockSymbol(t.OrderRecord.Symbol)
		default:
			panic("invalid order class")
		}
	} else {
		panic("invalid trade record")
	}
}

func NewTradeRecord(order *BacktesterOrder, timestamp time.Time, quantity float64, price float64) *TradeRecord {
	return &TradeRecord{
		Order:     order,
		OrderID:   order.ID,
		Timestamp: timestamp,
		Quantity:  quantity,
		Price:     price,
	}
}
