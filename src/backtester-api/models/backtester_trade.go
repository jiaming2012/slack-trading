package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterTrade struct {
	Symbol     eventmodels.Instrument `json:"symbol"`
	CloseId    *uint                  `json:"close_id,omitempty"`
	CreateDate time.Time              `json:"create_date"`
	Quantity   float64                `json:"quantity"`
	Price      float64                `json:"price"`
}

func (t *BacktesterTrade) ToTradeRecord(playgroundID uuid.UUID, orderID uint) *TradeRecord {
	return NewTradeRecord(playgroundID, orderID, t.CreateDate, t.Quantity, t.Price)
}

func NewBacktesterTrade(symbol eventmodels.Instrument, createDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		Symbol:     symbol,
		CreateDate: createDate,
		Quantity:   quantity,
		Price:      price,
	}
}
