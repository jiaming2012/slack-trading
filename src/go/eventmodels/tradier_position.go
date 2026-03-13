package eventmodels

import "time"

type TradierPosition struct {
	CostBasis    float64
	DateAcquired time.Time
	ID           int
	Quantity     float64
	Symbol       OptionSymbol
}
