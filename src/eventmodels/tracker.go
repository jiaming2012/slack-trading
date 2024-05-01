package eventmodels

type Tracker struct {
	BaseRequestEvent
	Type          TrackerType    `json:"type"`
	StartTracker  *StartTracker  `json:"startTracker"`
	StopTracker   *StopTracker   `json:"stopTracker"`
	SignalTracker *SignalTracker `json:"signalTracker"`
}

func (c *Tracker) GetSavedEventParameters() []SavedEventParameters {
	return []SavedEventParameters{
		{
			StreamName: TrackersStream,
			EventName:  CreateTrackerEvent,
		},
	}
}
