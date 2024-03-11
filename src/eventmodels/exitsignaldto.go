package eventmodels

type ExitSignalDTO struct {
	Signal      *SignalV2DTO `json:"signal"`
	ResetSignal *ResetSignal `json:"resetSignal"`
}

func (s *ExitSignalDTO) ToExitSignal() *ExitSignal {
	var signal *SignalV2

	if s.Signal != nil {
		signal = s.Signal.ToSignalV2()
		s.ResetSignal.AffectedSignal = signal
	}

	return &ExitSignal{
		Signal:      signal,
		ResetSignal: s.ResetSignal,
	}
}

func (s *SignalV2DTO) ToSignalV2() *SignalV2 {
	return &SignalV2{
		Name:        s.Name,
		isSatisfied: s.IsSatisfied,
		lastUpdated: s.LastUpdated,
	}
}
