package eventmodels

type TrackerV1 struct {
	BaseRequestEvent
	Type          TrackerType      `json:"type"`
	StartTracker  *StartTracker    `json:"startTracker"`
	StopTracker   *StopTracker     `json:"stopTracker"`
	SignalTracker *SignalTrackerV1 `json:"signalTracker"`
}

func (c *TrackerV1) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    TrackersStream,
		EventName:     CreateTrackerEvent,
		SchemaVersion: 1,
	}
}
