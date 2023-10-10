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
		c.EntrySignal.Update(true)
		c.ResetSignal.Update(false)

		log.Infof("entry condition %v was met", c.EntrySignal.Name)
	} else {
		c.EntrySignal.Update(false)
		c.ResetSignal.Update(true)

		log.Infof("exit condition %v was met", c.ResetSignal.Name)
	}
}

func (c *EntryCondition) String() string {
	return fmt.Sprintf("Entry: %v | Exit: %v", c.EntrySignal, c.ResetSignal)
}

type ExitCondition struct {
	Name                   string            `json:"name"`
	ExitSignals            []*ExitSignal     `json:"exitSignals"`
	ReentrySignals         []*SignalV2       `json:"reentrySignals"`
	Constraints            SignalConstraints `json:"constraints"`
	LevelIndex             int               `json:"levelIndex"`
	MaxTriggerCount        *int              `json:"maxTriggerCount"`
	TriggerCount           int               `json:"triggerCount"`
	ClosePercent           ClosePercent      `json:"closePercent"`
	AwaitingReentrySignals bool              `json:"awaitingReentrySignals"`
}

func (c *ExitCondition) ResetReentrySignals() {
	for _, s := range c.ReentrySignals {
		s.Update(false)
	}
}

func NewExitCondition(name string, levelIndex int, exitSignals []*ExitSignal, resetSignals []*SignalV2, constraints []*ExitSignalConstraint, closePercent ClosePercent, maxTriggerCount *int) (*ExitCondition, error) {
	if levelIndex < 0 {
		return nil, fmt.Errorf("NewExitCondition: LevelIndex must be >= 0")
	}

	if err := closePercent.Validate(); err != nil {
		return nil, fmt.Errorf("NewExitCondition: failed to validate close percent: %w", err)
	}

	condition := &ExitCondition{
		Name:            name,
		ExitSignals:     exitSignals,
		ReentrySignals:  resetSignals,
		Constraints:     constraints,
		LevelIndex:      levelIndex,
		ClosePercent:    closePercent,
		MaxTriggerCount: maxTriggerCount,
		TriggerCount:    0,
	}

	if err := condition.Validate(); err != nil {
		return nil, fmt.Errorf("NewExitCondition: condition validation failed: %w", err)
	}

	return condition, nil
}

func (c *ExitCondition) IsSatisfied(priceLevel *PriceLevel, params map[string]interface{}) (bool, error) {
	if len(c.ExitSignals) == 0 {
		log.Infof("ExitCondition.isSatisfied: false due to no exit signals set")
		return false, nil
	}

	if c.MaxTriggerCount != nil && c.TriggerCount >= *c.MaxTriggerCount {
		log.Infof("ExitCondition.isSatisfied: false due to triggerCount(%v) >= maxTriggerCount(%v)", c.TriggerCount, *c.MaxTriggerCount)
		return false, nil
	}

	if c.AwaitingReentrySignals {
		if len(c.ReentrySignals) == 0 {
			log.Warnf("ExitCondition.isSatisfied: awaiting reset will always be true: no reset signals set")
		} else {
			resetSignalsAllSatisfied := true
			for _, signal := range c.ReentrySignals {
				if !signal.IsSatisfied() {
					resetSignalsAllSatisfied = false
					break
				}
			}

			if resetSignalsAllSatisfied {
				log.Infof("ExitCondition.isSatisfied: switching awaiting reset from true -> false for %v", c.Name)
				c.AwaitingReentrySignals = false
				c.ResetReentrySignals()
			}
		}
	}

	if c.AwaitingReentrySignals {
		log.Infof("ExitCondition.isSatisfied: false due to awaiting reset")
		return false, nil
	}

	for _, signal := range c.ExitSignals {
		if !signal.Signal.IsSatisfied() {
			return false, nil
		}
	}

	for _, constraint := range c.Constraints {
		check, err := constraint.Check(priceLevel, c, params)
		if err != nil {
			return false, fmt.Errorf("contraint check failed: %w", err)
		}
		if !check {
			log.Infof("ExitCondition.isSatisfied: false due to failed constraint check, %v", constraint.Name)
			return false, nil
		}
	}

	// todo: condition should update from an event that notifies that a trade was opened
	// currently this creates a bug if the signal fires, but the execution of the trade fails, e.g.
	// because the broker is offline
	c.TriggerCount += 1
	c.AwaitingReentrySignals = true
	log.Infof("ExitCondition.isSatisfied: %v exit condition satisfied", c.Name)

	return true, nil
}

func (c *ExitCondition) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("ExitCondition.Validate: name is not set")
	}

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
