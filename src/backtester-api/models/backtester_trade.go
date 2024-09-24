package models

import "time"

type BacktesterTrade struct {
	TransactionDate time.Time
	Quantity        float64
	Price           float64
}

func NewBacktesterTrade(transactionDate time.Time, quantity float64, price float64) *BacktesterTrade {
	return &BacktesterTrade{
		TransactionDate: transactionDate,
		Quantity:        quantity,
		Price:           price,
	}
}
