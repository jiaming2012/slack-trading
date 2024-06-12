package eventmodels

import (
	"time"
)

type Tick struct {
	Timestamp time.Time
	Price     float64
	Source    DatafeedName
}

func NewTick(timestamp time.Time, price float64, datafeed DatafeedName) *Tick {
	return &Tick{
		Timestamp: timestamp,
		Price:     price,
		Source:    datafeed,
	}
}
