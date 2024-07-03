package eventmodels

type tradierMarketsHistory struct {
	Day TradierCandleDTO `json:"day"`
}

type TradierMarketsHistoryResponseDTO struct {
	History tradierMarketsHistory `json:"history"`
}
