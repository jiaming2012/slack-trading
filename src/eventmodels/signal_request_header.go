package eventmodels

type SignalRequestHeader struct {
	Timeframe uint         `json:"timeframe"`
	Source    SignalSource `json:"source"`
	Symbol    StockSymbol  `json:"symbol"`
}
