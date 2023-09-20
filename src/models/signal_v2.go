package models

import "fmt"

type SignalV2 struct {
	Name        string `json:"name"`
	IsSatisfied bool   `json:"is_satisfied"`
}

func NewSignalV2(name string) *SignalV2 {
	return &SignalV2{Name: name, IsSatisfied: false}
}

func (s SignalV2) String() string {
	var isSatisfied string

	if s.IsSatisfied {
		isSatisfied = "satisfied"
	} else {
		isSatisfied = "not satisfied"
	}

	return fmt.Sprintf("%v (%v)", s.Name, isSatisfied)
}
