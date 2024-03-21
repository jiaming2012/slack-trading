package eventmodels

import "fmt"

type OptionAlertType string

const (
	LastPrice OptionAlertType = "lastPrice"
	Delta     OptionAlertType = "delta"
)

func (t OptionAlertType) String() string {
	return string(t)
}

func NewOptionAlertType(s string) (OptionAlertType, error) {
	if s != "lastPrice" && s != "delta" {
		return "", fmt.Errorf("invalid OptionAlertType: %s", s)
	}

	return OptionAlertType(s), nil
}
