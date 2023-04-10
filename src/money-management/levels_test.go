package money_management

import (
	"github.com/stretchr/testify/assert"
	"slack-trading/src/models"
	"testing"
)

// todo
// 1. place algo trade on rsi cross < 30 || > 70 if net exposure  <> 0
// 2. should see a slack alert
// 3. add 1 BTC on each trade. close all on opposite signal

func TestAccount(t *testing.T) {
	t.Run("fails if no levels are set", func(t *testing.T) {
		_, err := NewAccount(1.0, 0.5, PriceLevels{})
		assert.ErrorIs(t, err, models.LevelsNotSetErr)
	})

	t.Run("errors if maxLossPercentage is invalid", func(t *testing.T) {
		_, err := NewAccount(1.0, -1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, models.MaxLossPercentErr)

		_, err = NewAccount(1.0, 1.1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.NotNil(t, err, models.MaxLossPercentErr)
	})

	t.Run("errors if price levels are not sorted", func(t *testing.T) {
		_, err := NewAccount(1.0, 1.0, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 3.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, models.PriceLevelsNotSortedErr)
	})
}

func TestPlacingTrades(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05
	priceLevels := PriceLevels{
		Values: []*PriceLevel{
			{
				Price:             1.0,
				NoOfTrades:        3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				AllocationPercent: 0.5,
			},
		},
	}

	t.Run("can place a buy order", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, priceLevels)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("can place a sell order", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, priceLevels)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		err = account.PlaceOrder(models.TradeTypeSell, 1.5, 2.0)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("able to place trade in another band when original band is full", func(t *testing.T) {

	})

	t.Run("errors is placing a trade when account balance is too low", func(t *testing.T) {

	})

	t.Run("volume increases in a specific band as winners increase", func(t *testing.T) {

	})

	t.Run("volume decreases in a specific band as losers increase", func(t *testing.T) {

	})

	t.Run("errors if too many trades place within price level", func(t *testing.T) {
		// MaxTradesPerPriceLevelErr
	})

	t.Run("errors if placing a trade outside of range", func(t *testing.T) {
		trade := models.Trade{
			RequestedPrice: 1.5,
		}

		account, err := NewAccount(1000.00, 0.5, PriceLevels{
			Values: []*PriceLevel{
				{
					Price: 1.0,
				},
				{
					Price: 2.0,
				},
				{
					Price: 2.2,
				},
			},
		})
		assert.Nil(t, err)

		err = account.CanPlaceTrade(trade)
		assert.Nil(t, err)

		// OK
		trade.RequestedPrice = 2.1
		err = account.CanPlaceTrade(trade)
		assert.Nil(t, err)

		// failure case
		trade.RequestedPrice = 2.2
		err = account.CanPlaceTrade(trade)
		assert.ErrorIs(t, err, models.PriceOutsideLimitsErr)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("errors when a trade needs to be closed due to stop loss", func(t *testing.T) {

	})

	t.Run("errors when all trades need to be closed because account balance is too low", func(t *testing.T) {

	})
}

func TestBalance(t *testing.T) {
	t.Run("balances allocations are correctly allocated", func(t *testing.T) {

	})

	t.Run("used balance increases after trade is placed", func(t *testing.T) {

	})
}
