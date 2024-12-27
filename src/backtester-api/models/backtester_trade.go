package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterTrade struct {
	ID         uint                   `json:"id"`
	Symbol     eventmodels.Instrument `json:"symbol"`
	CreateDate time.Time              `json:"create_date"`
	Quantity   float64                `json:"quantity"`
	Price      float64                `json:"price"`
}

func NewBacktesterTrade(id uint, symbol eventmodels.Instrument, createDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		ID:         id,
		Symbol:     symbol,
		CreateDate: createDate,
		Quantity:   quantity,
		Price:      price,
	}
}
