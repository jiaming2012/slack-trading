package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type SignalTracker struct {
	Symbol    StockSymbol `json:"symbol"`
	Timestamp time.Time   `json:"timestamp"`
	Name      string      `json:"name"`
}

func NewSignalTracker(symbol StockSymbol, timestamp time.Time, name string, requestID uuid.UUID) *TrackerV1 {
	return &TrackerV1{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeSignal,
		SignalTracker: &SignalTracker{
			Symbol:    symbol,
			Timestamp: timestamp,
			Name:      name,
		},
	}
}
