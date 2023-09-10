package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// todo
// 1. place algo trade on rsi cross < 30 || > 70 if net exposure  <> 0
// 2. should see a slack alert
// 3. add 1 BTC on each trade. close all on opposite signal

func TestAccountStrategy(t *testing.T) {
	name := "Test Account"
	direction := Direction("up")
	priceLevels := []*PriceLevel{
		{
			Price:             1.0,
			MaxNoOfTrades:     3,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             2.0,
			MaxNoOfTrades:     1,
			AllocationPercent: 0.5,
			StopLoss:          0.5,
		},
		{
			Price:             3.0,
			AllocationPercent: 0,
		},
	}

	t.Run("cannot add a strategy with the same name", func(t *testing.T) {
		account, err := NewAccount(name, 1000)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, 100, priceLevels)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		strategy2, err := NewStrategy("test", "BTCUSD", direction, 100, priceLevels)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy2)
		assert.Error(t, err)
	})
}

func TestPlacingTrades(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	balance := 10000.00
	name := "Test Placing Trades"
	direction := Direction("up")
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	timeframe := 5
	symbol := "TestSymbol"

	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
				StopLoss:          0.5,
			},
			{
				Price:             2.0,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
				StopLoss:          1.0,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
				StopLoss:          9.0,
			},
		}
	}

	newPriceLevels2 := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     2,
				AllocationPercent: 0.5,
				StopLoss:          0.5,
			},
			{
				Price:    2.0,
				StopLoss: 1.5,
			},
			{
				Price:             3.0,
				MaxNoOfTrades:     2,
				AllocationPercent: 0.5,
				StopLoss:          2.5,
			},
			{
				Price:    4.0,
				StopLoss: 3.0,
			},
		}
	}

	t.Run("can place an open trade request", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		openPrice := 1.5

		req, err := account.PlaceOpenTradeRequest(strategy.Name, openPrice)
		assert.Nil(t, err)

		assert.Equal(t, TradeTypeBuy, req.Type)
		assert.Equal(t, openPrice, req.Price)
		assert.Equal(t, strategy, req.Strategy)

		curPriceLevel := strategy.findPriceLevel(openPrice)
		assert.NotNil(t, curPriceLevel)

		assert.Equal(t, curPriceLevel.StopLoss, req.StopLoss)
		assert.Equal(t, strategy.Symbol, req.Symbol)
	})

	t.Run("can place a sell order", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", "down", balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		openPrice := 2.0

		req, err := account.PlaceOpenTradeRequest(strategy.Name, openPrice)
		assert.Nil(t, err)

		assert.Equal(t, TradeTypeSell, req.Type)
		assert.Equal(t, openPrice, req.Price)
		assert.Equal(t, strategy, req.Strategy)

		curPriceLevel := strategy.findPriceLevel(openPrice)
		assert.NotNil(t, curPriceLevel)

		assert.Equal(t, curPriceLevel.StopLoss, req.StopLoss)
		assert.Equal(t, strategy.Symbol, req.Symbol)
	})

	t.Run("able to place trade in another band when original band is full", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance, newPriceLevels2())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		trade1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, 1, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade1)
		assert.Nil(t, err)

		trade2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, 1, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade2)
		assert.Nil(t, err)

		trade3, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, 1, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade3)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		trade4, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, 1, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade4)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		trade6, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 3.5, 1, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade6)
		assert.Nil(t, err)
	})

	t.Run("always able to place a trade which reduces account exposure", func(t *testing.T) {
		priceLevels := []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     2,
				AllocationPercent: 1,
				StopLoss:          0.5,
			},
			{
				Price: 2.0,
			},
			{
				Price:             3.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0.0,
				StopLoss:          2.0,
			},
		}

		requestedPrice := 1.5

		account, err := NewAccount(name, balance)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance, priceLevels)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		trade1, err := strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade1)
		assert.Nil(t, err)

		trade2, err := strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade2)
		assert.Nil(t, err)

		trade3, err := strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade3)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		trade4, err := strategy.NewCloseTrade(id, timeframe, timestamp, requestedPrice, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade4)
		assert.Nil(t, err)
	})

	t.Run("able to place additional trades in bands once previous trade is closed", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		curPrice := 1.5
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", symbol, direction, balance, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		tradesRemaining, side := strategy.TradesRemaining(curPrice)
		assert.Equal(t, 3, tradesRemaining)
		assert.Equal(t, side, TradeTypeNone)

		trade1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade1)
		assert.Nil(t, err)

		trade2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade2)
		assert.Nil(t, err)

		trade3, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade3)
		assert.Nil(t, err)

		tradesRemaining, side = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 0, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)

		trade4, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade4)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		trade5, err := NewCloseTrade(id, []*Trade{trade1, trade2, trade3}, timeframe, timestamp, curPrice, 2.5)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade5)
		assert.Nil(t, err)

		tradesRemaining, side = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 2, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)
	})

	t.Run("able to close a trade outside of price bands", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		tr1Volume := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, tr1Volume, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		tr1ClosePrc := 10.5
		closeTr, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, -tr1Volume)
		assert.Nil(t, err)

		assert.Equal(t, TradeTypeClose, closeTr.Type)
		assert.Equal(t, -tr1Volume, closeTr.RequestedVolume)
		assert.Equal(t, tr1ClosePrc, closeTr.RequestedPrice)

		closeTr.Execute(tr1ClosePrc, -tr1Volume)
		assert.Equal(t, -tr1Volume, closeTr.ExecutedVolume)
		assert.Equal(t, tr1ClosePrc, closeTr.ExecutedPrice)
	})

	t.Run("closing trades must have close percentage", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		tr1Volume := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, tr1Volume, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		tr1ClosePrc := 10.5
		_, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, -tr1Volume-0.001)
		assert.ErrorIs(t, err, InvalidClosingTradeVolumeErr)
	})

	t.Run("closing one half of a trade twice increases the number of trades allowed by one", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		curPrice := 1.5
		tradesRemaining, _ := strategy.TradesRemaining(curPrice)
		assert.Equal(t, 3, tradesRemaining)

		trVolume := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, trVolume, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, trVolume, 1.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		tradesRemaining, _ = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)

		tr1ClosePrc := 10.5
		tr3, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, trVolume/2.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr3)
		assert.Nil(t, err)

		tradesRemaining, _ = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)
	})

	t.Run("volume increases in a specific band as winners increase", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		trVolume := 1.0
		tr1, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, 1.9, trVolume)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		tr3, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr3)
		assert.Nil(t, err)

		assert.Greater(t, tr3.ExecutedVolume, tr1.ExecutedVolume)
	})

	t.Run("volume decreases in a specific band as losers increase", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		tr1, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, 1.2, tr1.ExecutedVolume)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		tr3, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(tr3)
		assert.Nil(t, err)

		assert.Less(t, tr3.ExecutedVolume, tr1.ExecutedVolume)
	})
}

func TestUpdate(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	balance := 10000.00
	name := "Test Placing Trades"
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	timeframe := 5
	direction := Up

	t.Run("errors when a trade needs to be closed due to stop loss", func(t *testing.T) {
		band1SL := 0.5

		priceLevel := []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          band1SL,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.5,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}

		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance, priceLevel)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		closeReq, err := account.Update(1.5, timeframe)
		assert.Nil(t, err)
		assert.Nil(t, closeReq)

		t0, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		strategy.AutoExecuteTrade(t0)
		assert.Nil(t, err)

		closeReq, err = account.Update(band1SL+0.2, timeframe)
		assert.Nil(t, err)
		assert.Nil(t, closeReq)

		closeReq, err = account.Update(band1SL, timeframe)
		assert.Nil(t, err)
		assert.NotNil(t, closeReq)
		assert.Equal(t, 1, len(closeReq))
		assert.Equal(t, t0, closeReq[0].Trade)
	})

	t.Run("errors account needs to be closed due to stop out with up strategy", func(t *testing.T) {
		getTime := func() time.Time {
			return time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
		}

		getID := func() uuid.UUID {
			return uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
		}

		curPrice := 100000.0
		stopLoss := 0.0000001
		priceLevels := []*PriceLevel{
			{
				Price:             curPrice,
				StopLoss:          stopLoss,
				MaxNoOfTrades:     1,
				AllocationPercent: 1.0,
			},
			{
				Price:             curPrice + 5000.0,
				AllocationPercent: 0,
			},
		}

		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", Up, balance, priceLevels)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		maxLoss := balance

		trade, err := strategy.NewOpenTrade(id, timeframe, timestamp, curPrice)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade)
		assert.Nil(t, err)

		closeReq, err := account.checkStopOut(timeframe, curPrice, getTime, getID)
		assert.Nil(t, err)
		assert.Nil(t, closeReq)

		stopOutPrice := ((trade.ExecutedPrice * trade.ExecutedVolume) - maxLoss) / trade.ExecutedVolume

		closeReq, err = account.checkStopOut(timeframe, stopOutPrice, getTime, getID)
		assert.Nil(t, err)
		assert.NotNil(t, closeReq)
		assert.Len(t, closeReq, 1)
		assert.Equal(t, -trade.ExecutedVolume, closeReq[0].Volume)
		assert.Equal(t, "stop out", closeReq[0].Reason)
	})

	t.Run("errors account needs to be closed due to stop out with down strategy", func(t *testing.T) {
		getTime := func() time.Time {
			return time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
		}

		getID := func() uuid.UUID {
			return uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
		}

		openPrice := 100000.0
		stopLoss := openPrice * 2
		priceLevels := []*PriceLevel{
			{
				Price:             openPrice,
				StopLoss:          stopLoss,
				MaxNoOfTrades:     2,
				AllocationPercent: 1.0,
			},
			{
				Price:             openPrice + 10000.0,
				AllocationPercent: 0,
			},
		}

		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", Down, balance, priceLevels)
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		maxLoss := balance

		trade1, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade1)
		assert.Nil(t, err)

		trade2, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice+5000.0)
		assert.Nil(t, err)
		err = strategy.AutoExecuteTrade(trade2)
		assert.Nil(t, err)

		closeReq, err := account.checkStopOut(timeframe, openPrice+5000.0, getTime, getID)
		assert.Nil(t, err)
		assert.Nil(t, closeReq)

		vwap, vol, _ := strategy.GetTrades().Vwap()
		stopOutPrice := float64(vwap) - (maxLoss / float64(vol))

		closeReq, err = account.checkStopOut(timeframe, stopOutPrice, getTime, getID)
		assert.Nil(t, err)
		assert.NotNil(t, closeReq)
		assert.Len(t, closeReq, 1)
		assert.Equal(t, float64(-vol), closeReq[0].Volume)
		assert.Equal(t, "stop out", closeReq[0].Reason)
	})
}

func TestTradeValidation(t *testing.T) {
	name := "TestTradeValidation"
	balance := 1000.00
	direction := Up
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	timeframe := 5

	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				AllocationPercent: 0.33333,
				MaxNoOfTrades:     1,
				Price:             1.0,
				StopLoss:          0.5,
			},
			{
				AllocationPercent: 0.33333,
				MaxNoOfTrades:     1,
				Price:             2.0,
				StopLoss:          1.5,
			},
			{
				AllocationPercent: 0.33333,
				MaxNoOfTrades:     1,
				Price:             2.2,
				StopLoss:          2.0,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	t.Run("errors when placing a trade outside of a trading band", func(t *testing.T) {
		account, err := NewAccount(name, balance)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		_, err = strategy.NewOpenTrade(id, timeframe, timestamp, 0.5)
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
	})

	t.Run("errors if checking to placing a trade outside of range", func(t *testing.T) {
		account, err := NewAccount(name, balance)
		assert.Nil(t, err)

		strategy, err := NewStrategy("test", "BTCUSD", direction, balance/2.0, newPriceLevels())
		assert.Nil(t, err)

		err = account.AddStrategy(*strategy)
		assert.Nil(t, err)

		// success case
		trade, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.Nil(t, err)
		assert.NotNil(t, trade)

		// failure case
		trade, err = strategy.NewOpenTrade(id, timeframe, timestamp, 11.0)
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
		assert.Nil(t, trade)
	})
}
