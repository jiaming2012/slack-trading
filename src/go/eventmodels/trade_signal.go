package eventmodels

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
	return SavedEventParameters{
		StreamName:    TradeSignalStream,
		EventName:     TradeSignalEventName,
		SchemaVersion: 1,
	}
}
