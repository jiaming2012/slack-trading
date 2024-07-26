package eventmodels

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// OptionSymbolComponents struct to hold parsed option details
type OptionSymbolComponents struct {
	Underlying  string
	Expiration  time.Time
	OptionType  ThetaDataOptionType
	StrikePrice float64
	Symbol      OptionSymbol
}

// NewOptionSymbolComponents parses an option ticker into its components
func NewOptionSymbolComponents(ticker OptionSymbol) (*OptionSymbolComponents, error) {
	// Regular expression to match the option ticker format
	re := regexp.MustCompile(`^([A-Z]+)(\d{2})(\d{2})(\d{2})([CP])(\d+)$`)
	matches := re.FindStringSubmatch(string(ticker))
	if matches == nil {
		return nil, fmt.Errorf("invalid option ticker format, matching %s", ticker)
	}

	// Extract and parse the details
	underlying := matches[1]
	year, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid year in ticker: %s", matches[2])
	}
	month, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("invalid month in ticker: %s", matches[3])
	}
	day, err := strconv.Atoi(matches[4])
	if err != nil {
		return nil, fmt.Errorf("invalid day in ticker: %s", matches[4])
	}
	optionType := matches[5]
	strikePrice, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid strike price in ticker: %s", matches[6])
	}

	// Construct the expiration date
	expiration := time.Date(2000+year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	expiration, err = ConvertToMarketClose(expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to convert expiration to market close: %w", err)
	}

	return &OptionSymbolComponents{
		Underlying:  underlying,
		Expiration:  expiration,
		OptionType:  ThetaDataOptionType(optionType),
		StrikePrice: strikePrice / 1000,
		Symbol:      ticker,
	}, nil
}

// ParseOptionTicker function to parse the option ticker
func NewOptionSymbolComponentsOld(ticker OptionSymbol) (*OptionSymbolComponents, error) {
	// Regular expression to match the option ticker format
	re := regexp.MustCompile(`^([A-Z]+)(\d{2})(\d{2})(\d{2})([CP])(\d{8})$`)
	matches := re.FindStringSubmatch(string(ticker))
	if matches == nil {
		return nil, fmt.Errorf("invalid option ticker format")
	}

	// Extract and parse the details
	underlying := matches[1]
	year, _ := strconv.Atoi(matches[2])
	month, _ := strconv.Atoi(matches[3])
	day, _ := strconv.Atoi(matches[4])
	optionType := matches[5]
	strikePrice, _ := strconv.ParseFloat(matches[6], 64)

	// Construct the expiration date
	expiration := time.Date(2000+year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	expiration, err := ConvertToMarketClose(expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to convert expiration to market close: %w", err)
	}

	return &OptionSymbolComponents{
		Underlying:  underlying,
		Expiration:  expiration,
		OptionType:  ThetaDataOptionType(optionType),
		StrikePrice: strikePrice / 1000,
		Symbol:      OptionSymbol(ticker),
	}, nil
}
