package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StockTickItemDTO struct {
	Symbol           string  `json:"symbol"`
	LastPrice        float64 `json:"last"`
	Volume           float64 `json:"volume"`
	High             float64 `json:"high"`
	Low              float64 `json:"low"`
	Open             float64 `json:"open"`
	Close            float64 `json:"close"`
	AverageVolume    int     `json:"average_volume"`
	LastVolume       int     `json:"last_volume"`
	ChangePercentage float64 `json:"change_percentage"`
	AskSize          int     `json:"asksize"`
	BidSize          int     `json:"bidsize"`
	Ask              float64 `json:"ask"`
	Bid              float64 `json:"bid"`
}

func (d *StockTickItemDTO) ToModel(uuid uuid.UUID, now time.Time) *StockTick {
	return &StockTick{
		BaseRequestEvent: BaseRequestEvent{
			Meta: MetaData{
				RequestID: uuid,
			},
		},
		Timestamp:        now,
		Symbol:           StockSymbol(d.Symbol),
		LastPrice:        d.LastPrice,
		Volume:           d.Volume,
		High:             d.High,
		Low:              d.Low,
		Open:             d.Open,
		Close:            d.Close,
		AverageVolume:    d.AverageVolume,
		LastVolume:       d.LastVolume,
		ChangePercentage: d.ChangePercentage,
		AskSize:          d.AskSize,
		BidSize:          d.BidSize,
		Ask:              d.Ask,
		Bid:              d.Bid,
	}
}
