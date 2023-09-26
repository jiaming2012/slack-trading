package models

import "time"

type Tick struct {
	Timestamp time.Time `json:"timestamp"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
}
