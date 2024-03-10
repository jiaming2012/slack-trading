package eventmodels

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TradeFulfilledEvent struct {
	Meta           *MetaData
	RequestID      uuid.UUID
	Timestamp      time.Time
	ResponseURL    string
	Symbol         string
	RequestedPrice float64
	ExecutedPrice  float64
	Volume         float64
}

func (ev TradeFulfilledEvent) GetMetaData() *MetaData {
	return ev.Meta
}

func (ev TradeFulfilledEvent) GetRequestID() uuid.UUID {
	return ev.RequestID
}

func (ev TradeFulfilledEvent) String() string {
	// 1.05 btc @30023.70 successfully placed
	return fmt.Sprintf("TradeFulfilledEvent: %v (%.5f, %.2f)", ev.Symbol, ev.Volume, ev.ExecutedPrice)
}
