package eventmodels

import (
	"time"

	"github.com/google/uuid"
)

type OptionAlert struct {
	ID             uuid.UUID            `json:"id"`
	AlertType      OptionAlertType      `json:"alert_type"`
	OptionSymbol   string               `json:"option_symbol"`
	Condition      OptionAlertCondition `json:"condition"`
	IsOptionActive bool                 `json:"is_option_active"`
	TriggeredAt    *time.Time           `json:"triggered_at"`
}

func NewOptionAlert(id uuid.UUID, alertType OptionAlertType, optionSymbol string, condition OptionAlertCondition) *OptionAlert {
	return &OptionAlert{
		ID:             id,
		AlertType:      alertType,
		OptionSymbol:   optionSymbol,
		Condition:      condition,
		IsOptionActive: true,
		TriggeredAt:    nil,
	}
}
