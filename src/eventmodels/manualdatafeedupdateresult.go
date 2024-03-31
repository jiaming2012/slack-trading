package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type ManualDatafeedUpdateResult struct {
	BaseResponseEvent2
	UpdatedAt time.Time `json:"updatedAt"`
	Tick      Tick      `json:"tick"`
}

func NewManualDatafeedUpdateResult(requestID uuid.UUID, updatedAt time.Time, tick Tick) *ManualDatafeedUpdateResult {
	return &ManualDatafeedUpdateResult{BaseResponseEvent2: BaseResponseEvent2{Meta: MetaData{RequestID: requestID}}, UpdatedAt: updatedAt, Tick: tick}
}
