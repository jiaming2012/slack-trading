package eventmodels

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TradeRequestEvent struct {
	RequestID   uuid.UUID
	Timestamp   time.Time
	Symbol      string
	Price       float64
	Volume      float64
	ResponseURL string
}

func (ev TradeRequestEvent) GetRequestID() uuid.UUID {
	return ev.RequestID
}

func (ev TradeRequestEvent) String() string {
	return fmt.Sprintf("TradeRequestEvent: %v (%.5f, %.2f)", ev.Symbol, ev.Volume, ev.Price)
}
