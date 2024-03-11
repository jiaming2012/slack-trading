package eventmodels

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type EntryCondition struct {
	EntrySignal *SignalV2
	ResetSignal *ResetSignal
}

func (c *EntryCondition) ConvertToDTO() *EntryConditionDTO {
	return &EntryConditionDTO{
		EntrySignal: c.EntrySignal.ConvertToDTO(),
		ResetSignal: c.ResetSignal,
	}
}

func (c *EntryCondition) UpdateState(isEntry bool, timestamp time.Time) {
	if isEntry {
		c.EntrySignal.Update(true, timestamp)
		log.Infof("entry condition %v was met", c.EntrySignal.Name)
	} else {
		c.ResetSignal.Update(timestamp)
		log.Infof("exit condition %v was met", c.ResetSignal.Name)
	}
}

func (c *EntryCondition) String() string {
	return fmt.Sprintf("Entry: %v | Exit: %v", c.EntrySignal, c.ResetSignal)
}
