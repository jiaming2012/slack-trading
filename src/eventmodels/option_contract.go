package eventmodels

import "time"

type OptionContract struct {
	BaseRequestEvent
	ID             OptionContractID // we use the event number assigned by eventstore as the ID
	Symbol         string           `json:"symbol"`
	Description    string           `json:"description"`
	Strike         float64          `json:"strike"`
	OptionType     OptionType       `json:"option_type"`
	ContractSize   int              `json:"contract_size"`
	Expiration     time.Time        `json:"expiration"`
	ExpirationType string           `json:"expiration_type"`
}

func (c *OptionContract) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: OptionContractStream,
		EventName:  CreateOptionContractEvent,
	}
}
