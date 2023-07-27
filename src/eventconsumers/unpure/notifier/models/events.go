package models

import "time"

type RealizedProfitLossEvent struct {
	Profit float64
}

type TradeFulfilledEvent struct {
	Timestamp time.Time
	Symbol    string
	Volume    float64
	Price     float64
}
