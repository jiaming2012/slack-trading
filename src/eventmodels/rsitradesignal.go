package eventmodels

type RsiTradeSignal struct {
	Value          float64
	IsBuy          bool
	RequestedPrice float64
}
