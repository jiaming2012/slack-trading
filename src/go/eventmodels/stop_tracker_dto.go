package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StopTrackerDTO struct {
	TrackerStartID uuid.UUID `json:"trackerStartID"`
	Timestamp      time.Time `json:"timestamp"`
	Reason         string    `json:"reason"`
}

func (dto *StopTrackerDTO) ConvertToObject() *StopTracker {
	return &StopTracker{
		TrackerStartID: EventStreamID(dto.TrackerStartID),
		Timestamp:      dto.Timestamp,
		Reason:         dto.Reason,
	}
}
