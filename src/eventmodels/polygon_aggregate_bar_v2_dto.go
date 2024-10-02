package eventmodels

import (
	"fmt"
	"time"
)

type PolygonAggregateBarV2DTO struct {
	Volume    float64 `json:"Volume" csv:"volume"`
	VWAP      float64 `json:"-" csv:"-"`
	Open      float64 `json:"Open" csv:"open"`
	Close     float64 `json:"Close" csv:"close"`
	High      float64 `json:"High" csv:"high"`
	Low       float64 `json:"Low" csv:"low"`
	Timestamp string  `json:"Datetime" csv:"timestamp"`
}

func (dto *PolygonAggregateBarV2DTO) ToModel() (*PolygonAggregateBarV2, error) {
	format := "2006-01-02 15:04:05"
	timestamp, err := time.Parse(format, dto.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return &PolygonAggregateBarV2{
		Volume:    dto.Volume,
		VWAP:      dto.VWAP,
		Open:      dto.Open,
		Close:     dto.Close,
		High:      dto.High,
		Low:       dto.Low,
		Timestamp: timestamp,
	}, nil
}
