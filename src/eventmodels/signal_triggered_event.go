package eventmodels

import (
	"context"
	"time"
)

type SignalTriggeredEvent struct {
	Timestamp time.Time
	Symbol    StockSymbol
	Signal    SignalName
	Ctx       context.Context
}
