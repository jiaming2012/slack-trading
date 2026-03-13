package eventmodels

import (
	"log"
	"time"
)

type TradierMarketsTimeSalesDTO struct {
	Time      string  `json:"time"`
	Timestamp int     `json:"timestamp"`
	Price     float64 `json:"price"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Vwap      float64 `json:"vwap"`
}

func (t *TradierMarketsTimeSalesDTO) GetTimestamp() time.Time {
	layout := "2006-01-02T15:04:00"

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("failed to load location: %v", err)
	}

	parsedTime, err := time.ParseInLocation(layout, t.Time, loc)
	if err != nil {
		log.Fatalf("failed to parse time: %v", err)
	}

	return parsedTime
}

func (t *TradierMarketsTimeSalesDTO) GetOpen() float64 {
	return t.Open
}

func (t *TradierMarketsTimeSalesDTO) GetHigh() float64 {
	return t.High
}

func (t *TradierMarketsTimeSalesDTO) GetLow() float64 {
	return t.Low
}

func (t *TradierMarketsTimeSalesDTO) GetClose() float64 {
	return t.Close
}

func (t *TradierMarketsTimeSalesDTO) GetVolume() float64 {
	return t.Volume
}
