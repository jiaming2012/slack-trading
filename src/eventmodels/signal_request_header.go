package eventmodels

type SignalRequestHeader struct {
	Timeframe uint                `json:"timeframe"`
	Source    SignalRequestSource `json:"source"`
	Symbol    string              `json:"symbol"`
}
