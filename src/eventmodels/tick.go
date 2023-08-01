package eventmodels

import "time"

type Tick struct {
	Timestamp time.Time
	Price     float64
}
