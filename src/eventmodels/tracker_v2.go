package eventmodels

type TrackerV2 struct {
	BaseRequestEvent
	Type          TrackerType      `json:"type"`
	StartTracker  *StartTracker    `json:"startTracker"`
	StopTracker   *StopTracker     `json:"stopTracker"`
	SignalTracker *SignalTrackerV2 `json:"signalTracker"`
}

func (c *TrackerV2) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    TrackersStream,
		EventName:     CreateTrackerEvent,
		SchemaVersion: 2,
	}
}
