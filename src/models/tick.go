package models

import "time"

type Tick struct {
	Timestamp time.Time
	Bid       float64
	Ask       float64
}
