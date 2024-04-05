package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StockTickDTO struct {
	Quotes StockTickQuoteDTO `json:"quotes"`
}

type StockTickQuoteDTO struct {
	Tick StockTickItemDTO `json:"quote"`
}

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
}

type StockTick struct {
	BaseRequestEvent
	Timestamp        time.Time `json:"timestamp"`
	Symbol           string    `json:"symbol"`
	LastPrice        float64   `json:"last"`
	Volume           float64   `json:"volume"`
	High             float64   `json:"high"`
	Low              float64   `json:"low"`
	Open             float64   `json:"open"`
	Close            float64   `json:"close"`
	AverageVolume    int       `json:"average_volume"`
	LastVolume       int       `json:"last_volume"`
	ChangePercentage float64   `json:"change_percentage"`
	AskSize          int       `json:"asksize"`
	BidSize          int       `json:"bidsize"`
}

type StockTicks []StockTick

func (ticks StockTicks) ToRows() [][]interface{} {
	results := make([][]interface{}, 0)

	for i := len(ticks) - 1; i >= 0; i-- {
		results = append(results, []interface{}{
			ticks[i].Timestamp.Format(time.RFC3339),
			ticks[i].LastPrice,
			ticks[i].Volume,
		})
	}

	return results
}

func (d *StockTick) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		EventName:  CreateNewStockTickEvent,
		StreamName: StockTickStream,
	}
}

func (d *StockTickItemDTO) ToModel(uuid uuid.UUID, now time.Time) *StockTick {
	return &StockTick{
		BaseRequestEvent: BaseRequestEvent{
			Meta: MetaData{
				RequestID: uuid,
			},
		},
		Timestamp:        now,
		Symbol:           d.Symbol,
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
	}
}
