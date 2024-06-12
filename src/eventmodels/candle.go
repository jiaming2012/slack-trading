package eventmodels

import (
	"time"
)

type Candle struct {
	Timestamp   time.Time
	LastUpdated time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
}
