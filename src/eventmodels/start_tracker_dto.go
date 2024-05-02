package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type StartTrackerDTO struct {
	UnderlyingSymbol  StockSymbol `json:"underlyingSymbol"`
	OptionContractIDs []uuid.UUID `json:"optionContractIDs"`
	Timestamp         time.Time   `json:"timestamp"`
	Reason            string      `json:"reason"`
}

func (dto *StartTrackerDTO) ConvertToObject() *StartTracker {
	contractIDs := make([]EventStreamID, len(dto.OptionContractIDs))
	for i, id := range dto.OptionContractIDs {
		contractIDs[i] = EventStreamID(id)
	}
	return &StartTracker{
		UnderlyingSymbol:  dto.UnderlyingSymbol,
		OptionContractIDs: contractIDs,
		Timestamp:         dto.Timestamp,
		Reason:            dto.Reason,
	}
}
