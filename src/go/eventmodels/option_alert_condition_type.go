package eventmodels

import "fmt"

type OptionAlertConditionType string

const (
	Cross OptionAlertConditionType = "cross"
)

func (t OptionAlertConditionType) String() string {
	return string(t)
}

func NewOptionAlertConditionType(s string) (OptionAlertConditionType, error) {
	if s != "cross" {
		return "", fmt.Errorf("invalid OptionAlertConditionType: %s", s)
	}

	return OptionAlertConditionType(s), nil
}
