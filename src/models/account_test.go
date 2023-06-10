package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// todo
// 1. place algo trade on rsi cross < 30 || > 70 if net exposure  <> 0
// 2. should see a slack alert
// 3. add 1 BTC on each trade. close all on opposite signal

func TestAccount(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05

	t.Run("the last price level must have an allocation of zero", func(t *testing.T) {
		priceLevels := PriceLevels{
			Values: []*PriceLevel{
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
			},
		}

		_, err := NewAccount(1.0, 0.5, priceLevels)

		assert.ErrorIs(t, err, PriceLevelsLastAllocationErr)

		priceLevels.Values = append(priceLevels.Values, &PriceLevel{
			Price:             6.0,
			NoOfTrades:        0,
			AllocationPercent: 0,
		})

		_, err = NewAccount(1.0, 0.5, priceLevels)

		assert.Nil(t, err)
	})

	t.Run("fails if no levels are set", func(t *testing.T) {
		_, err := NewAccount(1.0, 0.5, PriceLevels{})
		assert.ErrorIs(t, err, LevelsNotSetErr)
	})

	t.Run("errors if maxLossPercentage is invalid", func(t *testing.T) {
		_, err := NewAccount(1.0, -1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, MaxLossPercentErr)

		_, err = NewAccount(1.0, 1.1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.NotNil(t, err, MaxLossPercentErr)
	})

	t.Run("errors if price levels are not sorted", func(t *testing.T) {
		_, err := NewAccount(1.0, 1.0, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0, AllocationPercent: 1, NoOfTrades: 1}, {Price: 3.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, PriceLevelsNotSortedErr)
	})

	t.Run("num of trade > 0 if allocation is > 0", func(t *testing.T) {
		_priceLevels := PriceLevels{
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
				{
					Price: 3.0,
				},
			},
		}

		_, err := NewAccount(balance, maxLossPerc, _priceLevels)
		assert.ErrorIs(t, err, NoOfTradeMustBeNonzeroErr)
	})

	t.Run("num of trades must be zero if allocation is zero", func(t *testing.T) {
		_priceLevels1 := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					AllocationPercent: 0,
				},
				{
					Price:             3.0,
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

		_, err := NewAccount(balance, maxLossPerc, _priceLevels1)
		assert.Nil(t, err)

		_priceLevels2 := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					NoOfTrades:        3,
					AllocationPercent: 0,
				},
				{
					Price:             3.0,
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

		_, err = NewAccount(balance, maxLossPerc, _priceLevels2)
		assert.ErrorIs(t, err, NoOfTradesMustBeZeroErr)
	})
}

func TestPlacingTrades(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05
	newPriceLevels := func() PriceLevels {
		return PriceLevels{
			Values: []*PriceLevel{
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

	newPriceLevels2 := func() PriceLevels {
		return PriceLevels{
			Values: []*PriceLevel{
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
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("can place a sell order", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		_, err = account.PlaceOrder(TradeTypeSell, 1.5, 2.0, -1)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 1)
	})

	t.Run("able to place trade in another band when original band is full", func(t *testing.T) {

		account, err := NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(TradeTypeBuy, 2.5, 1.0, -1)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(TradeTypeBuy, 3.5, 1.0, -1)
		assert.Nil(t, err)
	})

	t.Run("always able to place a trade which reduces account exposure", func(t *testing.T) {
		priceLevels := PriceLevels{
			Values: []*PriceLevel{
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

		account, err := NewAccount(balance, maxLossPerc, priceLevels)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(TradeTypeSell, curPrice, -1, 0.5)
		assert.Nil(t, err)
	})

	t.Run("able to place additional trades in bands once previous trade is closed", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		curPrice := 1.5
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.Nil(t, err)

		tradesRemaining, side := account.TradesRemaining(curPrice)
		assert.Equal(t, 0, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)

		_, err = account.PlaceOrder(TradeTypeBuy, curPrice, 1.0, -1)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		_, err = account.PlaceOrder(TradeTypeSell, curPrice, 2.5, 1)
		assert.Nil(t, err)
		tradesRemaining, side = account.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)
	})

	t.Run("no stop loss required for closing trades", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeSell, 1.9, -1, 1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeSell, 3.5, 4.5, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, -1, 1)
		assert.Nil(t, err)
	})

	t.Run("able to close a trade outside of price bands", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		tr1, err := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)
		tr1.AutoExecute()

		level := account.findPriceLevel(tr1.RequestedPrice)

		req := BulkCloseRequest{
			Items: []BulkCloseRequestItem{
				{
					Level:        level,
					ClosePercent: 1.0,
				},
			},
		}

		trades, err := req.Execute(10.5)
		assert.Nil(t, err)
		assert.Len(t, trades, 1)
		assert.Equal(t, TradeTypeSell, trades[0].Side())
		assert.Equal(t, -tr1.Volume, trades[0].Volume)
		assert.Equal(t, 10.5, trades[0].RequestedPrice)
	})

	t.Run("closing trades must have close percentage", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeBuy, 1.7, 1.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeSell, 1.9, -1, -1)
		assert.ErrorIs(t, err, InvalidClosePercentErr)
	})

	t.Run("trades closed in any band reduces exposure of previous trades in a different band", func(t *testing.T) {

	})

	t.Run("closing one half of a trade twice increases the number of trades allowed by one", func(t *testing.T) {

	})

	t.Run("volume increases in a specific band as winners increase", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		t1, err := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)
		t1.AutoExecute()

		t2, err := account.PlaceOrder(TradeTypeSell, 1.9, -1, 1)
		assert.Nil(t, err)
		t2.AutoExecute()

		t3, err := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.Nil(t, err)
		t3.AutoExecute()

		assert.Greater(t, t3.Volume, t1.Volume)
	})

	t.Run("volume decreases in a specific band as losers increase", func(t *testing.T) {
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels())
		assert.Nil(t, err)

		t1, err := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		t1.AutoExecute()
		assert.Nil(t, err)

		t2, err := account.PlaceOrder(TradeTypeSell, 1.2, -1, 1)
		t2.AutoExecute()
		assert.Nil(t, err)

		t3, err := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		t3.AutoExecute()
		assert.Nil(t, err)

		assert.Less(t, t3.Volume, t1.Volume)
	})

	// todo: turn price level into price bands
	t.Run("errors when too much money is lost within a price range", func(t *testing.T) {
		var err error
		account, err := NewAccount(balance, maxLossPerc, newPriceLevels2())
		assert.Nil(t, err)

		for i := 0; i < 41; i++ { // i < 41 was chosen to make the test fail
			t1, tradeErr := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
			t1.AutoExecute()
			assert.Nil(t, tradeErr)

			t2, tradeErr := account.PlaceOrder(TradeTypeSell, 1.2, -1, 1)
			t2.AutoExecute()
			assert.Nil(t, tradeErr)
		}

		_, tradeErr := account.PlaceOrder(TradeTypeBuy, 1.5, 1.0, -1)
		assert.ErrorIs(t, tradeErr, TradeVolumeIsZeroErr)
	})
}

func TestUpdate(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05

	t.Run("errors when a trade needs to be closed due to stop loss", func(t *testing.T) {
		priceLevel := PriceLevels{
			Values: []*PriceLevel{
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

		account, err := NewAccount(balance, maxLossPerc, priceLevel)
		assert.Nil(t, err)

		closeReq := account.Update(1.5)
		assert.Nil(t, closeReq)

		t1, err := account.PlaceOrder(TradeTypeSell, 1.5, 2.0, -1)
		assert.Nil(t, err)

		_, err = account.PlaceOrder(TradeTypeSell, 1.6, 2.5, -1)
		assert.Nil(t, err)

		closeReq = account.Update(1.9)
		assert.Nil(t, closeReq)

		closeReq = account.Update(2.3)
		assert.NotNil(t, closeReq)
		assert.Equal(t, 1, len(closeReq.Trades))
		assert.Equal(t, t1.ID, closeReq.Trades[0].ID)
	})

	t.Run("errors when all trades need to be closed because account balance is too low", func(t *testing.T) {
		priceLevel := PriceLevels{
			Values: []*PriceLevel{
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

		account, err := NewAccount(balance, maxLossPerc, priceLevel)
		assert.Nil(t, err)

		curPrice := 100000.00
		maxLoss := balance * maxLossPerc

		trade, err := account.PlaceOrder(TradeTypeBuy, curPrice, 30000, -1)
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
	newPriceLevels := func() PriceLevels {
		return PriceLevels{
			Values: []*PriceLevel{
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
		account, err := NewAccount(balance, maxLossPercentage, newPriceLevels())
		_, err = account.PlaceOrder(TradeTypeBuy, 0.5, 0.1, -1)
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
	})

	t.Run("errors if checking to placing a trade outside of range", func(t *testing.T) {
		trade := TradeRequest{
			Price: 1.5,
		}

		account, err := NewAccount(balance, maxLossPercentage, newPriceLevels())
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
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
	})
}
