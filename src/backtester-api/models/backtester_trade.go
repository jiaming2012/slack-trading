package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterTrade struct {
	OrderId    uint                   `json:"order_id,omitempty"`
	Symbol     eventmodels.Instrument `json:"symbol"`
	CreateDate time.Time              `json:"create_date"`
	Quantity   float64                `json:"quantity"`
	Price      float64                `json:"price"`
}

func (t *BacktesterTrade) ToTradeRecord() *TradeRecord {
	return NewTradeRecord(t.OrderId, t.CreateDate, t.Quantity, t.Price)
}

func NewBacktesterTrade(orderId uint, symbol eventmodels.Instrument, createDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		OrderId:    orderId,
		Symbol:     symbol,
		CreateDate: createDate,
		Quantity:   quantity,
		Price:      price,
	}
}
