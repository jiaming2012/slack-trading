package eventmodels

type ExecuteOpenTradeResult struct {
	BaseResponseEvent
	PriceLevelIndex int    `json:"priceLevelIndex"`
	Trade           *Trade `json:"trade"`
}
