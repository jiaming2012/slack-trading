package eventmodels

type TradeLevels struct {
	PriceLevelIndex int         `json:"priceLevelIndex"`
	Trades          []*TradeDTO `json:"trades"`
}
