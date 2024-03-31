package eventmodels

import (
	"fmt"
	"time"
)

type TradeRequestEvent struct {
	BaseRequestEvent
	Timestamp   time.Time
	Symbol      string
	Price       float64
	Volume      float64
	ResponseURL string
}

func (ev TradeRequestEvent) String() string {
	return fmt.Sprintf("TradeRequestEvent: %v (%.5f, %.2f)", ev.Symbol, ev.Volume, ev.Price)
}
