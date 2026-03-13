package eventmodels

import (
	"fmt"

	"github.com/google/uuid"
)

type CreateOptionAlertDTO struct {
	AlertType    string                  `json:"alert_type"`
	OptionSymbol string                  `json:"option_symbol"`
	Condition    OptionAlertConditionDTO `json:"condition"`
}

func (dto CreateOptionAlertDTO) NewObject(id uuid.UUID) (*OptionAlert, error) {
	alertType, err := NewOptionAlertType(dto.AlertType)
	if err != nil {
		return nil, fmt.Errorf("CreateOptionAlertDTO: invalid OptionAlertType: %w", err)
	}

	condition, err := NewOptionAlertCondition(dto.Condition.Type, dto.Condition.Direction, dto.Condition.Value)
	if err != nil {
		return nil, fmt.Errorf("CreateOptionAlertDTO: invalid OptionAlertCondition: %w", err)
	}

	return NewOptionAlert(id, alertType, dto.OptionSymbol, condition), nil
}
