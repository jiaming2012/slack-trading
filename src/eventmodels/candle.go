package eventmodels

import (
	"time"
)

type ICandle interface {
	GetTimestamp() time.Time
	GetOpen() float64
	GetHigh() float64
	GetLow() float64
	GetClose() float64
	GetVolume() float64
}

type Candle struct {
	Timestamp   time.Time
	LastUpdated time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
}
