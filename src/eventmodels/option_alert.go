package eventmodels

import "github.com/google/uuid"

type OptionAlert struct {
	ID           uuid.UUID            `json:"id"`
	AlertType    OptionAlertType      `json:"alertType"`
	OptionSymbol string               `json:"optionSymbol"`
	Condition    OptionAlertCondition `json:"condition"`
}

func NewOptionAlert(alertType OptionAlertType, optionSymbol string, condition OptionAlertCondition) *OptionAlert {
	return &OptionAlert{
		ID:           uuid.New(),
		AlertType:    alertType,
		OptionSymbol: optionSymbol,
		Condition:    condition,
	}
}
