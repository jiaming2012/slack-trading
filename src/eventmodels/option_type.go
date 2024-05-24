package eventmodels

import "fmt"

type OptionType string

func (o OptionType) Validate() error {
	if o != Call && o != Put {
		return fmt.Errorf("OptionType: Validate: invalid option type: %s", o)
	}

	return nil
}

const (
	Call OptionType = "call"
	Put  OptionType = "put"
)
