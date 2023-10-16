package models

import (
	"fmt"
	"time"
)

type SignalV2DTO struct {
	Name        string    `json:"name"`
	IsSatisfied bool      `json:"isSatisfied"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type SignalV2 struct {
	Name        string `json:"name"`
	isSatisfied bool
	lastUpdated time.Time
}

type ResetSignal struct {
	Name           string    `json:"name"`
	AffectedSignal *SignalV2 `json:"-"`
	LastUpdated    time.Time `json:"lastUpdated"`
}

func NewResetSignal(name string, affectedSignal *SignalV2, lastUpdated time.Time) *ResetSignal {
	return &ResetSignal{Name: name, AffectedSignal: affectedSignal, LastUpdated: lastUpdated}
}

func (s *ResetSignal) Update(timestamp time.Time) {
	if s.AffectedSignal.isSatisfied {
		s.AffectedSignal.Update(false, timestamp)
	}

	s.LastUpdated = timestamp
}

func (s *SignalV2) ConvertToDTO() *SignalV2DTO {
	return &SignalV2DTO{
		Name:        s.Name,
		IsSatisfied: s.IsSatisfied(),
		LastUpdated: s.lastUpdated,
	}
}

func (s *SignalV2) IsSatisfied() bool {
	return s.isSatisfied
}

func (s *SignalV2) Update(isSatisfied bool, timestamp time.Time) {
	s.isSatisfied = isSatisfied
	s.lastUpdated = timestamp
}

func NewSignalV2(name string, lastUpdated time.Time) *SignalV2 {
	return &SignalV2{Name: name, isSatisfied: false, lastUpdated: lastUpdated}
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
