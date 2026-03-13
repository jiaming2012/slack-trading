package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type TrackerV3 struct {
	BaseRequestEvent
	Type           TrackerType      `json:"type"`
	StartTracker   *StartTracker    `json:"startTracker"`
	StopTracker    *StopTracker     `json:"stopTracker"`
	SignalTracker  *SignalTrackerV2 `json:"signalTracker"`
	StartFxTracker *StartFxTracker  `json:"startFxTracker"`
	streamName     StreamName       `json:"-"`
}

func (c *TrackerV3) GetSavedEventParameters() SavedEventParameters {
	if c.streamName == "" { // for backwards compatibility
		c.streamName = TrackersStream
	}

	return SavedEventParameters{
		StreamName:    c.streamName,
		EventName:     CreateTrackerEvent,
		SchemaVersion: 3,
	}
}

func NewSignalTrackerV3(name string, header SignalRequestHeader, timestamp time.Time, requestID uuid.UUID, streamName StreamName) *TrackerV3 {
	return &TrackerV3{
		BaseRequestEvent: BaseRequestEvent{Meta: MetaData{RequestID: requestID}},
		Type:             TrackerTypeSignal,
		SignalTracker: &SignalTrackerV2{
			Header:    header,
			Timestamp: timestamp,
			Name:      name,
		},
		streamName: streamName,
	}
}
