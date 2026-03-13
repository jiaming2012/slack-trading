package eventmodels

import "time"

type HistOptionOhlc struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int
}
