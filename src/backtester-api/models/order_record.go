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
	Timestamp       time.Time         `gorm:"column:timestamp;type:timestamp;not null"`
	Trades          []TradeRecord     `gorm:"foreignKey:OrderID"`
}

func (o *OrderRecord) ToBacktesterOrder() (*BacktesterOrder, error) {
	class := BacktesterOrderClass(o.Class)
	var symbol eventmodels.Instrument

	switch class {
	case BacktesterOrderClassEquity:
		symbol = eventmodels.NewStockSymbol(o.Symbol)
	default:
		return nil, fmt.Errorf("invalid order class: %s", class)
	}

	var trades []*BacktesterTrade
	for _, t := range o.Trades {
		tr, err := t.ToBacktesterTrade(symbol)
		if err != nil {
			return nil, fmt.Errorf("OrderRecord.ToBacktesterOrder(): failed to convert trade: %w", err)
		}

		trades = append(trades, tr)
	}

	return &BacktesterOrder{
		ID:               o.ExternalOrderID,
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
		Trades:           trades,
		RejectReason:     o.RejectReason,
		CreateDate:       o.Timestamp,
	}, nil
}
