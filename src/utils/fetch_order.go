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

type OptionProfit struct {
	Profit    float64
	IsInMoney bool
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
			return (underlyingPriceAtExpiry - option.StrikePrice) * optionMultiplier, nil
		} else {
			return 0, nil
		}
	} else if option.OptionType == "P" {
		if underlyingPriceAtExpiry < option.StrikePrice {
			return (option.StrikePrice - underlyingPriceAtExpiry) * optionMultiplier, nil
		} else {
			return 0, nil
		}
	} else {
		return 0, errors.New("invalid option type")
	}
}

func calculateSpreadProfitAtExpiry(option1 Option, side1 string, option2 Option, side2 string, underlyingClosePrcAtExpiry float64, optionMultiplier float64) (OptionProfit, OptionProfit, error) {
	profit1, err := calculateOptionProfitAtExpiry(option1, underlyingClosePrcAtExpiry, optionMultiplier)
	if err != nil {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("failed to calculate option1 profit: %w", err)
	}

	var optionProfit1 OptionProfit
	if profit1 > 0 {
		optionProfit1.IsInMoney = true
	}

	if side1 == "sell_to_open" {
		profit1 *= -1
	} else if side1 == "buy_to_open" {
	} else {
		return OptionProfit{}, OptionProfit{}, errors.New("invalid side for option1")
	}

	optionProfit1.Profit = profit1

	profit2, err := calculateOptionProfitAtExpiry(option2, underlyingClosePrcAtExpiry, optionMultiplier)
	if err != nil {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("failed to calculate option2 profit: %w", err)
	}

	var optionProfit2 OptionProfit
	if profit2 > 0 {
		optionProfit2.IsInMoney = true
	}

	if side2 == "sell_to_open" {
		profit2 *= -1
	} else if side2 == "buy_to_open" {
	} else {
		return OptionProfit{}, OptionProfit{}, errors.New("invalid side for option2")
	}

	optionProfit2.Profit = profit2

	return optionProfit1, optionProfit2, nil
}

func CalculateOptionOrderSpreadResult(order *eventmodels.TradierOrder, underlyingDailyCandles []eventmodels.TradierCandleDTO, symbol1Data []eventmodels.OratsOptionData, symbol2Data []eventmodels.OratsOptionData) (*eventmodels.OptionOrderSpreadResult, error) {
	optionMultiplier := 100.0

	if order == nil {
		return nil, errors.New("order cannot be nil")
	}

	if len(underlyingDailyCandles) == 0 {
		return nil, errors.New("underlyingCandles cannot be empty")
	}

	if len(symbol1Data) == 0 {
		return nil, errors.New("symbol1Data cannot be empty")
	}

	if len(symbol2Data) == 0 {
		return nil, errors.New("symbol2Data cannot be empty")
	}

	if order.Strategy != "spread" {
		return nil, errors.New("order strategy must be spread")
	}

	if len(order.Leg) != 2 {
		return nil, errors.New("order must have exactly 2 legs")
	}

	signalName, expectedProfit, requestedPrice, err := DecodeTag(order.Tag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tag: %w", err)
	}

	requestedPrice *= -1
	slippage := requestedPrice - order.AvgFillPrice

	option1, err := ParseOptionTicker(order.Leg[0].OptionSymbol)
	side1 := order.Leg[0].Side
	if err != nil {
		return nil, fmt.Errorf("failed to parse option1 ticker: %w", err)
	}

	var option1Type eventmodels.OptionType
	if option1.OptionType == "C" {
		option1Type = eventmodels.OptionTypeCall
	} else if option1.OptionType == "P" {
		option1Type = eventmodels.OptionTypePut
	} else {
		return nil, errors.New("invalid option1 type")
	}

	option2, err := ParseOptionTicker(order.Leg[1].OptionSymbol)
	side2 := order.Leg[1].Side
	if err != nil {
		return nil, fmt.Errorf("failed to parse option2 ticker: %w", err)
	}

	var option2Type eventmodels.OptionType
	if option2.OptionType == "C" {
		option2Type = eventmodels.OptionTypeCall
	} else if option2.OptionType == "P" {
		option2Type = eventmodels.OptionTypePut
	} else {
		return nil, errors.New("invalid option2 type")
	}

	now := time.Now()

	isOption1Expired := isOptionExpired(*option1, now)
	if isOption1Expired != isOptionExpired(*option2, now) {
		return nil, errors.New("both options must have the same expiration status")
	}

	expirationDate, err := eventmodels.ConvertToMarketClose(option1.Expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to convert expiration to market close: %w", err)
	}

	var debitPaid, creditReceived float64
	if order.AvgFillPrice > 0 {
		debitPaid = order.AvgFillPrice * optionMultiplier
	} else {
		creditReceived = -order.AvgFillPrice * optionMultiplier
	}

	result := eventmodels.OptionOrderSpreadResult{
		OrderID:          order.ID,
		Underlying:       order.Symbol,
		ExecutionType:    order.Type,
		Strategy:         order.Strategy,
		CreatedTimestamp: order.CreateDate,
		DebitPaid:        debitPaid,
		CreditReceived:   creditReceived,
		OrderID1:         order.Leg[1].ID,
		Side1:            order.Leg[1].Side,
		OptionType1:      option2Type,
		Symbol1:          order.Leg[1].OptionSymbol,
		StrikePrice1:     option2.StrikePrice,
		Quantity1:        order.Leg[1].Quantity,
		AvgFillPrice1:    order.Leg[1].AvgFillPrice,
		OrderID2:         order.Leg[0].ID,
		Side2:            order.Leg[0].Side,
		OptionType2:      option1Type,
		Symbol2:          order.Leg[0].OptionSymbol,
		Quantity2:        order.Leg[0].Quantity,
		StrikePrice2:     option1.StrikePrice,
		AvgFillPrice2:    order.Leg[0].AvgFillPrice,
		SignalName:       string(signalName),
		ExpectedProfit:   expectedProfit * optionMultiplier,
		RequestedPrice:   requestedPrice,
		IsClosed:         isOption1Expired,
		ExpirationDate:   expirationDate,
		ExecutedPrice:    order.AvgFillPrice,
		Slippage:         slippage,
	}

	if isOption1Expired {
		symbol1DataAtExpiry, err := findTradierCandleDTOAt(option1.Expiration, underlyingDailyCandles)
		if err != nil {
			return nil, fmt.Errorf("failed to find symbol1 data at expiry: %w", err)
		}

		optionMultiplier := 100.0
		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(*option2, side2, *option1, side1, symbol1DataAtExpiry.Close, optionMultiplier)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate spread profit at expiry: %w", err)
		}

		creditReceivedAtOpen := -order.AvgFillPrice * 100

		result.PriceAtExpiry = symbol1DataAtExpiry.Close
		result.InTheMoney1 = optionProfit1.IsInMoney
		result.Profit1 = optionProfit1.Profit
		result.InTheMoney2 = optionProfit2.IsInMoney
		result.Profit2 = optionProfit2.Profit
		result.Profit = creditReceivedAtOpen + optionProfit1.Profit + optionProfit2.Profit
	}

	return &result, nil
}
