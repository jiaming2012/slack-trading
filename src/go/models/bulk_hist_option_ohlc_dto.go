package models

type BulkHistOptionOhlcDTO struct {
	Candles  []HistOptionOhlcDTO     `json:"ticks"`
	Contract ThetaDataOptionContract `json:"contract"`
}
