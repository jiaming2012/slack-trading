package models

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type Condition struct {
	EntrySignal SignalV2 `json:"entry"`
	ExitSignal  SignalV2 `json:"exit"`
}

func (c *Condition) UpdateState(isEntry bool) {
	if isEntry {
		c.EntrySignal.IsSatisfied = true
		c.ExitSignal.IsSatisfied = false

		log.Infof("entry condition %v was met", c.EntrySignal.Name)
	} else {
		c.EntrySignal.IsSatisfied = false
		c.ExitSignal.IsSatisfied = true

		log.Infof("exit condition %v was met", c.ExitSignal.Name)
	}
}

func (c *Condition) String() string {
	return fmt.Sprintf("Entry: %v | Exit: %v", c.EntrySignal, c.ExitSignal)
}
