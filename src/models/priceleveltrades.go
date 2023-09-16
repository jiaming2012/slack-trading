package models

type TradeLevels struct {
	PriceLevelIndex int    `json:"priceLevelIndex"`
	Trades          Trades `json:"trades"`
}
