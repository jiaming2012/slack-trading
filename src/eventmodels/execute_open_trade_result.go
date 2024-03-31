package eventmodels

type ExecuteOpenTradeResult struct {
	BaseResponseEvent2
	PriceLevelIndex int    `json:"priceLevelIndex"`
	Trade           *Trade `json:"trade"`
}
