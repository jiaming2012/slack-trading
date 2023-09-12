package models

type ExecuteOpenTradeResult struct {
	PriceLevelIndex int     `json:"priceLevelIndex"`
	ExecutedPrice   float64 `json:"executedPrice"`
	ExecutedVolume  float64 `json:"executedVolume"`
}
