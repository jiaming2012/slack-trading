package eventmodels

import "time"

type TrackerType string

const (
	TrackerTypeStart TrackerType = "start"
	TrackerTypeStop  TrackerType = "stop"
)

type Tracker struct {
	BaseRequestEvent
	Timestamp         time.Time        `json:"timestamp"`
	Reason            string           `json:"reason"`
	Type              TrackerType      `json:"type"`
	UnderlyingSymbol  *string          `json:"underlyingSymbol"`
	OptionContractIDs *[]EventStreamID `json:"optionContractIDs"`
	TrackerStartID    *EventStreamID   `json:"trackerStartID"`
}

func (c *Tracker) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: TrackersStream,
		EventName:  CreateTrackerEvent,
	}
}
