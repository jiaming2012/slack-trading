package eventmodels

import "time"

type OptionContractV3DTO struct {
	Symbol              OptionSymbol   `json:"symbol"`
	UnderlyingSymbol    StockSymbol    `json:"underlying_symbol"`
	Description         string         `json:"description"`
	Strike              float64        `json:"strike"`
	OptionType          OptionType     `json:"option_type"`
	ContractSize        int            `json:"contract_size"`
	Expiration          time.Time      `json:"expiration"`
	TimeUntilExpiration string         `json:"time_until_expiration"`
	ExpirationDate      ExpirationDate `json:"expiration_date"`
	ExpirationType      string         `json:"expiration_type"`
	Bid                 float64        `json:"bid"`
	Ask                 float64        `json:"ask"`
	Stats               OptionStats    `json:"stats"`
}
