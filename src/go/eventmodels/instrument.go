package eventmodels

import "fmt"

type Instrument interface {
	GetTicker() string
}

// todo: refactor eventmodels and models to make class OrderRecordClass
func NewInstrument(class string, symbol string) (Instrument, error) {
	switch class {
	case "equity":
		return NewStockSymbol(symbol), nil
	case "option":
		s, err := OptionSymbol(symbol).ConvertToOptionContractV3()
		if err != nil {
			return nil, fmt.Errorf("failed to convert option symbol: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown instrument class: %s", class)
	}
}
