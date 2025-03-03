package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type OrderRecord struct {
	gorm.Model
	PlaygroundID    uuid.UUID         `gorm:"column:playground_id;type:uuid;not null;index:idx_playground_order"`
	Playground      PlaygroundSession `gorm:"foreignKey:PlaygroundID;references:ID"`
	AccountType     string            `gorm:"column:account_type;type:text"`
	ExternalOrderID uint              `gorm:"column:external_id;not null;index:idx_external_order_id"`
	Class           string            `gorm:"column:class;type:text;not null"`
	Symbol          string            `gorm:"column:symbol;type:text;not null"`
	Side            string            `gorm:"column:side;type:text;not null"`
	Quantity        float64           `gorm:"column:quantity;type:numeric;not null"`
	OrderType       string            `gorm:"column:order_type;type:text;not null"`
	Duration        string            `gorm:"column:duration;type:text;not null"`
	Price           *float64          `gorm:"column:price;type:numeric"`
	RequestedPrice  float64           `gorm:"column:requested_price;type:numeric"`
	StopPrice       *float64          `gorm:"column:stop_price;type:numeric"`
	Status          string            `gorm:"column:status;type:text;not null"`
	RejectReason    *string           `gorm:"column:reject_reason;type:text"`
	Tag             string            `gorm:"column:tag;type:text"`
	Timestamp       time.Time         `gorm:"column:timestamp;type:timestamptz;not null"`
	Closes          []*OrderRecord    `gorm:"many2many:order_closes"`
	ClosedBy        []*TradeRecord    `gorm:"many2many:trade_closed_by"`
	Reconciles      []*OrderRecord    `gorm:"many2many:order_reconciles"`
	Trades          []*TradeRecord    `gorm:"foreignKey:OrderID"`
}

func (o *OrderRecord) ToBacktesterOrder() (*BacktesterOrder, error) {
	var closes []*BacktesterOrder
	for _, c := range o.Closes {
		co, err := c.ToBacktesterOrder()
		if err != nil {
			return nil, fmt.Errorf("OrderRecord.ToBacktesterOrder(): failed to convert close order: %w", err)
		}

		closes = append(closes, co)
	}

	var reconciles []*BacktesterOrder
	for _, r := range o.Reconciles {
		co, err := r.ToBacktesterOrder()
		if err != nil {
			return nil, fmt.Errorf("OrderRecord.ToBacktesterOrder(): failed to convert reconciled order: %w", err)
		}

		reconciles = append(reconciles, co)
	}

	for _, t := range o.Trades {
		t.OrderRecord = o
	}

	for _, c := range o.ClosedBy {
		c.OrderRecord = o
	}

	return &BacktesterOrder{
		ID:               o.ExternalOrderID,
		PlaygroundID:     o.PlaygroundID,
		Class:            BacktesterOrderClass(o.Class),
		Symbol:           eventmodels.NewStockSymbol(o.Symbol),
		Side:             TradierOrderSide(o.Side),
		AbsoluteQuantity: o.Quantity,
		Type:             BacktesterOrderType(o.OrderType),
		Duration:         BacktesterOrderDuration(o.Duration),
		Price:            o.Price,
		RequestedPrice:   o.RequestedPrice,
		StopPrice:        o.StopPrice,
		Tag:              o.Tag,
		Status:           BacktesterOrderStatus(o.Status),
		Trades:           o.Trades,
		RejectReason:     o.RejectReason,
		CreateDate:       o.Timestamp,
		Closes:           closes,
		ClosedBy:         o.ClosedBy,
		Reconciles:       reconciles,
	}, nil
}
