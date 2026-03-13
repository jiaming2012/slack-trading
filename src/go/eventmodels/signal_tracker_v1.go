package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type SignalTrackerV1 struct {
	Symbol    StockSymbol `json:"symbol"`
	Timestamp time.Time   `json:"timestamp"`
	Name      string      `json:"name"`
}

func NewSignalTrackerV1(symbol StockSymbol, timestamp time.Time, name string, requestID uuid.UUID) *TrackerV1 {
	return &TrackerV1{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeSignal,
		SignalTracker: &SignalTrackerV1{
			Symbol:    symbol,
			Timestamp: timestamp,
			Name:      name,
		},
	}
}
