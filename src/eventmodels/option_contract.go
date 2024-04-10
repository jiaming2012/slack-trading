package eventmodels

import "time"

type OptionContract struct {
	ID             OptionContractID `json:"id"`
	Symbol         string           `json:"symbol"`
	Description    string           `json:"description"`
	Strike         float64          `json:"strike"`
	OptionType     OptionType       `json:"option_type"`
	ContractSize   int              `json:"contract_size"`
	Expiration     time.Time        `json:"expiration"`
	ExpirationType string           `json:"expiration_type"`
}
