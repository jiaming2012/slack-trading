package eventmodels

import "time"

type OptionContractV1 struct {
	BaseRequestEvent
	Symbol           OptionSymbol `json:"symbol"`
	UnderlyingSymbol StockSymbol  `json:"underlying_symbol"`
	Description      string       `json:"description"`
	Strike           float64      `json:"strike"`
	OptionType       OptionType   `json:"option_type"`
	ContractSize     int          `json:"contract_size"`
	Expiration       time.Time    `json:"expiration"`
	ExpirationType   string       `json:"expiration_type"`
}

func (c *OptionContractV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    OptionContractStream,
		EventName:     CreateOptionContractEvent,
		SchemaVersion: 1,
	}
}
