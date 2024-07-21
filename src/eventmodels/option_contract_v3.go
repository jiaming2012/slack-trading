package eventmodels

import "time"

type OptionContractV3 struct {
	BaseRequestEvent
	Timestamp        time.Time      `json:"timestamp"`
	Symbol           OptionSymbol   `json:"symbol"`
	UnderlyingSymbol StockSymbol    `json:"underlying_symbol"`
	Description      string         `json:"description"`
	Strike           float64        `json:"strike"`
	OptionType       OptionType     `json:"option_type"`
	ContractSize     int            `json:"contract_size"`
	Expiration       time.Time      `json:"expiration"`
	ExpirationDate   ExpirationDate `json:"expiration_date"`
	ExpirationType   string         `json:"expiration_type"`
	Bid              float64        `json:"bid"`
	Ask              float64        `json:"ask"`
	AverageFillPrice float64        `json:"average_fill_price"`
}

func (c *OptionContractV3) TimeUntilExpiration(now time.Time) time.Duration {
	return c.Expiration.Sub(now)
}

func (c *OptionContractV3) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    OptionContractStream,
		EventName:     CreateOptionContractEvent,
		SchemaVersion: 3,
	}
}

func (c *OptionContractV3) ToDTOV1() *OptionContractV1DTO {
	return &OptionContractV1DTO{
		Timestamp:        c.Timestamp,
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
		AverageFillPrice: c.AverageFillPrice,
	}
}

func (c *OptionContractV3) ToDTO(now time.Time) *OptionContractV3DTO {
	return &OptionContractV3DTO{
		Symbol:              c.Symbol,
		UnderlyingSymbol:    c.UnderlyingSymbol,
		Description:         c.Description,
		Strike:              c.Strike,
		OptionType:          c.OptionType,
		ContractSize:        c.ContractSize,
		Expiration:          c.Expiration,
		TimeUntilExpiration: FormatDuration(c.TimeUntilExpiration(now)),
		ExpirationDate:      c.ExpirationDate,
		ExpirationType:      c.ExpirationType,
		Bid:                 c.Bid,
		Ask:                 c.Ask,
	}
}
