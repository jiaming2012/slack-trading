package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type OptionAlertUpdateEvent struct {
	BaseRequestEvent2
	AlertID      uuid.UUID `json:"alert_id"`
	CreatedAt    time.Time `json:"created_at"`
	AlertMessage string    `json:"alert_message"`
}
