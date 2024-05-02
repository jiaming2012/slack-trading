package eventmodels

type TrackerDTO struct {
	BaseRequestEvent
	Type          TrackerType      `json:"type"`
	StartTracker  *StartTrackerDTO `json:"startTracker"`
	StopTracker   *StopTrackerDTO  `json:"stopTracker"`
	SignalTracker *SignalTracker   `json:"signalTracker"`
}

func (dto *TrackerDTO) ConvertToObject() *Tracker {
	return &Tracker{
		BaseRequestEvent: dto.BaseRequestEvent,
		Type:             dto.Type,
		StartTracker:     dto.StartTracker.ConvertToObject(),
		StopTracker:      dto.StopTracker.ConvertToObject(),
		SignalTracker:    dto.SignalTracker,
	}
}
