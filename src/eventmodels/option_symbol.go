package eventmodels

import (
	"fmt"
	"strings"
)

type OptionSymbol string

func (s OptionSymbol) NoPrefix() string {
	if strings.HasPrefix(string(s), "O:") {
		return string(s)[2:]
	}

	return string(s)
}

func (s OptionSymbol) Description() (string, error) {
	components, err := NewOptionSymbolComponents(s)
	if err != nil {
		return "", fmt.Errorf("OptionSymbol.Description: failed to parse option symbol: %w", err)
	}

	// Format the expiration date
	expiration := components.Expiration.Format("Jan 2 2006")

	// Format the strike price
	strikePrice := fmt.Sprintf("%.2f", components.StrikePrice)

	// Format the option type
	optionType := "Call"
	if components.OptionType == "P" {
		optionType = "Put"
	}

	// Construct the human-readable format
	formatted := fmt.Sprintf("%s %s $%s %s", components.Underlying, expiration, strikePrice, optionType)

	return formatted, nil
}

func NewOptionSymbol(option OptionSymbolComponents) (OptionSymbol, error) {
	// Validate the option type
	if option.OptionType != "C" && option.OptionType != "P" {
		return "", fmt.Errorf("invalid option type: %s", option.OptionType)
	}

	// Format the expiration date components
	year := option.Expiration.Year() % 100 // last two digits of the year
	month := int(option.Expiration.Month())
	day := option.Expiration.Day()

	// Format the strike price to 8 digits
	strikePrice := fmt.Sprintf("%08d", int(option.StrikePrice*1000))

	// Construct the option ticker
	ticker := fmt.Sprintf("%s%02d%02d%02d%s%s",
		option.Underlying, year, month, day, option.OptionType, strikePrice)

	return OptionSymbol(ticker), nil
}
