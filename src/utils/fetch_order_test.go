package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestCalculateOptionOrderSpreadResult(t *testing.T) {
	t.Run("order is nil", func(t *testing.T) {
		var order eventmodels.OptionOrderSpreadResult
		data := []eventmodels.OratsOptionData{
			{
				Ticker:    "AAPL",
				TradeDate: "2021-01-01",
				SpotPrice: 100,
				ExpirDate: "2021-01-15",
			},
		}

		_, err := CalculateOptionOrderSpreadResult(&order, data)
		assert.NotNil(t, err)
	})

	t.Run("data is empty", func(t *testing.T) {
		order := eventmodels.OptionOrderSpreadResult{
			Underlying:        "AAPL",
			CreatedTimestamp:  eventmodels.NewTimestamp("2021-01-01"),
			ExpirDate:         "2021-01-15",
			Quantity1:         1,
			Quantity2:         1,
			AvgFillPrice1:     1,
			AvgFillPrice2:     1,
			MaxProfit:         0,
			MaxProfitTimestamp: eventmodels.NewTimestamp("2021-01-01"),
		}
		data := []eventmodels.OratsOptionData{}

		_, err := CalculateOptionOrderSpreadResult(&order, data)
		assert.NotNil(t, err)
	}
}
