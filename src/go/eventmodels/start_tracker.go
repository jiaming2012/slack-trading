package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StartTracker struct {
	UnderlyingSymbol      StockSymbol
	OptionContractSymbols []OptionSymbol
	Timestamp             time.Time
	Reason                string
}

func (t *StartTracker) ConvertToDTO() *StartTrackerDTO {
	contractIDs := make([]OptionSymbol, len(t.OptionContractSymbols))
	copy(contractIDs, t.OptionContractSymbols)
	return &StartTrackerDTO{
		UnderlyingSymbol: t.UnderlyingSymbol,
		OptionSymbols:    contractIDs,
		Timestamp:        t.Timestamp,
		Reason:           t.Reason,
	}
}

func NewStartFxTracker(symbol FxSymbol, timestamp time.Time, reason string, requestID uuid.UUID) *TrackerV3 {
	return &TrackerV3{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeStartFx,
		StartFxTracker: &StartFxTracker{
			Symbol:    symbol,
			Timestamp: timestamp,
			Reason:    reason,
		},
	}
}

func NewStartTracker(underlyingSymbol StockSymbol, optionContractSymbols []OptionSymbol, timestamp time.Time, reason string, requestID uuid.UUID) *TrackerV1 {
	return &TrackerV1{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeStart,
		StartTracker: &StartTracker{
			UnderlyingSymbol:      underlyingSymbol,
			OptionContractSymbols: optionContractSymbols,
			Timestamp:             timestamp,
			Reason:                reason,
		},
	}
}
