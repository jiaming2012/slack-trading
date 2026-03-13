package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type SignalTrackerV2 struct {
	Header    SignalRequestHeader `json:"header"`
	Timestamp time.Time           `json:"timestamp"`
	Name      string              `json:"name"`
}

func NewSignalTrackerV2(name string, header SignalRequestHeader, timestamp time.Time, requestID uuid.UUID) *TrackerV2 {
	return &TrackerV2{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeSignal,
		SignalTracker: &SignalTrackerV2{
			Header:    header,
			Timestamp: timestamp,
			Name:      name,
		},
	}
}
