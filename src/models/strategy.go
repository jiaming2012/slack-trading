package models

import (
	"fmt"
	"strings"
)

type Strategy struct {
	Name       string
	Conditions []Condition
}

func (s Strategy) String() string {
	str := strings.Builder{}

	str.WriteString(fmt.Sprintf("%v\n", s.Name))

	for _, signal := range s.Conditions {
		str.WriteString(signal.String() + "\n")
	}

	return str.String()
}

func (s *Strategy) isConditionUnique(signal Signal) bool {
	for _, cond := range s.Conditions {
		if cond.Signal.String() == signal.String() {
			return false
		}
	}

	return true
}

func (s *Strategy) RemoveCondition(signal Signal) error {
	for i, cond := range s.Conditions {
		if cond.Signal.String() == signal.String() {
			s.Conditions = append(s.Conditions[:i], s.Conditions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("Strategy.RemoveCondition: could not find signal %v", signal)
}

func (s *Strategy) AddCondition(signal Signal) error {
	if !s.isConditionUnique(signal) {
		return fmt.Errorf("signal %v already exists", signal)
	}

	s.Conditions = append(s.Conditions, Condition{
		Signal:      signal,
		IsSatisfied: false,
	})

	return nil
}

func NewStrategy(name string) *Strategy {
	return &Strategy{
		Name:       name,
		Conditions: make([]Condition, 0),
	}
}
