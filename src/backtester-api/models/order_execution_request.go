package models

import "time"

type OrderExecutionRequest struct {
	Price    float64
	Quantity float64
	Time     time.Time
}
