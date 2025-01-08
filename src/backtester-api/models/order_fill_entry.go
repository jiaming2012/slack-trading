package models

import "time"

type OrderFillEntry struct {
	Price float64
	Time  time.Time
}
