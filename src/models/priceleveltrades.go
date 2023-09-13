package models

type PriceLevelTrades struct {
	PriceLevelIndex int    `json:"priceLevelIndex"`
	Trades          Trades `json:"trades"`
}
