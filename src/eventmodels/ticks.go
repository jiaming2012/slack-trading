package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type OptionType string

const (
	Call OptionType = "call"
	Put  OptionType = "put"
)

type OptionContractID uint

type OptionContractChainDTO struct {
	Options OptionChainDTO `json:"options"`
}

type OptionChainDTO struct {
	Values []*OptionChainTickDTO `json:"option"`
}

type OptionChainTickDTO struct {
	Symbol           string  `json:"symbol"`
	Description      string  `json:"description"`
	ChangePercentage float64 `json:"change_percentage"`
	LastVolume       int     `json:"last_volume"`
	AverageVolume    int     `json:"average_volume"`
	Bid              float64 `json:"bid"`
	Ask              float64 `json:"ask"`
	Last             float64 `json:"last"`
	Change           float64 `json:"change"`
	Volume           int     `json:"volume"`
	Open             float64 `json:"open"`
	High             float64 `json:"high"`
	Low              float64 `json:"low"`
	Close            float64 `json:"close"`
	PrevClose        float64 `json:"prevclose"`
	BidSize          int     `json:"bidsize"`
	AskSize          int     `json:"asksize"`
	OpenInterest     int     `json:"open_interest"`
}

func (d *OptionChainTickDTO) ToModel(id OptionContractID, uuid uuid.UUID, now time.Time) *OptionChainTick {
	return &OptionChainTick{
		BaseRequestEvent: BaseRequestEvent{
			Meta: MetaData{
				RequestID: uuid,
			},
		},
		Timestamp:        now,
		OptionContractID: id,
		ChangePercentage: d.ChangePercentage,
		LastVolume:       d.LastVolume,
		AverageVolume:    d.AverageVolume,
		Last:             d.Last,
		Bid:              d.Bid,
		Ask:              d.Ask,
		Change:           d.Change,
		Volume:           d.Volume,
		Open:             d.Open,
		High:             d.High,
		Low:              d.Low,
		Close:            d.Close,
		PrevClose:        d.PrevClose,
		BidSize:          d.BidSize,
		AskSize:          d.AskSize,
		OpenInterest:     d.OpenInterest,
	}
}

type OptionChainTick struct {
	BaseRequestEvent
	OptionContractID OptionContractID `json:"option_contract_id"`
	Timestamp        time.Time        `json:"timestamp"`
	ChangePercentage float64          `json:"change_percentage"`
	LastVolume       int              `json:"last_volume"`
	AverageVolume    int              `json:"average_volume"`
	Bid              float64          `json:"bid"`
	Ask              float64          `json:"ask"`
	Last             float64          `json:"last"`
	Change           float64          `json:"change"`
	Volume           int              `json:"volume"`
	Open             float64          `json:"open"`
	High             float64          `json:"high"`
	Low              float64          `json:"low"`
	Close            float64          `json:"close"`
	PrevClose        float64          `json:"prevclose"`
	BidSize          int              `json:"bidsize"`
	AskSize          int              `json:"asksize"`
	OpenInterest     int              `json:"open_interest"`
}

func (t *OptionChainTick) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		EventName:  CreateNewOptionChainTickEvent,
		StreamName: OptionChainTickStream,
	}
}

type OptionContract struct {
	ID           OptionContractID `json:"id"`
	Symbol       string           `json:"symbol"`
	Description  string           `json:"description"`
	Strike       float64          `json:"strike"`
	OptionType   OptionType       `json:"option_type"`
	ContractSize int              `json:"contract_size"`
}

type OptionChain struct {
	Calls          []OptionContractID `json:"calls"`
	Puts           []OptionContractID `json:"puts"`
	ExpirationDate string             `json:"expiration_date"`
	ExpirationType string             `json:"expiration_type"`
	Underlying     string             `json:"underlying"`
}
