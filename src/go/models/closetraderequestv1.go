package models

type CloseTradeRequestV1 struct {
	Trade     *Trade
	Strategy  *Strategy
	Timeframe int
	Volume    float64
	Reason    string
}
