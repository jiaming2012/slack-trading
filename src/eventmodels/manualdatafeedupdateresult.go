package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type ManualDatafeedUpdateResult struct {
	BaseResponseEvent
	UpdatedAt time.Time `json:"updatedAt"`
	Tick      Tick      `json:"tick"`
}

func NewManualDatafeedUpdateResult(requestID uuid.UUID, updatedAt time.Time, tick Tick) *ManualDatafeedUpdateResult {
	return &ManualDatafeedUpdateResult{BaseResponseEvent: BaseResponseEvent{Meta: MetaData{RequestID: requestID}}, UpdatedAt: updatedAt, Tick: tick}
}
