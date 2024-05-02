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

func (t *StopTracker) ConvertToDTO() *StopTrackerDTO {
	return &StopTrackerDTO{
		TrackerStartID: uuid.UUID(t.TrackerStartID),
		Timestamp:      t.Timestamp,
		Reason:         t.Reason,
	}
}

func NewStopTracker(trackerStartID EventStreamID, timestamp time.Time, reason string, requestID uuid.UUID) *TrackerV1 {
	return &TrackerV1{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeStop,
		StopTracker: &StopTracker{
			TrackerStartID: trackerStartID,
			Timestamp:      timestamp,
			Reason:         reason,
		},
	}
}
