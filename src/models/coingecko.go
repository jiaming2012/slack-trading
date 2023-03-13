package models

type MarketItem struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

type TickerItem struct {
	Base      string `json:"base"`
	Target    string `json:"target"`
	Market    MarketItem
	LastPrice float64 `json:"last"`
}

type GeckoCoin struct {
	Id      string       `json:"id"`
	Symbol  string       `json:"symbol"`
	Tickers []TickerItem `json:"tickers"`
}
