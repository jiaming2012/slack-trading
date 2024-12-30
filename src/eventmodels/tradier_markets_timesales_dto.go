package eventmodels

type TradierMarketsTimeSalesDTO struct {
	Time      string  `json:"time"`
	Timestamp int     `json:"timestamp"`
	Price     float64 `json:"price"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int     `json:"volume"`
	Vwap      float64 `json:"vwap"`
}
