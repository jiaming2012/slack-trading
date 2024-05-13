package eventmodels

type TrackerV3 struct {
	BaseRequestEvent
	Type           TrackerType      `json:"type"`
	StartTracker   *StartTracker    `json:"startTracker"`
	StopTracker    *StopTracker     `json:"stopTracker"`
	SignalTracker  *SignalTrackerV2 `json:"signalTracker"`
	StartFxTracker *StartFxTracker  `json:"startFxTracker"`
}

func (c *TrackerV3) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName:    TrackersStream,
		EventName:     CreateTrackerEvent,
		SchemaVersion: 3,
	}
}
