package eventmodels

import "time"

type ExitSignal struct {
	Signal      *SignalV2
	ResetSignal *ResetSignal
}

func NewExitSignal(signal *SignalV2, resetSignal *ResetSignal) *ExitSignal {
	return &ExitSignal{Signal: signal, ResetSignal: resetSignal}
}

func (s *ExitSignal) Update(signalType SignalType) {
	now := time.Now().UTC()

	switch signalType {
	case SignalTypeExit:
		s.Signal.Update(true, now)
	case SignalTypeReset:
		s.ResetSignal.Update(now)
	default:
		return
	}
}

func (s *ExitSignal) ConvertToDTO() *ExitSignalDTO {
	return &ExitSignalDTO{
		Signal:      s.Signal.ConvertToDTO(),
		ResetSignal: s.ResetSignal,
	}
}
