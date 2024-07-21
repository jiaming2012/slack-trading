package eventmodels

type SignalRequestHeader struct {
	Timeframe uint         `json:"timeframe"`
	Source    SignalSource `json:"source"`
	Symbol    StockSymbol  `json:"symbol"`
}

func NewSignalRequestHeader (timeframe uint, source SignalSource, symbol StockSymbol) *SignalRequestHeader {
	return &SignalRequestHeader{
		Timeframe: timeframe,
		Source:    source,
		Symbol:    symbol,
	}
}