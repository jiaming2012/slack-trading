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
	MaxTriggerCount *int              `json:"maxTriggerCount"`
	TriggerCount    int               `json:"triggerCount"`
	ClosePercent    ClosePercent      `json:"closePercent"`
	AwaitingReset   bool              `json:"awaitingReset"`
}

func NewExitCondition(levelIndex int, signals []*SignalV2, resetSignals []*SignalV2, constraints []*ExitSignalConstraint, closePercent ClosePercent, maxTriggerCount *int) (*ExitCondition, error) {
	condition := &ExitCondition{
		ExitSignals:     signals,
		ResetSignals:    resetSignals,
		Constraints:     constraints,
		LevelIndex:      levelIndex,
		ClosePercent:    closePercent,
		MaxTriggerCount: maxTriggerCount,
		TriggerCount:    0,
	}

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

func (c *ExitCondition) IsSatisfied(priceLevel *PriceLevel, params map[string]interface{}) (bool, error) {
	if len(c.ExitSignals) == 0 {
		log.Infof("ExitCondition.IsSatisfied: false due to no exit signals set")
		return false, nil
	}

	if c.MaxTriggerCount != nil && c.TriggerCount >= *c.MaxTriggerCount {
		log.Infof("ExitCondition.IsSatisfied: false due to triggerCount(%v) >= maxTriggerCount(%v)", c.TriggerCount, *c.MaxTriggerCount)
		return false, nil
	}

	for _, signal := range c.ExitSignals {
		if !signal.IsSatisfied {
			c.AwaitingReset = false
			return false, nil
		}
	}

	for _, constraint := range c.Constraints {
		check, err := constraint.Check(priceLevel, c, params)
		if err != nil {
			return false, fmt.Errorf("contraint check failed: %w", err)
		}
		if !check {
			log.Infof("ExitCondition.IsSatisfied: false due to failed constraint check, %v", constraint.Name)
			return false, nil
		}
	}

	if c.AwaitingReset {
		log.Infof("ExitCondition.IsSatisfied: false due to awaiting reset")
		return false, nil
	}

	c.TriggerCount += 1
	c.AwaitingReset = true

	return true, nil
}

func (c *ExitCondition) Validate() error {
	if c.LevelIndex < 0 {
		return fmt.Errorf("ExitCondition.Validate: LevelIndex must be >= 0, found %v", c.LevelIndex)
	}

	if c.ClosePercent <= 0 || c.ClosePercent > 1 {
		return fmt.Errorf("ExitCondition.Validate: ClosePercent must be > 0 and <= 1, found %v", c.ClosePercent)
	}

	if c.TriggerCount < 0 {
		return fmt.Errorf("ExitCondition.Validate: TriggerCount must be >= 0, found %v", c.TriggerCount)
	}

	return nil
}
