package eventmodels

type EntryConditionDTO struct {
	EntrySignal *SignalV2DTO `json:"entrySignal"`
	ResetSignal *ResetSignal `json:"resetSignal"`
}

func (c *EntryConditionDTO) ToEntryCondition() *EntryCondition {
	entrySignal := c.EntrySignal.ToSignalV2()
	resetSignal := c.ResetSignal

	resetSignal.AffectedSignal = entrySignal

	return &EntryCondition{
		EntrySignal: entrySignal,
		ResetSignal: resetSignal,
	}
}
