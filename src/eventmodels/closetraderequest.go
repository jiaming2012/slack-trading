package eventmodels

type CloseTradeRequest struct {
	AccountName     string
	StrategyName    string
	PriceLevelIndex int
	Percent         float64
	Reason          string
}
