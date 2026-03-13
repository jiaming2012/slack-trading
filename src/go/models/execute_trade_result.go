package models

type ExecuteOpenTradeResult struct {
	PriceLevelIndex int    `json:"priceLevelIndex"`
	Trade           *Trade `json:"trade"`
}

type ExecuteCloseTradesResult struct {
	Trade *Trade `json:"trade"`
}
