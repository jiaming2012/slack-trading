package service

import (
	"github.com/stretchr/testify/assert"
	"slack-trading/src/models"
	"testing"
)

func TestPlacingTrades(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05
	newPriceLevels := func() models.PriceLevels {
		return models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					NoOfTrades:        1,
					AllocationPercent: 0.5,
				},
				{
					Price:             10.0,
					NoOfTrades:        0,
					AllocationPercent: 0,
				},
			},
		}
	}

	newPriceLevels2 := func() models.PriceLevels {
		return models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        2,
					AllocationPercent: 0.5,
				},
				{
					Price: 2.0,
				},
				{
					Price:             3.0,
					NoOfTrades:        2,
					AllocationPercent: 0.5,
				},
				{
					Price: 4.0,
				},
			},
		}
	}

	t.Run("can place a buy order", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("can place a sell order", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		_, err = account.PlaceOrder(models.TradeTypeSell, 1.5, 2.0, -1)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("able to place trade in another band when original band is full", func(t *testing.T) {

		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.ErrorIs(t, err, models.MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 2.5, 1.0, -1)
		assert.ErrorIs(t, err, models.MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 3.5, 1.0, -1)
		assert.Nil(t, err)
	})

	t.Run("always able to place a trade which reduces account exposure", func(t *testing.T) {
		priceLevels := models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        2,
					AllocationPercent: 1,
				},
				{
					Price: 2.0,
				},
				{
					Price:             3.0,
					NoOfTrades:        0,
					AllocationPercent: 0.0,
				},
			},
		}

		curPrice := 1.5

		account, err := models.NewAccount(balance, maxLossPerc, priceLevels)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.ErrorIs(t, err, models.MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(models.TradeTypeSell, curPrice, -1, 0.5)
		assert.Nil(t, err)
	})

	t.Run("able to place additional trades in bands once previous trade is closed", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		curPrice := 1.5
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		tradesRemaining, side := account.TradesRemaining(curPrice)
		assert.Equal(t, 0, tradesRemaining)
		assert.Equal(t, side, models.TradeTypeBuy)

		_, err = account.PlaceOrder(models.TradeTypeBuy, curPrice, 1.0, -1)
		assert.ErrorIs(t, err, models.MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(models.TradeTypeSell, curPrice, 2.5, 1)
		assert.Nil(t, err)
		tradesRemaining, side = account.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)
		assert.Equal(t, side, models.TradeTypeBuy)
	})

	t.Run("no stop loss required for closing trades", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeSell, 1.9, -1, 1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeSell, 3.5, 4.5, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, -1, 1)
		assert.Nil(t, err)
	})

	t.Run("closing trades must have close percentage", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeBuy, 1.7, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeSell, 1.9, -1, -1)
		assert.ErrorIs(t, err, models.InvalidClosePercentErr)
	})

	t.Run("volume increases in a specific band as winners increase", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		t1, err := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)
		t1.AutoExecute()

		t2, err := account.PlaceOrder(models.TradeTypeSell, 1.9, -1, 1)
		assert.Nil(t, err)
		t2.AutoExecute()

		t3, err := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)
		t3.AutoExecute()

		assert.Greater(t, t3.Volume, t1.Volume)
	})

	t.Run("volume decreases in a specific band as losers increase", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		t1, err := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		t1.AutoExecute()
		assert.Nil(t, err)

		t2, err := account.PlaceOrder(models.TradeTypeSell, 1.2, -1, 1)
		t2.AutoExecute()
		assert.Nil(t, err)

		t3, err := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		t3.AutoExecute()
		assert.Nil(t, err)

		assert.Less(t, t3.Volume, t1.Volume)
	})

	// todo: turn price level into price bands
	t.Run("errors when too much money is lost within a price range", func(t *testing.T) {
		var err error
		account, err := models.NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		for i := 0; i < 41; i++ { // i < 41 was chosen to make the test fail
			t1, tradeErr := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
			t1.AutoExecute()
			assert.Nil(t, tradeErr)

			t2, tradeErr := account.PlaceOrder(models.TradeTypeSell, 1.2, -1, 1)
			t2.AutoExecute()
			assert.Nil(t, tradeErr)
		}

		_, tradeErr := account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
		assert.ErrorIs(t, tradeErr, models.TradeVolumeIsZeroErr)
	})
}

func TestUpdate(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05

	t.Run("errors when a trade needs to be closed due to stop loss", func(t *testing.T) {
		priceLevel := models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					NoOfTrades:        1,
					AllocationPercent: 0.5,
				},
				{
					Price:             10.0,
					NoOfTrades:        0,
					AllocationPercent: 0,
				},
			},
		}

		account, err := models.NewAccount(balance, maxLossPerc, priceLevel)
		assert.Nil(t, err)

		closeReq := account.Update(1.5)
		assert.Nil(t, closeReq)

		t1, err := account.PlaceOrder(models.TradeTypeSell, 1.5, 2.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(models.TradeTypeSell, 1.6, 2.5, -1)
		assert.Nil(t, err)

		closeReq = account.Update(1.9)
		assert.Nil(t, closeReq)

		closeReq = account.Update(2.3)
		assert.NotNil(t, closeReq)
		assert.Equal(t, 1, len(closeReq.Trades))
		assert.Equal(t, t1.ID, closeReq.Trades[0].ID)
	})

	t.Run("errors when all trades need to be closed because account balance is too low", func(t *testing.T) {
		priceLevel := models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					Price:             100000.0,
					NoOfTrades:        1,
					AllocationPercent: 1.0,
				},
				{
					Price:             105000.0,
					AllocationPercent: 0,
				},
			},
		}

		account, err := models.NewAccount(balance, maxLossPerc, priceLevel)
		assert.Nil(t, err)

		curPrice := 100000.00
		maxLoss := balance * maxLossPerc

		trade, err := account.PlaceOrder(models.TradeTypeBuy, curPrice, 30000, -1)
		assert.Nil(t, err)

		trade.Execute(curPrice)

		stopOutPrice := ((trade.ExecutedPrice * trade.Volume) - maxLoss) / trade.Volume

		closeReq := account.Update(stopOutPrice)
		assert.NotNil(t, closeReq)
		assert.Len(t, closeReq.Trades, 1)
		assert.Equal(t, closeReq.Trades[0].ID, trade.ID)
	})
}

func TestTradeValidation(t *testing.T) {
	balance := 1000.00
	maxLossPercentage := 0.5
	newPriceLevels := func() models.PriceLevels {
		return models.PriceLevels{
			Values: []*models.PriceLevel{
				{
					AllocationPercent: 0.33333,
					NoOfTrades:        1,
					Price:             1.0,
				},
				{
					AllocationPercent: 0.33333,
					NoOfTrades:        1,
					Price:             2.0,
				},
				{
					AllocationPercent: 0.33333,
					NoOfTrades:        1,
					Price:             2.2,
				},
				{
					Price:             10.0,
					NoOfTrades:        0,
					AllocationPercent: 0,
				},
			},
		}
	}

	t.Run("errors when placing a trade outside of a trading band", func(t *testing.T) {
		account, err := models.NewAccount(balance, maxLossPercentage, newPriceLevels())
		_, err = account.PlaceOrder(models.TradeTypeBuy, 0.5, 0.1, -1)
		assert.ErrorIs(t, err, models.PriceOutsideLimitsErr)
	})

	t.Run("errors if checking to placing a trade outside of range", func(t *testing.T) {
		trade := models.TradeRequest{
			Price: 1.5,
		}

		account, err := models.NewAccount(balance, maxLossPercentage, newPriceLevels())
		assert.Nil(t, err)

		_, err = account.CanPlaceTrade(trade)
		assert.Nil(t, err)

		// OK
		trade.Price = 2.1
		_, err = account.CanPlaceTrade(trade)
		assert.Nil(t, err)

		// failure case
		trade.Price = 11.0
		_, err = account.CanPlaceTrade(trade)
		assert.ErrorIs(t, err, models.PriceOutsideLimitsErr)
	})
}

func TestPriceLevel(t *testing.T) {
	t.Run("test trades remaining", func(t *testing.T) {
		priceLevel := models.PriceLevel{
			NoOfTrades: 5,
			Trades: &models.Trades{
				{
					Volume: 1.0,
				},
				{
					Volume: -1.0,
				},
			},
		}

		tradesRemaining, side := priceLevel.NewTradesRemaining()
		assert.Equal(t, 5, tradesRemaining)
		assert.Equal(t, side, models.TradeTypeBuy)

		priceLevel.Trades.Add(&models.Trade{
			Volume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		assert.Equal(t, 4, tradesRemaining)
		assert.Equal(t, side, models.TradeTypeSell)
	})
}

//func TestBalance(t *testing.T) {
//	balance := 10000.00
//	maxLossPerc := 0.05
//	models.PriceLevels := models.PriceLevels{
//		Values: []*models.PriceLevel{
//			{
//				Price:             1.0,
//				NoOfTrades:        3,
//				AllocationPercent: 0.5,
//			},
//			{
//				Price:             2.0,
//				AllocationPercent: 0.5,
//			},
//		},
//	}
//
//	t.Run("balances allocations are correctly allocated", func(t *testing.T) {
//		account, err := models.NewAccount(balance, maxLossPerc, models.PriceLevels)
//		assert.Nil(t, err)
//
//
//
//		err = account.PlaceOrder(models.TradeTypeBuy, 1.5, 1.0, -1)
//		assert.Nil(t, err)
//	})
//
//	t.Run("used balance increases after trade is placed", func(t *testing.T) {
//
//	})
//}
