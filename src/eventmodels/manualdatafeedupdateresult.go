package eventmodels

import (
	"time"

	"github.com/google/uuid"

	"slack-trading/src/models"
)

type ManualDatafeedUpdateResult struct {
	Meta 	*MetaData   `json:"meta"`
	RequestID uuid.UUID   `json:"requestID"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Tick      models.Tick `json:"tick"`
}

func (r *ManualDatafeedUpdateResult) GetMetaData() *MetaData {
	return r.Meta
}

func (r *ManualDatafeedUpdateResult) GetRequestID() uuid.UUID {
	return r.RequestID
}

func NewManualDatafeedUpdateResult(requestID uuid.UUID, updatedAt time.Time, tick models.Tick) *ManualDatafeedUpdateResult {
	return &ManualDatafeedUpdateResult{RequestID: requestID, UpdatedAt: updatedAt, Tick: tick}
}
