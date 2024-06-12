package eventmodels

import "fmt"

type OptionAlertType string

const (
	LastPrice OptionAlertType = "last_price"
	Delta     OptionAlertType = "delta"
)

func (t OptionAlertType) String() string {
	return string(t)
}

func NewOptionAlertType(s string) (OptionAlertType, error) {
	if s != string(LastPrice) && s != string(Delta) {
		return "", fmt.Errorf("invalid OptionAlertType: %s", s)
	}

	return OptionAlertType(s), nil
}
