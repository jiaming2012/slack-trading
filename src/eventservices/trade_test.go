package eventservices

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"slack-trading/src/models"
)

func TestRealizedDrawdown(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	symbol := "BTCUSD"
	datafeed := models.NewDatafeed(models.ManualDatafeed)
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	tf := new(int)
	*tf = 5

	priceLevelsUp := []*models.PriceLevel{
		{
			Price:             1.0,
			MaxNoOfTrades:     2,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             3.0,
			AllocationPercent: 0,
		},
	}

	priceLevelsDown := []*models.PriceLevel{
		{
			Price: 1.0,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          3.5,
		},
		{
			Price:             3.0,
			MaxNoOfTrades:     1.0,
			AllocationPercent: 0.5,
			StopLoss:          4.0,
		},
	}

	account, err := models.NewAccount("testAccount", 1000, datafeed)
	assert.NoError(t, err)

	buyStrategy, err := models.NewStrategyDeprecated("longStrategy", symbol, models.Up, 100, priceLevelsUp, account)
	assert.NoError(t, err)
	err = account.AddStrategy(buyStrategy)
	assert.NoError(t, err)

	t.Run("ignores candles before trade open", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, datafeed)
		assert.NoError(t, err)

		sellStrategy, err := models.NewStrategyDeprecated("shortStrategy", symbol, models.Down, 100, priceLevelsDown, account)
		assert.NoError(t, err)
		err = account.AddStrategy(sellStrategy)
		assert.NoError(t, err)

		requestedPrice := 2.5

		sellTrade, _, err2 := sellStrategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err2)
		_, err2 = sellStrategy.AutoExecuteTrade(sellTrade)
		assert.NoError(t, err2)

		candles := []*models.Candle{
			{
				Timestamp: ts.Add(-1 * time.Second),
				High:      requestedPrice + 0.9,
			},
			{
				Timestamp: ts.Add(1 * time.Second),
				High:      requestedPrice + 0.5,
			},
		}

		assert.Equal(t, requestedPrice+0.5, RealizedDrawdown(sellTrade, candles, nil))
	})

	t.Run("zero when no candles", func(t *testing.T) {
		requestedPrice := 2.5

		buyTrade, _, err2 := buyStrategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err2)
		_, err2 = buyStrategy.AutoExecuteTrade(buyTrade)
		assert.NoError(t, err2)

		assert.Equal(t, 0.0, RealizedDrawdown(buyTrade, []*models.Candle{}, nil))
	})

	t.Run("buy trade", func(t *testing.T) {
		requestedPrice := 2.5

		buyTrade, _, err2 := buyStrategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err2)
		_, err2 = buyStrategy.AutoExecuteTrade(buyTrade)
		assert.NoError(t, err2)

		candles := []*models.Candle{
			{
				Timestamp: ts.Add(1 * time.Second),
				Low:       requestedPrice - 0.1,
			},
			{
				Timestamp: ts.Add(2 * time.Second),
				Low:       requestedPrice - 0.5,
			},
			{
				Timestamp: ts.Add(3 * time.Second),
				Low:       requestedPrice - 0.4,
			},
		}

		assert.Equal(t, requestedPrice-0.5, RealizedDrawdown(buyTrade, candles, nil))
	})

	t.Run("sell trade", func(t *testing.T) {
		account, err := models.NewAccount("testAccount", 1000, datafeed)
		assert.NoError(t, err)

		sellStrategy, err := models.NewStrategyDeprecated("shortStrategy", symbol, models.Down, 100, priceLevelsDown, account)
		assert.NoError(t, err)
		err = account.AddStrategy(sellStrategy)
		assert.NoError(t, err)

		requestedPrice := 2.5

		sellTrade, _, err2 := sellStrategy.NewOpenTrade(id, tf, ts, requestedPrice)
		assert.NoError(t, err2)
		_, err2 = sellStrategy.AutoExecuteTrade(sellTrade)
		assert.NoError(t, err2)

		candles := []*models.Candle{
			{
				Timestamp: ts.Add(1 * time.Second),
				High:      requestedPrice + 0.9,
			},
			{
				Timestamp: ts.Add(2 * time.Second),
				High:      requestedPrice + 0.5,
			},
			{
				Timestamp: ts.Add(3 * time.Second),
				High:      requestedPrice + 0.7,
			},
		}

		assert.Equal(t, requestedPrice+0.9, RealizedDrawdown(sellTrade, candles, nil))

		candles = append(candles, &models.Candle{
			Timestamp: ts.Add(4 * time.Second),
			High:      requestedPrice + 1.2,
		})

		assert.Equal(t, requestedPrice+1.2, RealizedDrawdown(sellTrade, candles, nil))
	})
}
