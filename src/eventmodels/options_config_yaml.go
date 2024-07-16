package eventmodels

import (
	"fmt"
	"strings"
)

type OptionsConfigYAML struct {
	Options []OptionYAML `yaml:"options"`
}

func (o *OptionsConfigYAML) GetOption(symbol StockSymbol) (*OptionYAML, error) {
	sym1 := strings.ToLower(string(symbol))
	for _, option := range o.Options {
		sym2 := strings.ToLower(option.Symbol)
		if sym1 == sym2 {
			return &option, nil
		}
	}

	return nil, fmt.Errorf("OptionsConfigYAML: option not found")
}
