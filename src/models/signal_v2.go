package models

import (
	"fmt"
	"time"
)

type SignalV2 struct {
	Name        string    `json:"name"`
	isSatisfied bool      `json:"is_satisfied"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func (s *SignalV2) IsSatisfied() bool {
	return s.isSatisfied
}

func (s *SignalV2) Update(isSatisfied bool) {
	s.isSatisfied = isSatisfied
	s.LastUpdated = time.Now()
}

func NewSignalV2(name string, lastUpdated time.Time) *SignalV2 {
	return &SignalV2{Name: name, isSatisfied: false, LastUpdated: lastUpdated}
}

func (s *SignalV2) String() string {
	var isSatisfied string

	if s.isSatisfied {
		isSatisfied = "satisfied"
	} else {
		isSatisfied = "not satisfied"
	}

	return fmt.Sprintf("%v (%v)", s.Name, isSatisfied)
}
