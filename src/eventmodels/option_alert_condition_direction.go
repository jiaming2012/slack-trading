package eventmodels

import "fmt"

type OptionAlertConditionDirection string

const (
	Above OptionAlertConditionDirection = "above"
	Below OptionAlertConditionDirection = "below"
)

func (d OptionAlertConditionDirection) String() string {
	return string(d)
}

func NewOptionAlertConditionDirection(s string) (OptionAlertConditionDirection, error) {
	if s != "above" && s != "below" {
		return "", fmt.Errorf("invalid OptionAlertConditionDirection: %s", s)
	}

	return OptionAlertConditionDirection(s), nil
}
