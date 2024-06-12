package eventmodels

import "fmt"

type OptionAlertCondition struct {
	Type      OptionAlertConditionType      `json:"type"`
	Direction OptionAlertConditionDirection `json:"direction"`
	Value     float64                       `json:"value"`
}

func NewOptionAlertCondition(inputType string, inputDirection string, inputValue float64) (OptionAlertCondition, error) {
	var err error
	var condition OptionAlertCondition
	condition.Type, err = NewOptionAlertConditionType(inputType)
	if err != nil {
		return condition, fmt.Errorf("invalid OptionAlertConditionType: %w", err)
	}

	condition.Direction, err = NewOptionAlertConditionDirection(inputDirection)
	if err != nil {
		return condition, fmt.Errorf("invalid OptionAlertConditionDirection: %w", err)
	}

	condition.Value = inputValue
	return condition, nil
}
