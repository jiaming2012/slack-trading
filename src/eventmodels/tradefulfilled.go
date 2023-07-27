package eventmodels

import (
	"fmt"
	"time"
)

type TradeFulfilledEvent struct {
	Timestamp      time.Time
	Symbol         string
	RequestedPrice float64
	ExecutedPrice  float64
	Volume         float64
}

func (ev TradeFulfilledEvent) String() string {
	return fmt.Sprintf("TradeFulfilledEvent: %v (%.5f, %.2f)", ev.Symbol, ev.Volume, ev.ExecutedPrice)
}
