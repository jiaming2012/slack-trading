package models

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type EntryCondition struct {
	EntrySignal *SignalV2 `json:"entrySignal"`
	ResetSignal *SignalV2 `json:"resetSignal"`
}

func (c *EntryCondition) UpdateState(isEntry bool) {
	if isEntry {
		c.EntrySignal.IsSatisfied = true
		c.ResetSignal.IsSatisfied = false

		log.Infof("entry condition %v was met", c.EntrySignal.Name)
	} else {
		c.EntrySignal.IsSatisfied = false
		c.ResetSignal.IsSatisfied = true

		log.Infof("exit condition %v was met", c.ResetSignal.Name)
	}
}

func (c *EntryCondition) String() string {
	return fmt.Sprintf("Entry: %v | Exit: %v", c.EntrySignal, c.ResetSignal)
}

type ExitCondition struct {
	ExitSignals     []*SignalV2       `json:"exitSignals"`
	ResetSignals    []*SignalV2       `json:"resetSignals"`
	Constraints     SignalConstraints `json:"constraints"`
	LevelIndex      int               `json:"levelIndex"`
	MaxTriggerCount int               `json:"maxTriggerCount"`
	ClosePercent    ClosePercent      `json:"closePercent"`
}

func NewExitCondition(levelIndex int, signals []*SignalV2, constraints []*SignalConstraint, closePercent ClosePercent) (*ExitCondition, error) {
	condition := &ExitCondition{ExitSignals: signals, Constraints: constraints, LevelIndex: levelIndex, ClosePercent: closePercent}

	if err := condition.Validate(); err != nil {
		return nil, fmt.Errorf("NewExitCondition: condition validation failed: %w", err)
	}

	if levelIndex < 0 {
		return nil, fmt.Errorf("NewExitCondition: LevelIndex must be >= 0")
	}

	if err := closePercent.Validate(); err != nil {
		return nil, fmt.Errorf("NewExitCondition: failed to validate close percent: %w", err)
	}

	return condition, nil
}

func (c *ExitCondition) IsSatisfied(priceLevel *PriceLevel) bool {
	if len(c.ExitSignals) == 0 {
		return false
	}

	for _, signal := range c.ExitSignals {
		if !signal.IsSatisfied {
			return false
		}
	}

	for _, constraint := range c.Constraints {
		if !constraint.Check(priceLevel, c) {
			return false
		}
	}

	return true
}

func (c *ExitCondition) Validate() error {
	if c.LevelIndex < 0 {
		return fmt.Errorf("ExitCondition.Validate: LevelIndex must be >= 0, found %v", c.LevelIndex)
	}

	if c.ClosePercent <= 0 || c.ClosePercent > 1 {
		return fmt.Errorf("ExitCondition.Validate: ClosePercent must be > 0 and <= 1, found %v", c.ClosePercent)
	}

	return nil
}
