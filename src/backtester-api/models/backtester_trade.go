package models

import "time"

type BacktesterTrade struct {
	Symbol          string
	TransactionDate time.Time
	Quantity        float64
	Price           float64
}

func NewBacktesterTrade(symbol string, transactionDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		Symbol:          symbol,
		TransactionDate: transactionDate,
		Quantity:        quantity,
		Price:           price,
	}
}
