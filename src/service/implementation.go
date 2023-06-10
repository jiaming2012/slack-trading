package service

type TradeWorker struct {
}

type NewPriceLevelRequest struct {
}

type PriceLevel struct {
	Timeframe  int
	Support    []float64
	Resistance []float64
}

func NewPriceLevel(timeframe int, support []float64, resistance []float64) *PriceLevel {
	return &PriceLevel{
		Timeframe:  timeframe,
		Support:    support,
		Resistance: resistance,
	}
}
