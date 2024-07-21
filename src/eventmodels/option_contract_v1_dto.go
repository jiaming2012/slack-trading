package eventmodels

import "time"

type OptionContractV1DTO struct {
	Timestamp        time.Time    `json:"timestamp"`
	Symbol           OptionSymbol `json:"symbol"`
	UnderlyingSymbol StockSymbol  `json:"underlying_symbol"`
	Description      string       `json:"description"`
	Strike           float64      `json:"strike"`
	OptionType       OptionType   `json:"option_type"`
	ContractSize     int          `json:"contract_size"`
	Expiration       time.Time    `json:"expiration"`
	ExpirationType   string       `json:"expiration_type"`
	Bid              float64      `json:"bid"`
	Ask              float64      `json:"ask"`
	AverageFillPrice float64      `json:"avg_fill_price"`
}
