package models

import "fmt"

type Condition struct {
	Signal      Signal
	IsSatisfied bool
}

func (c Condition) String() string {
	var isSatisfied string
	if c.IsSatisfied {
		isSatisfied = "satisfied"
	} else {
		isSatisfied = "not satisfied"
	}

	return fmt.Sprintf("%v (%v)", c.Signal, isSatisfied)
}
