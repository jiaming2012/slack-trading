package eventmodels

import "time"

type OptionSpreadLeg struct {
	ID           uint
	Timestamp    time.Time
	Side         string
	Symbol       OptionSymbol
	Quantity     float64
	AvgFillPrice float64
}
