package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StartTracker struct {
	UnderlyingSymbol  StockSymbol
	OptionContractIDs []EventStreamID
	Timestamp         time.Time
	Reason            string
}

func (t *StartTracker) ConvertToDTO() *StartTrackerDTO {
	contractIDs := make([]uuid.UUID, len(t.OptionContractIDs))
	for i, id := range t.OptionContractIDs {
		contractIDs[i] = uuid.UUID(id)
	}
	return &StartTrackerDTO{
		UnderlyingSymbol:  t.UnderlyingSymbol,
		OptionContractIDs: contractIDs,
		Timestamp:         t.Timestamp,
		Reason:            t.Reason,
	}
}

func NewStartTracker(underlyingSymbol StockSymbol, optionContractIDs []EventStreamID, timestamp time.Time, reason string, requestID uuid.UUID) *TrackerV1 {
	return &TrackerV1{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeStart,
		StartTracker: &StartTracker{
			UnderlyingSymbol:  underlyingSymbol,
			OptionContractIDs: optionContractIDs,
			Timestamp:         timestamp,
			Reason:            reason,
		},
	}
}
