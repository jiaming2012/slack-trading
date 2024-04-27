package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StopTracker struct {
	TrackerStartID EventStreamID `json:"trackerStartID"`
	Timestamp      time.Time     `json:"timestamp"`
	Reason         string        `json:"reason"`
}

func NewStopTracker(trackerStartID EventStreamID, timestamp time.Time, reason string, requestID uuid.UUID) *Tracker {
	return &Tracker{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeStop,
		StopTracker: &StopTracker{
			TrackerStartID: trackerStartID,
			Timestamp:      timestamp,
			Reason:         reason,
		},
	}
}
