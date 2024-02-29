package eventmodels

import (
	"time"

	"slack-trading/src/models"
)

type Tick struct {
	Timestamp time.Time
	Price     float64
	Source    models.DatafeedName
}

func NewTick(timestamp time.Time, price float64, datafeed models.DatafeedName) *Tick {
	return &Tick{
		Timestamp: timestamp,
		Price:     price,
		Source:    datafeed,
	}
}
