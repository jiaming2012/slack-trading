package models

import "fmt"

type SignalConstraints []*ExitSignalConstraint

func (constraints SignalConstraints) Validate() error {
	names := make(map[string]struct{})
	for _, c := range constraints {
		if _, exists := names[c.Name]; exists {
			return fmt.Errorf("SignalConstraints.Validate: duplicate name not allowed, found %v twice", c.Name)
		}
		names[c.Name] = struct{}{}
	}

	return nil
}

type ExitSignalConstraint struct {
	Name  string                                 `json:"name"`
	Check func(*PriceLevel, *ExitCondition) bool `json:"-"`
}

func NewExitSignalConstraint(name string, check func(level *PriceLevel, condition *ExitCondition) bool) *ExitSignalConstraint {
	return &ExitSignalConstraint{Name: name, Check: check}
}

func PriceLevelProfitLossAboveZeroConstraint(priceLevel *PriceLevel, exitCondition *ExitCondition) bool {

	return false
}
