package eventmodels

type StockTickQuoteDTO struct {
	Tick StockTickItemDTO `json:"quote"`
}
