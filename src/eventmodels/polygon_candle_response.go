package eventmodels

import "time"

type PolygonCandleDTO struct {
	Volume    float64 `json:"v"`
	Open      float64 `json:"o"`
	Close     float64 `json:"c"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Timestamp int64   `json:"t"`
	Count     int     `json:"n"`
	Vwap      float64 `json:"vw"`
}

func (d *PolygonCandleDTO) ToCandleDTO() (*CandleDTO, error) {
	// convert from Unix Msec timestamp for the start of the aggregate window.
	timestamp := time.Unix(0, d.Timestamp*int64(time.Millisecond)).UTC()

	return &CandleDTO{
		Volume: int(d.Volume),
		Open:   d.Open,
		Close:  d.Close,
		High:   d.High,
		Low:    d.Low,
		Date:   timestamp.Format("2006-01-02 15:04:05"),
	}, nil
}

type PolygonCandleResponse struct {
	Ticker       string             `json:"ticker"`
	QueryCount   int                `json:"queryCount"`
	ResultsCount int                `json:"resultsCount"`
	Adjusted     bool               `json:"adjusted"`
	Results      []PolygonCandleDTO `json:"results"`
	Status       string             `json:"status"`
}
