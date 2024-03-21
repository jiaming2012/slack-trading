package eventmodels

import "github.com/google/uuid"

type OptionAlert struct {
	ID           uuid.UUID            `json:"id"`
	AlertType    OptionAlertType      `json:"alertType"`
	OptionSymbol string               `json:"optionSymbol"`
	Condition    OptionAlertCondition `json:"condition"`
}

func NewOptionAlert(id uuid.UUID, alertType OptionAlertType, optionSymbol string, condition OptionAlertCondition) *OptionAlert {
	return &OptionAlert{
		ID:           id,
		AlertType:    alertType,
		OptionSymbol: optionSymbol,
		Condition:    condition,
	}
}
