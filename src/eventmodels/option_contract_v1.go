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
	Bid              float64      `json:"bid"`
	Ask              float64      `json:"ask"`
}

func (c *OptionContractV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    OptionContractStream,
		EventName:     CreateOptionContractEvent,
		SchemaVersion: 1,
	}
}

func (c *OptionContractV1) ToDTO() *OptionContractV1DTO {
	return &OptionContractV1DTO{
		Symbol:           c.Symbol,
		UnderlyingSymbol: c.UnderlyingSymbol,
		Description:      c.Description,
		Strike:           c.Strike,
		OptionType:       c.OptionType,
		ContractSize:     c.ContractSize,
		Expiration:       c.Expiration,
		ExpirationType:   c.ExpirationType,
		Bid:              c.Bid,
		Ask:              c.Ask,
	}
}
