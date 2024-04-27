package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StartTracker struct {
	UnderlyingSymbol  string          `json:"underlyingSymbol"`
	OptionContractIDs []EventStreamID `json:"optionContractIDs"`
	Timestamp         time.Time       `json:"timestamp"`
	Reason            string          `json:"reason"`
}

func NewStartTracker(underlyingSymbol string, optionContractIDs []EventStreamID, timestamp time.Time, reason string, requestID uuid.UUID) *Tracker {
	return &Tracker{
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
