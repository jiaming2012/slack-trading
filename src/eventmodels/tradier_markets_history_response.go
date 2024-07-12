package eventmodels

type tradierMarketsHistory struct {
	Day CandleDTO `json:"day"`
}

type TradierMarketsHistoryResponseDTO struct {
	History tradierMarketsHistory `json:"history"`
}
