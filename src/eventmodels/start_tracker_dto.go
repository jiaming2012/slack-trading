package eventmodels

import (
	"time"
)

type StartTrackerDTO struct {
	UnderlyingSymbol StockSymbol    `json:"underlyingSymbol"`
	OptionSymbols    []OptionSymbol `json:"contractSymbols"`
	Timestamp        time.Time      `json:"timestamp"`
	Reason           string         `json:"reason"`
}

func (dto *StartTrackerDTO) ConvertToObject() *StartTracker {
	contractIDs := make([]OptionSymbol, len(dto.OptionSymbols))
	for i, symbol := range dto.OptionSymbols {
		contractIDs[i] = OptionSymbol(symbol)
	}
	return &StartTracker{
		UnderlyingSymbol:      dto.UnderlyingSymbol,
		OptionContractSymbols: contractIDs,
		Timestamp:             dto.Timestamp,
		Reason:                dto.Reason,
	}
}
