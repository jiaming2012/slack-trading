package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
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

func findOratsOptionDataAt(timestamp time.Time, data []eventmodels.OratsOptionData) (eventmodels.OratsOptionData, error) {
	for _, d := range data {
		if d.TradeDate == timestamp.Format("2006-01-02") {
			return d, nil
		}
	}

	return eventmodels.OratsOptionData{}, errors.New("no matching data found")
}

func findTradierCandleDTOAt(timestamp time.Time, data []eventmodels.TradierCandleDTO) (eventmodels.TradierCandleDTO, error) {
	for _, d := range data {
		if d.Date == timestamp.Format("2006-01-02") {
			return d, nil
		}
	}

	return eventmodels.TradierCandleDTO{}, errors.New("no matching data found")
}

func isOptionExpired(option Option, now time.Time) bool {
	return option.Expiration.Before(now)
}

func calcOptionSpreadCostBasis(spread eventmodels.TradierOrder) float64 {
	option1Cost := spread.Leg[0].AvgFillPrice * spread.Leg[0].ExecQuantity
	if spread.Leg[0].Type == "sell_to_open" {
		option1Cost = -option1Cost
	}

	option2Cost := spread.Leg[1].AvgFillPrice * spread.Leg[1].ExecQuantity
	if spread.Leg[1].Type == "sell_to_open" {
		option2Cost = -option2Cost
	}

	return option1Cost + option2Cost
}

func calculateOptionProfitAtExpiry(option Option, underlyingPriceAtExpiry float64, optionMultiplier float64) (float64, error) {
	if option.OptionType == "C" {
		if underlyingPriceAtExpiry > option.StrikePrice {
			return (underlyingPriceAtExpiry - option.StrikePrice) * 100, nil
		} else {
			return 0, nil
		}
	} else if option.OptionType == "P" {
		if underlyingPriceAtExpiry < option.StrikePrice {
			return (option.StrikePrice - underlyingPriceAtExpiry) * 100, nil
		} else {
			return 0, nil
		}
	} else {
		return 0, errors.New("invalid option type")
	}
}

func calculateSpreadProfitAtExpiry(option1 Option, side1 string, option2 Option, side2 string, underlyingClosePrcAtExpiry float64) (float64, error) {
	profit := 0.0

	profit1, err := calculateOptionProfitAtExpiry(option1, underlyingClosePrcAtExpiry, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate option1 profit: %w", err)
	}

	if side1 == "sell_to_open" {
		profit -= profit1
	} else if side1 == "buy_to_open" {
		profit += profit1
	} else {
		return 0, errors.New("invalid side for option1")
	}

	profit2, err := calculateOptionProfitAtExpiry(option2, underlyingClosePrcAtExpiry, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate option2 profit: %w", err)
	}

	if side2 == "sell_to_open" {
		profit -= profit2
	} else if side2 == "buy_to_open" {
		profit += profit2
	} else {
		return 0, errors.New("invalid side for option2")
	}

	return profit, nil
}

func CalculateOptionOrderSpreadResult(order *eventmodels.TradierOrder, underlyingDailyCandles []eventmodels.TradierCandleDTO, symbol1Data []eventmodels.OratsOptionData, symbol2Data []eventmodels.OratsOptionData) (eventmodels.OptionOrderSpreadResult, error) {
	if order == nil {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("order cannot be nil")
	}

	if len(underlyingDailyCandles) == 0 {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("underlyingCandles cannot be empty")
	}

	if len(symbol1Data) == 0 {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("symbol1Data cannot be empty")
	}

	if len(symbol2Data) == 0 {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("symbol2Data cannot be empty")
	}

	if order.Strategy != "spread" {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("order strategy must be spread")
	}

	if len(order.Leg) != 2 {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("order must have exactly 2 legs")
	}

	signalName, expectedProfit, requestedPrice, err := DecodeTag(order.Tag)
	if err != nil {
		return eventmodels.OptionOrderSpreadResult{}, fmt.Errorf("failed to decode tag: %w", err)
	}

	option1, err := ParseOptionTicker(order.Leg[0].OptionSymbol)
	side1 := order.Leg[0].Side
	if err != nil {
		return eventmodels.OptionOrderSpreadResult{}, fmt.Errorf("failed to parse option1 ticker: %w", err)
	}

	option2, err := ParseOptionTicker(order.Leg[1].OptionSymbol)
	side2 := order.Leg[1].Side
	if err != nil {
		return eventmodels.OptionOrderSpreadResult{}, fmt.Errorf("failed to parse option2 ticker: %w", err)
	}

	now := time.Now()

	isOption1Expired := isOptionExpired(*option1, now)
	if isOption1Expired != isOptionExpired(*option2, now) {
		return eventmodels.OptionOrderSpreadResult{}, errors.New("both options must have the same expiration status")
	}

	result := eventmodels.OptionOrderSpreadResult{
		Underlying:       order.Symbol,
		ExecutionType:    order.Type,
		Strategy:         order.Strategy,
		CreatedTimestamp: order.CreateDate,
		OrderID1:         order.Leg[0].ID,
		Symbol1:          order.Leg[0].Symbol,
		Type1:            order.Leg[0].Type,
		Quantity1:        order.Leg[0].Quantity,
		AvgFillPrice1:    order.Leg[0].AvgFillPrice,
		OrderID2:         order.Leg[1].ID,
		Symbol2:          order.Leg[1].Symbol,
		Type2:            order.Leg[1].Type,
		Quantity2:        order.Leg[1].Quantity,
		AvgFillPrice2:    order.Leg[1].AvgFillPrice,
		SignalName:       string(signalName),
		ExpectedProfit:   expectedProfit,
		RequestedPrice:   requestedPrice,
		IsClosed:         isOption1Expired,
		ExpirationDate:   option1.Expiration,
		ExecutedPrice:    order.AvgFillPrice,
	}

	if isOption1Expired {
		symbol1DataAtExpiry, err := findTradierCandleDTOAt(option1.Expiration, underlyingDailyCandles)
		if err != nil {
			return eventmodels.OptionOrderSpreadResult{}, fmt.Errorf("failed to find symbol1 data at expiry: %w", err)
		}

		profit, err := calculateSpreadProfitAtExpiry(*option1, side1, *option2, side2, symbol1DataAtExpiry.Close)
		if err != nil {
			return eventmodels.OptionOrderSpreadResult{}, fmt.Errorf("failed to calculate spread profit at expiry: %w", err)
		}

		creditReceivedAtOpen := -order.AvgFillPrice * 100
		
		result.PriceAtExpiry = symbol1DataAtExpiry.Close
		result.Profit = creditReceivedAtOpen + profit
	}

	return result, nil
}
