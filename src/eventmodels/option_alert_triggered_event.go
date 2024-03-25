package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type OptionAlertUpdateEvent struct {
	AlertID   uuid.UUID `json:"alert_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (ev *OptionAlertUpdateEvent) GetMetaData() *MetaData {
	return nil
}

func (ev *OptionAlertUpdateEvent) GetRequestID() uuid.UUID {
	return ev.AlertID
}
