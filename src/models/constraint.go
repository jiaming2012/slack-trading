package models

import "fmt"

type SignalConstraints []*SignalConstraint

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

type SignalConstraint struct {
	Name  string
	Check func(s *Strategy) bool
}

func NewSignalConstraint(name string, check func(s *Strategy) bool) *SignalConstraint {
	return &SignalConstraint{Name: name, Check: check}
}
