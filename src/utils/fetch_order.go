package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type OptionProfit struct {
	Profit    float64
	IsInMoney bool
}

func findOratsOptionDataAt(timestamp time.Time, data []eventmodels.OratsOptionData) (eventmodels.OratsOptionData, error) {
	for _, d := range data {
		if d.TradeDate == timestamp.Format("2006-01-02") {
			return d, nil
		}
	}

	return eventmodels.OratsOptionData{}, errors.New("no matching data found")
}

func findCandleDTOAt(timestamp time.Time, data []*eventmodels.CandleDTO) (*eventmodels.CandleDTO, error) {
	var previousData *eventmodels.CandleDTO
	for _, d := range data {
		dateStamp, err := time.Parse("2006-01-02 15:04:00", d.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp %v: %w", d.Date, err)
		}

		if dateStamp.Equal(timestamp) {
			return d, nil
		}

		if dateStamp.After(timestamp) {
			if dateStamp.Sub(timestamp) > 5*time.Minute {
				log.Warnf("findCandleDTOAt: found a datestamp %v that is more than 5 minutes after the requested timestamp %v", dateStamp, timestamp)
			}

			return previousData, nil
		}

		previousData = d
	}

	return nil, errors.New("findCandleDTOAt: no matching data found")
}

func isOptionExpired(option eventmodels.OptionSymbolComponents, now time.Time) bool {
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

func calculateOptionProfitAtExpiry(option eventmodels.OptionSymbolComponents, side string, underlyingPriceAtExpiry float64, optionMultiplier float64) (float64, error) {
	if option.OptionType == "C" {
		if underlyingPriceAtExpiry > option.StrikePrice {
			profit := (underlyingPriceAtExpiry - option.StrikePrice) * optionMultiplier
			if side == "buy_to_open" {
				return profit, nil
			} else if side == "sell_to_open" {
				return -profit, nil
			} else {
				return 0, fmt.Errorf("calculateOptionProfitAtExpiry: invalid side %v", side)
			}

		} else {
			return 0, nil
		}
	} else if option.OptionType == "P" {
		if underlyingPriceAtExpiry < option.StrikePrice {
			profit := (option.StrikePrice - underlyingPriceAtExpiry) * optionMultiplier
			if side == "buy_to_open" {
				return profit, nil
			} else if side == "sell_to_open" {
				return -profit, nil
			} else {
				return 0, fmt.Errorf("calculateOptionProfitAtExpiry: invalid side %v", side)
			}

		} else {
			return 0, nil
		}
	} else {
		return 0, errors.New("invalid option type")
	}
}

func calculateSpreadProfitAtExpiry(option1 eventmodels.OptionSymbolComponents, side1 string, optionPremium float64, option2 eventmodels.OptionSymbolComponents, side2 string, optionPremium2 float64, underlyingClosePrcAtExpiry float64, optionMultiplier float64) (OptionProfit, OptionProfit, error) {
	profit1, err := calculateOptionProfitAtExpiry(option1, side1, underlyingClosePrcAtExpiry, optionMultiplier)
	if err != nil {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("calculateSpreadProfitAtExpiry: failed to calculate option1 profit: %w", err)
	}

	if side1 == "sell_to_open" {
		profit1 += optionPremium * optionMultiplier
	} else if side1 == "buy_to_open" {
		profit1 -= optionPremium * optionMultiplier
	} else {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("calculateSpreadProfitAtExpiry: invalid side1 %v", side1)
	}

	var optionProfit1 OptionProfit
	if option1.OptionType == eventmodels.ThetaDataOptionTypeCall {
		if underlyingClosePrcAtExpiry > option1.StrikePrice {
			optionProfit1.IsInMoney = true
		} else {
			optionProfit1.IsInMoney = false
		}
	} else if option1.OptionType == eventmodels.ThetaDataOptionTypePut {
		if underlyingClosePrcAtExpiry < option1.StrikePrice {
			optionProfit1.IsInMoney = true
		} else {
			optionProfit1.IsInMoney = false
		}
	} else {
		return OptionProfit{}, OptionProfit{}, errors.New("calculateSpreadProfitAtExpiry: invalid option1 type")
	}

	optionProfit1.Profit = profit1

	profit2, err := calculateOptionProfitAtExpiry(option2, side2, underlyingClosePrcAtExpiry, optionMultiplier)
	if err != nil {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("calculateSpreadProfitAtExpiry: failed to calculate option2 profit: %w", err)
	}

	if side2 == "sell_to_open" {
		profit2 += optionPremium2 * optionMultiplier
	} else if side2 == "buy_to_open" {
		profit2 -= optionPremium2 * optionMultiplier
	} else {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("calculateSpreadProfitAtExpiry: invalid side2 %v", side2)
	}

	var optionProfit2 OptionProfit
	if option2.OptionType == eventmodels.ThetaDataOptionTypeCall {
		if underlyingClosePrcAtExpiry > option2.StrikePrice {
			optionProfit2.IsInMoney = true
		} else {
			optionProfit2.IsInMoney = false
		}
	} else if option2.OptionType == eventmodels.ThetaDataOptionTypePut {
		if underlyingClosePrcAtExpiry < option2.StrikePrice {
			optionProfit2.IsInMoney = true
		} else {
			optionProfit2.IsInMoney = false
		}
	} else {
		return OptionProfit{}, OptionProfit{}, fmt.Errorf("calculateSpreadProfitAtExpiry: invalid option2 type %v", option2.OptionType)
	}

	optionProfit2.Profit = profit2

	return optionProfit1, optionProfit2, nil
}

func FormatOptionSymbol(s eventmodels.OptionSymbol) eventmodels.OptionSymbol {
	upper := strings.ToUpper(string(s))
	if upper[:2] == "O:" {
		return s[2:]
	}

	return s
}

func CalculateOptionOrderSpreadResult(req eventmodels.OptionSpreadAnalysisRequest, underlyingDailyCandles []*eventmodels.CandleDTO, optionMultiplier float64) (*eventmodels.OptionOrderSpreadResult, error) {
	log.Infof("processing option spread analysis request %v", req)
	log.Infof("leg 1: %v", req.Leg1)
	log.Infof("leg 2: %v", req.Leg2)

	if len(underlyingDailyCandles) == 0 {
		return nil, errors.New("underlyingCandles cannot be empty")
	}

	signalName, expectedProfit, requestedPrice, err := DecodeTag(req.Tag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tag %v: %w", req.Tag, err)
	}

	requestedPrice *= -1
	slippage := requestedPrice - req.AvgFillPrice
	symbolLeg1 := FormatOptionSymbol(req.Leg1.Symbol)
	symbolLeg2 := FormatOptionSymbol(req.Leg2.Symbol)

	option1, err := eventmodels.NewOptionSymbolComponents(symbolLeg1)
	side1 := req.Leg1.Side
	if err != nil {
		return nil, fmt.Errorf("failed to parse option1 ticker %v: %w", symbolLeg1, err)
	}

	var option1Type eventmodels.OptionType
	if option1.OptionType == "C" {
		option1Type = eventmodels.OptionTypeCall
	} else if option1.OptionType == "P" {
		option1Type = eventmodels.OptionTypePut
	} else {
		return nil, fmt.Errorf("invalid option1 type %v", option1.OptionType)
	}

	option2, err := eventmodels.NewOptionSymbolComponents(symbolLeg2)
	side2 := req.Leg2.Side
	if err != nil {
		return nil, fmt.Errorf("failed to parse option2 ticker %v: %w", symbolLeg2, err)
	}

	var option2Type eventmodels.OptionType
	if option2.OptionType == "C" {
		option2Type = eventmodels.OptionTypeCall
	} else if option2.OptionType == "P" {
		option2Type = eventmodels.OptionTypePut
	} else {
		return nil, fmt.Errorf("invalid option2 type %v", option2.OptionType)
	}

	now := time.Now()

	isOption1Expired := isOptionExpired(*option1, now)
	if isOption1Expired != isOptionExpired(*option2, now) {
		return nil, errors.New("both options must have the same expiration status")
	}

	expirationDate, err := eventmodels.ConvertToMarketClose(option1.Expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to convert expiration to market close %v: %w", option1.Expiration, err)
	}

	var debitPaid, creditReceived float64
	if req.AvgFillPrice > 0 {
		debitPaid = req.AvgFillPrice * optionMultiplier
	} else {
		creditReceived = -req.AvgFillPrice * optionMultiplier
	}

	underlyingPriceAtOpen, err := findCandleDTOAt(req.CreateDate, underlyingDailyCandles)

	if err != nil {
		return nil, fmt.Errorf("failed to find underlying price at open for %v: %w, req.ID=%v", req.CreateDate, err, req.ID)
	}

	minDistBetweenStrikes := 0.0
	if req.Config.MinDistanceBetweenStrikes != nil {
		minDistBetweenStrikes = *req.Config.MinDistanceBetweenStrikes
	}

	result := eventmodels.OptionOrderSpreadResult{
		OrderID:                         req.ID,
		Underlying:                      req.Underlying,
		ExecutionType:                   req.ExecutionType,
		Strategy:                        "spread",
		CreatedTimestamp:                req.CreateDate,
		DebitPaid:                       debitPaid,
		CreditReceived:                  creditReceived,
		Quantity:                        req.Leg1.Quantity + req.Leg2.Quantity,
		OrderID1:                        req.Leg1.ID,
		Side1:                           req.Leg1.Side,
		OptionType1:                     option1Type,
		Timestamp1:                      req.Leg1.Timestamp,
		Symbol1:                         symbolLeg1,
		StrikePrice1:                    option1.StrikePrice,
		Quantity1:                       req.Leg1.Quantity,
		AvgFillPrice1:                   req.Leg1.AvgFillPrice,
		OrderID2:                        req.Leg2.ID,
		Side2:                           req.Leg2.Side,
		OptionType2:                     option2Type,
		Timestamp2:                      req.Leg2.Timestamp,
		Symbol2:                         symbolLeg2,
		Quantity2:                       req.Leg2.Quantity,
		StrikePrice2:                    option2.StrikePrice,
		AvgFillPrice2:                   req.Leg2.AvgFillPrice,
		SignalName:                      string(signalName),
		ExpectedProfit:                  expectedProfit * optionMultiplier,
		RequestedPrice:                  requestedPrice,
		IsClosed:                        isOption1Expired,
		ExpirationDate:                  expirationDate,
		ExecutedPrice:                   req.AvgFillPrice,
		Slippage:                        slippage,
		UnderlyingPriceAtOpen:           underlyingPriceAtOpen.Close,
		StartsAtConfig:                  req.Config.StartsAt,
		EndsAtConfig:                    req.Config.EndsAt,
		ExpirationsInDaysConfig:         req.Config.ExpirationsInDays,
		MinDistanceBetweenStrikesConfig: minDistBetweenStrikes,
		MaxNoOfStrikesConfig:            req.Config.MaxNoOfStrikes,
	}

	buffer := 15 * time.Minute
	if isOption1Expired {
		symbol1DataAtExpiry, err := findCandleDTOAt(option1.Expiration.Add(-buffer), underlyingDailyCandles)
		if err != nil {
			return nil, fmt.Errorf("failed to find symbol1 data at expiry: %w", err)
		}

		optionMultiplier := 100.0
		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(*option1, side1, req.Leg1.AvgFillPrice, *option2, side2, req.Leg2.AvgFillPrice, symbol1DataAtExpiry.Close, optionMultiplier)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate spread profit at expiry: %w", err)
		}

		result.UnderlyingPriceAtExpiry = symbol1DataAtExpiry.Close
		result.InTheMoney1 = optionProfit1.IsInMoney
		result.Profit1 = optionProfit1.Profit
		result.InTheMoney2 = optionProfit2.IsInMoney
		result.Profit2 = optionProfit2.Profit
		result.Profit = optionProfit1.Profit + optionProfit2.Profit
	}

	return &result, nil
}
