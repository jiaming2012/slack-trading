package models

import "time"

type OrderFillEntry struct {
	Price    float64
	Quantity float64
	Time     time.Time
}
