package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

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
	Strike           float64 `json:"strike"`
	ContractSize     int     `json:"contract_size"`
	OptionType       string  `json:"option_type"`
	ExpirationType   string  `json:"expiration_type"`
}

func (d *OptionChainTickDTO) ToModel(id OptionSymbol, uuid uuid.UUID, now time.Time) *OptionChainTickV1 {
	// todo: add error handling
	return &OptionChainTickV1{
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
		Strike:           d.Strike,
		ContractSize:     d.ContractSize,
		OptionType:       OptionType(d.OptionType),
		ExpirationType:   d.ExpirationType,
	}
}
