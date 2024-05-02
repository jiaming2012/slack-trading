package eventmodels

import "time"

type OptionChainTickV1 struct {
	BaseRequestEvent
	OptionContractID EventStreamID `json:"option_contract_id"`
	Timestamp        time.Time     `json:"timestamp"`
	ChangePercentage float64       `json:"change_percentage"`
	LastVolume       int           `json:"last_volume"`
	AverageVolume    int           `json:"average_volume"`
	Bid              float64       `json:"bid"`
	Ask              float64       `json:"ask"`
	Last             float64       `json:"last"`
	Change           float64       `json:"change"`
	Volume           int           `json:"volume"`
	Open             float64       `json:"open"`
	High             float64       `json:"high"`
	Low              float64       `json:"low"`
	Close            float64       `json:"close"`
	PrevClose        float64       `json:"prevclose"`
	BidSize          int           `json:"bidsize"`
	AskSize          int           `json:"asksize"`
	OpenInterest     int           `json:"open_interest"`
	Strike           float64       `json:"strike"`
	ContractSize     int           `json:"contract_size"`
	OptionType       OptionType    `json:"option_type"`
	ExpirationType   string        `json:"expiration_type"`
}

func (t *OptionChainTickV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		EventName:     CreateNewOptionChainTickEvent,
		StreamName:    OptionChainTickStream,
		SchemaVersion: 1,
	}
}
