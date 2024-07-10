package eventmodels

import "time"

// OptionSymbolComponents struct to hold parsed option details
type OptionSymbolComponents struct {
	Underlying  string
	Expiration  time.Time
	OptionType  string
	StrikePrice float64
	Symbol      OptionSymbol
}
