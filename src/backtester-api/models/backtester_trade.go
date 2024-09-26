package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterTrade struct {
	Symbol          eventmodels.Instrument
	TransactionDate time.Time
	Quantity        float64
	Price           float64
}

func NewBacktesterTrade(symbol eventmodels.Instrument, transactionDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		Symbol:          symbol,
		TransactionDate: transactionDate,
		Quantity:        quantity,
		Price:           price,
	}
}
