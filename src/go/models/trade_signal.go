package models

import (
	"time"

	"github.com/google/uuid"
)

type TradeSignal struct {
	BaseRequestEvent
	ID         uuid.UUID              `json:"id"`
	Name       SignalName             `json:"name"`
	Symbol     StockSymbol            `json:"symbol"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
	streamName StreamName             `json:"-"`
}

func NewTradeSignal(name SignalName, symbol StockSymbol, timestamp time.Time, attributes map[string]interface{}) *TradeSignal {
	return &TradeSignal{
		ID:         uuid.New(),
		Name:       name,
		Symbol:     symbol,
		Timestamp:  timestamp,
		Attributes: attributes,
		streamName: TradeSignalStream,
	}
}

func (s *TradeSignal) GetSavedEventParameters() SavedEventParameters {
	stream := s.streamName
	if stream == "" {
		stream = TradeSignalStream
	}
	return SavedEventParameters{
		StreamName:    stream,
		EventName:     TradeSignalEventName,
		SchemaVersion: 1,
	}
}

// SetStreamName overrides the ESDB stream for this signal.
// Used by SavePlayground to route sim signals to opaque per-run streams
// instead of the global trade-signals stream.
func (s *TradeSignal) SetStreamName(name StreamName) {
	s.streamName = name
}
