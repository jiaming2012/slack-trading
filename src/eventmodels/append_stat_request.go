package eventmodels

import "time"

type EquityPlot struct {
	Timestamp time.Time
	Value     float64
}

type AppendStatRequest struct {
	EquityPlot *EquityPlot
}
