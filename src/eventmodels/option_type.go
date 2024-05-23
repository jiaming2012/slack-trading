package eventmodels

import "fmt"

type OptionType string

func (o OptionType) Validate() error {
	if o != Calls && o != Puts {
		return fmt.Errorf("OptionType: Validate: invalid option type: %s", o)
	}

	return nil
}

const (
	Calls OptionType = "calls"
	Puts  OptionType = "puts"
)
