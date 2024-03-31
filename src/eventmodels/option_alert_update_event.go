package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type OptionAlertUpdateEvent struct {
	BaseRequestEvent
	AlertID      uuid.UUID `json:"alert_id"`
	CreatedAt    time.Time `json:"created_at"`
	AlertMessage string    `json:"alert_message"`
}

func (r *OptionAlertUpdateEvent) GetSavedEventParameters() SavedEventParameters {
	return SavedEventParameters{
		StreamName: OptionAlertsStreamName,
		EventName:  OptionAlertUpdateEventName,
	}
}
