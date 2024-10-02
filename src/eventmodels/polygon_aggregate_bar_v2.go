package eventmodels

import "time"

type PolygonAggregateBarV2 struct {
	Volume    float64   `json:"volume"`
	VWAP      float64   `json:"-"`
	Open      float64   `json:"open"`
	Close     float64   `json:"close"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Timestamp time.Time `json:"datetime"`
}

func (p *PolygonAggregateBarV2) ToDTO() PolygonAggregateBarV2DTO {
	return PolygonAggregateBarV2DTO{
		Volume:    p.Volume,
		Open:      p.Open,
		Close:     p.Close,
		High:      p.High,
		Low:       p.Low,
		Timestamp: p.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
