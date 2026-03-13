package eventmodels

import "time"

type StartFxTrackerDTO struct {
	Symbol    FxSymbol  `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

func (dto *StartFxTrackerDTO) ConvertToObject() *StartFxTracker {
	return &StartFxTracker{
		Symbol:    dto.Symbol,
		Timestamp: dto.Timestamp,
		Reason:    dto.Reason,
	}
}
