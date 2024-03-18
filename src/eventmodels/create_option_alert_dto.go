package eventmodels

import "fmt"

type CreateOptionAlertDTO struct {
	AlertType    string                  `json:"alertType"`
	OptionSymbol string                  `json:"optionSymbol"`
	Condition    OptionAlertConditionDTO `json:"condition"`
}

func (dto CreateOptionAlertDTO) Convert() (*OptionAlert, error) {
	alertType, err := NewOptionAlertType(dto.AlertType)
	if err != nil {
		return nil, fmt.Errorf("CreateOptionAlertDTO: invalid OptionAlertType: %w", err)
	}

	condition, err := NewOptionAlertCondition(dto.Condition.Type, dto.Condition.Direction, dto.Condition.Value)
	if err != nil {
		return nil, fmt.Errorf("CreateOptionAlertDTO: invalid OptionAlertCondition: %w", err)
	}

	return NewOptionAlert(alertType, dto.OptionSymbol, condition), nil
}
