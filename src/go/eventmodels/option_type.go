package eventmodels

import "fmt"

type OptionType string

func (o OptionType) Validate() error {
	if o != OptionTypeCall && o != OptionTypePut && o != OptionTypeCallSpread && o != OptionTypePutSpread {
		return fmt.Errorf("OptionType: Validate: invalid option type: %s", o)
	}

	return nil
}

const (
	OptionTypeCall       OptionType = "call"
	OptionTypePut        OptionType = "put"
	OptionTypeCallSpread OptionType = "vertical_call"
	OptionTypePutSpread  OptionType = "vertical_put"
)
