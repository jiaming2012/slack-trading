package eventmodels

import "time"

type StockTickV1 struct {
	BaseRequestEvent
	Timestamp        time.Time   `json:"timestamp"`
	Symbol           StockSymbol `json:"symbol"`
	LastPrice        float64     `json:"last"`
	Volume           float64     `json:"volume"`
	High             float64     `json:"high"`
	Low              float64     `json:"low"`
	Open             float64     `json:"open"`
	Close            float64     `json:"close"`
	AverageVolume    int         `json:"average_volume"`
	LastVolume       int         `json:"last_volume"`
	ChangePercentage float64     `json:"change_percentage"`
	AskSize          int         `json:"asksize"`
	BidSize          int         `json:"bidsize"`
	Ask              float64     `json:"ask"`
	Bid              float64     `json:"bid"`
}

func (d *StockTickV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		EventName:     CreateNewStockTickEvent,
		StreamName:    StockTickStream,
		SchemaVersion: 1,
	}
}
