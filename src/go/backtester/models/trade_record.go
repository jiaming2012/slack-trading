package models

import (
	"time"

	"gorm.io/gorm"
)

type TradeRecord struct {
	gorm.Model
	ParentTradeID    *uint        `gorm:"column:parent_trade_id;index;"`
	ParentTrade      *TradeRecord `gorm:"foreignKey:ParentTradeID;references:ID;constraint:OnDelete:SET NULL,OnUpdate:CASCADE;"`
	OrderID          *uint        `gorm:"column:order_id;index:idx_order_id;constraint:OnDelete:SET NULL,OnUpdate:CASCADE;"`
	ReconcileOrderID *uint        `gorm:"column:reconcile_order_id;index:idx_reconcile_order_id;constraint:OnDelete:SET NULL,OnUpdate:CASCADE;"`
	Timestamp        time.Time    `gorm:"column:timestamp;type:timestamptz;not null"`
	Quantity         float64      `gorm:"column:quantity;type:numeric;not null"`
	Price            float64      `gorm:"column:price;type:numeric;not null"`
}

func (tr *TradeRecord) UpdateOrder(order *OrderRecord) {
	if order.LiveAccountType == LiveAccountTypeSimulator {
		return // TODO: put an identifier in the trade record to indicate that this is a simulated trade
	}

	if order.LiveAccountType == LiveAccountTypeReconcilation {
		tr.ReconcileOrderID = &order.ID
	} else {
		tr.OrderID = &order.ID
	}
}

func NewTradeRecord(order *OrderRecord, timestamp time.Time, quantity float64, price float64) *TradeRecord {
	tr := &TradeRecord{
		Timestamp: timestamp,
		Quantity:  quantity,
		Price:     price,
	}

	return tr
}
