package eventmodels

import "time"

type OptionSpreadAnalysisRequest struct {
	ID            uint
	Underlying    StockSymbol
	ExecutionType string
	Leg1          OptionSpreadLeg
	Leg2          OptionSpreadLeg
	CreateDate    time.Time
	Tag           string
	AvgFillPrice  float64
	Config        *OptionYAML
}
