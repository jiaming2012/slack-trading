package models

import (
	"time"
)

type Tick struct {
	Symbol    string
	Timestamp time.Time
	Value     float64
}
