package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Option struct to hold parsed option details
type Option struct {
	Underlying  string
	Expiration  time.Time
	OptionType  string
	StrikePrice float64
}

// ParseOptionTicker function to parse the option ticker
func ParseOptionTicker(ticker string) (*Option, error) {
	// Regular expression to match the option ticker format
	re := regexp.MustCompile(`^([A-Z]+)(\d{2})(\d{2})(\d{2})([CP])(\d{8})$`)
	matches := re.FindStringSubmatch(ticker)
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

	// Create and return the Option struct
	option := &Option{
		Underlying:  underlying,
		Expiration:  expiration,
		OptionType:  optionType,
		StrikePrice: strikePrice / 1000,
	}
	return option, nil
}

func main() {
	// Example option ticker
	ticker := "GS250117C00280000"

	// Parse the option ticker
	option, err := ParseOptionTicker(ticker)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the parsed details
	fmt.Printf("Underlying: %s\n", option.Underlying)
	fmt.Printf("Expiration Date: %s\n", option.Expiration.Format("2006-01-02"))
	fmt.Printf("Option Type: %s\n", option.OptionType)
	fmt.Printf("Strike Price: %.2f\n", option.StrikePrice)
}
