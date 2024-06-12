package eventmodels

type ExitConditionDTO struct {
	Name                   string            `json:"name"`
	ExitSignals            []*ExitSignalDTO  `json:"exitSignals"`
	ReentrySignals         []*SignalV2DTO    `json:"reentrySignals"`
	Constraints            SignalConstraints `json:"constraints"`
	LevelIndex             int               `json:"levelIndex"`
	MaxTriggerCount        *int              `json:"maxTriggerCount"`
	TriggerCount           int               `json:"triggerCount"`
	ClosePercent           ClosePercent      `json:"closePercent"`
	AwaitingReentrySignals bool              `json:"awaitingReentrySignals"`
}

func (c *ExitConditionDTO) ToExitCondition() *ExitCondition {
	var exitSignals []*ExitSignal
	for _, s := range c.ExitSignals {
		exitSignals = append(exitSignals, s.ToExitSignal())
	}

	var reentrySignals []*SignalV2
	for _, s := range c.ReentrySignals {
		reentrySignals = append(reentrySignals, s.ToSignalV2())
	}

	return &ExitCondition{
		Name:            c.Name,
		ExitSignals:     exitSignals,
		ReentrySignals:  reentrySignals,
		Constraints:     c.Constraints,
		LevelIndex:      c.LevelIndex,
		MaxTriggerCount: c.MaxTriggerCount,
		TriggerCount:    c.TriggerCount,
		ClosePercent:    c.ClosePercent,
	}
}

func (c *ExitCondition) ConvertToDTO() *ExitConditionDTO {
	var exitSignals []*ExitSignalDTO
	for _, s := range c.ExitSignals {
		exitSignals = append(exitSignals, s.ConvertToDTO())
	}

	var reentrySignals []*SignalV2DTO
	for _, s := range c.ReentrySignals {
		reentrySignals = append(reentrySignals, s.ConvertToDTO())
	}

	return &ExitConditionDTO{
		Name:                   c.Name,
		ExitSignals:            exitSignals,
		ReentrySignals:         reentrySignals,
		Constraints:            c.Constraints,
		LevelIndex:             c.LevelIndex,
		MaxTriggerCount:        c.MaxTriggerCount,
		TriggerCount:           c.TriggerCount,
		ClosePercent:           c.ClosePercent,
		AwaitingReentrySignals: c.AwaitingReentrySignals(),
	}
}
