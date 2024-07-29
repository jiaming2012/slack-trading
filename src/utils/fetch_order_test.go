package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestCalculateOptionOrderSpreadResult(t *testing.T) {
	optionMultiplier := 100.0

	t.Run("order is nil", func(t *testing.T) {
		var order eventmodels.OptionSpreadAnalysisRequest
		data := []*eventmodels.CandleDTO{
			{
				Date:  "2021-01-01",
				Open:  100,
				High:  100,
				Low:   100,
				Close: 100,
			},
		}

		_, err := CalculateOptionOrderSpreadResult(order, data, optionMultiplier)
		assert.NotNil(t, err)
	})

	t.Run("data is empty", func(t *testing.T) {
		createdTstamp, err := time.Parse("2006-01-02", "2021-01-01")
		assert.Nil(t, err)

		order := eventmodels.OptionSpreadAnalysisRequest{
			ID:            1,
			Underlying:    "AAPL",
			ExecutionType: "market",
			CreateDate:    createdTstamp,
			Tag:           "",
			AvgFillPrice:  1,
			Leg1: eventmodels.OptionSpreadLeg{
				ID:           1,
				Timestamp:    createdTstamp,
				Symbol:       "AAPL_011521P100",
				Side:         "sell_to_open",
				Quantity:     1,
				AvgFillPrice: 1,
			},
			Leg2: eventmodels.OptionSpreadLeg{
				ID:           1,
				Timestamp:    createdTstamp,
				Symbol:       "AAPL_011521P95",
				Side:         "buy_to_open",
				Quantity:     1,
				AvgFillPrice: 1,
			},
		}
		data := []*eventmodels.CandleDTO{}

		_, err = CalculateOptionOrderSpreadResult(order, data, optionMultiplier)

		assert.NotNil(t, err)
	})
}

func TestCalculateOptionsPriceAtExpiry(t *testing.T) {
	optionMultiplier := 100.0

	t.Run("order is nil", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{}
		side1 := "sell_to_open"
		premiumPaid1 := 0.0

		option2 := eventmodels.OptionSymbolComponents{}
		side2 := "buy_to_open"
		premiumPaid2 := 0.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, 0, 0)
		assert.NotNil(t, err)

		assert.Equal(t, OptionProfit{}, optionProfit1)
		assert.Equal(t, OptionProfit{}, optionProfit2)
	})

	t.Run("call spread: both options expire out of the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 120,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 90.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0, IsInMoney: false}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0, IsInMoney: false}, optionProfit2)
	})

	t.Run("call spread: both options expire in the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 120,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 130.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0 - 3000.0, IsInMoney: true}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0 + 1000.0, IsInMoney: true}, optionProfit2)
	})

	t.Run("call spread: short option expires in the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 120,
			OptionType:  eventmodels.ThetaDataOptionTypeCall,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 110.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0 - 1000.0, IsInMoney: true}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0, IsInMoney: false}, optionProfit2)
	})

	t.Run("put spread: both options expire out of the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 80,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 110.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0, IsInMoney: false}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0, IsInMoney: false}, optionProfit2)
	})

	t.Run("put spread: both options expire in the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 80,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 70.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0 - 3000.0, IsInMoney: true}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0 + 1000.0, IsInMoney: true}, optionProfit2)
	})

	t.Run("put spread: short option expires in the money", func(t *testing.T) {
		option1 := eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side1 := "sell_to_open"
		premiumPaid1 := -1.0

		option2 := eventmodels.OptionSymbolComponents{
			StrikePrice: 80,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}
		side2 := "buy_to_open"
		premiumPaid2 := 0.5

		underlyingPriceAtExpiry := 90.0

		optionProfit1, optionProfit2, err := calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0 - 1000.0, IsInMoney: true}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0, IsInMoney: false}, optionProfit2)

		// flip the options
		option1 = eventmodels.OptionSymbolComponents{
			StrikePrice: 80,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}

		side1 = "buy_to_open"

		option2 = eventmodels.OptionSymbolComponents{
			StrikePrice: 100,
			OptionType:  eventmodels.ThetaDataOptionTypePut,
		}

		side2 = "sell_to_open"

		optionProfit1, optionProfit2, err = calculateSpreadProfitAtExpiry(option1, side1, premiumPaid1, option2, side2, premiumPaid2, underlyingPriceAtExpiry, optionMultiplier)
		assert.Nil(t, err)

		assert.Equal(t, OptionProfit{Profit: 100.0, IsInMoney: false}, optionProfit1)
		assert.Equal(t, OptionProfit{Profit: -50.0 - 1000.0, IsInMoney: true}, optionProfit2)
	})
}
