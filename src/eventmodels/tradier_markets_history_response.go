package eventmodels

type tradierMarketsHistory struct {
	Day struct {
		Date   string  `json:"date"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume int     `json:"volume"`
	} `json:"day"`
}

type TradierMarketsHistoryResponseDTO struct {
	History tradierMarketsHistory `json:"history"`
}
