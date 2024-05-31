package eventmodels

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAccountStrategy(t *testing.T) {
	name := "Test Account"
	direction := Direction("up")
	symbol := "symbol"
	balance := 100.0
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
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, 1000, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		strategy2, err := NewStrategyDeprecated(name, symbol, direction, balance, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy2)
		assert.Error(t, err)
	})
}

func TestPlacingTrades(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	balance := 10000.00
	name := "Test Placing Trades"
	direction := Up
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	symbol := "TestSymbol"

	timeframe := new(int)
	*timeframe = 5

	newUpPriceLevels := func() []*PriceLevel {
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

	newDownPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price: 1.0,
			},
			{
				Price:             2.0,
				MaxNoOfTrades:     2,
				AllocationPercent: 0.5,
				StopLoss:          12.5,
			},
			{
				Price:             3.0,
				MaxNoOfTrades:     2,
				AllocationPercent: 0.5,
				StopLoss:          3.5,
			},
		}
	}

	t.Run("can place an open trade request", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		openPrice := 1.5

		tr, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr)
		assert.NoError(t, err)

		assert.Equal(t, TradeTypeBuy, tr.Type)
		assert.Equal(t, symbol, tr.Symbol)
		assert.Equal(t, timeframe, tr.Timeframe)
		assert.Equal(t, timestamp, tr.Timestamp)
		assert.Equal(t, openPrice, tr.RequestedPrice)
		assert.Equal(t, openPrice, tr.ExecutedPrice)
	})

	t.Run("can place a sell order", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, Down, balance/2.0, newDownPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		assert.Len(t, *account.GetTrades(), 0)

		openPrice := 2.0

		tr, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr)
		assert.NoError(t, err)

		assert.Equal(t, TradeTypeSell, tr.Type)
		assert.Equal(t, symbol, tr.Symbol)
		assert.Equal(t, timeframe, tr.Timeframe)
		assert.Equal(t, timestamp, tr.Timestamp)
		assert.Equal(t, openPrice, tr.RequestedPrice)
		assert.Equal(t, openPrice, tr.ExecutedPrice)
	})

	t.Run("able to place trade in another band when original band is full", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		trade1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade1)
		assert.NoError(t, err)

		trade2, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade2)
		assert.NoError(t, err)

		trade3, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade3)
		assert.NoError(t, err)

		_, _, err = strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.ErrorIs(t, err, NoRemainingRiskAvailableErr)

		_, _, err = strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.ErrorIs(t, err, NoRemainingRiskAvailableErr)

		trade6, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 3.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade6)
		assert.NoError(t, err)
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
				Price:             2.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0.0,
			},
		}

		requestedPrice := 1.5

		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		trade1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.NoError(t, err)
		t1Result, err := strategy.AutoExecuteTrade(trade1)
		assert.NoError(t, err)

		trade2, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade2)
		assert.NoError(t, err)

		_, _, err = strategy.NewOpenTrade(id, timeframe, timestamp, requestedPrice)
		assert.ErrorIs(t, err, NoRemainingRiskAvailableErr)

		trade4, _, err := strategy.NewCloseTrades(id, timeframe, timestamp, requestedPrice, t1Result.PriceLevelIndex, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade4)
		assert.NoError(t, err)
	})

	t.Run("able to place additional trades in bands once previous trade is closed", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		curPrice := 1.5
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		tradesRemaining, side := strategy.TradesRemaining(curPrice)
		assert.Equal(t, 3, tradesRemaining)
		assert.Equal(t, side, TradeTypeNone)

		trade1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade1)
		assert.NoError(t, err)

		trade2, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade2)
		assert.NoError(t, err)

		trade3, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade3)
		assert.NoError(t, err)

		tradesRemaining, side = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 0, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)

		trade4, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, curPrice, 1.0, 1, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade4)
		assert.ErrorIs(t, err, MaxTradesPerPriceLevelErr)

		trade5, _, err := NewCloseTrade(id, []*Trade{trade1, trade2, trade3}, timeframe, timestamp, curPrice, 2.5, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade5)
		assert.NoError(t, err)

		tradesRemaining, side = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 2, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)
	})

	t.Run("able to close a trade outside of price bands", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		tr1Volume := 1.0
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, tr1Volume, 1.0, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr1ClosePrc := 10.5
		closeTr, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, -tr1Volume, nil)
		assert.NoError(t, err)

		assert.Equal(t, TradeTypeClose, closeTr.Type)
		assert.Equal(t, -tr1Volume, closeTr.RequestedVolume)
		assert.Equal(t, tr1ClosePrc, closeTr.RequestedPrice)

		closeTr.Execute(tr1ClosePrc, -tr1Volume)
		assert.Equal(t, -tr1Volume, closeTr.ExecutedVolume)
		assert.Equal(t, tr1ClosePrc, closeTr.ExecutedPrice)
	})

	t.Run("closing trades must have close percentage", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		tr1Volume := 1.0
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, tr1Volume, 1.0, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr1ClosePrc := 10.5
		_, _, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, -tr1Volume-0.001, nil)
		assert.ErrorIs(t, err, DuplicateCloseTradeErr)
	})

	t.Run("closing one half of a trade twice increases the number of trades allowed by one", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		curPrice := 1.5
		tradesRemaining, _ := strategy.TradesRemaining(curPrice)
		assert.Equal(t, 3, tradesRemaining)

		trVolume := 1.0
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, trVolume, 1.0, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, timestamp, 1.5, trVolume, 1.0, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		tradesRemaining, _ = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)

		tr1ClosePrc := 10.5
		tr3, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, tr1ClosePrc, trVolume/2.0, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		tradesRemaining, _ = strategy.TradesRemaining(curPrice)
		assert.Equal(t, 1, tradesRemaining)
	})

	t.Run("volume increases in a specific band as winners increase", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		trVolume := 1.0
		tr1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, 1.9, trVolume, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		tr3, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		assert.Greater(t, tr3.ExecutedVolume, tr1.ExecutedVolume)
	})

	t.Run("volume decreases in a specific band as losers increase", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newUpPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		tr1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, timestamp, 1.2, tr1.ExecutedVolume, nil)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		tr3, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		assert.Less(t, tr3.ExecutedVolume, tr1.ExecutedVolume)
	})
}

func TestUpdate(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	balance := 10000.00
	symbol := "symbol"
	name := "Test Placing Trades"
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	direction := Up

	timeframe := new(int)
	*timeframe = 5

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

		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, priceLevel, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		closeReq := account.checkSL(Tick{Price: 1.5})
		assert.Nil(t, closeReq)

		t0, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, 1.5)
		strategy.AutoExecuteTrade(t0)
		assert.NoError(t, err)

		closeReq = account.checkSL(Tick{Price: band1SL + 0.2})
		assert.Nil(t, closeReq)

		closeReq = account.checkSL(Tick{Price: band1SL})
		assert.NotNil(t, closeReq)
		assert.Equal(t, 1, len(closeReq))
		assert.Equal(t, 0, closeReq[0].PriceLevelIndex)
		assert.Equal(t, 1.0, closeReq[0].Percent)
	})

	t.Run("errors account needs to be closed due to stop out with up strategy", func(t *testing.T) {
		curPrice := 100000.0
		stopLoss := 0.0000001
		priceLevels := []*PriceLevel{
			{
				Price:             curPrice,
				StopLoss:          stopLoss,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             curPrice + 5000.0,
				StopLoss:          stopLoss,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             curPrice + 10000.0,
				AllocationPercent: 0,
			},
		}

		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, Up, balance, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		maxLoss := balance

		trade1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, curPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade1)
		assert.NoError(t, err)

		trade2, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, curPrice+5000.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade2)
		assert.NoError(t, err)

		tick := Tick{Price: curPrice}
		closeReq, err := account.CheckStopOut(tick)
		assert.NoError(t, err)
		assert.Nil(t, closeReq)

		totalVol := trade1.ExecutedVolume + trade2.ExecutedVolume
		vwap := (trade1.ExecutedPrice * (trade1.ExecutedVolume / totalVol)) + (trade2.ExecutedPrice * (trade2.ExecutedVolume / totalVol))
		stopOutPrice := ((vwap * totalVol) - maxLoss) / totalVol

		tick = Tick{Price: stopOutPrice}
		closeReq, err = account.CheckStopOut(tick)
		assert.NoError(t, err)
		assert.NotNil(t, closeReq)
		assert.Len(t, closeReq, 2)
		assert.Equal(t, 0, closeReq[0].PriceLevelIndex)
		assert.Equal(t, "stop out", closeReq[0].Reason)
		assert.Equal(t, 1.0, closeReq[0].Percent)
		assert.Equal(t, 1, closeReq[1].PriceLevelIndex)
		assert.Equal(t, "stop out", closeReq[1].Reason)
		assert.Equal(t, 1.0, closeReq[1].Percent)
	})

	t.Run("errors when stop out triggered with down strategy", func(t *testing.T) {
		openPrice := 100000.0
		stopLoss := openPrice * 2
		priceLevels := []*PriceLevel{
			{
				Price:             openPrice,
				AllocationPercent: 0,
			},
			{
				MaxNoOfTrades:     2,
				Price:             openPrice + 10000.0,
				StopLoss:          stopLoss,
				AllocationPercent: 1,
			},
		}

		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, Down, balance, priceLevels, account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		maxLoss := balance

		trade1, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade1)
		assert.NoError(t, err)

		trade2, _, err := strategy.NewOpenTrade(id, timeframe, timestamp, openPrice+5000.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(trade2)
		assert.NoError(t, err)

		closeReq, err := account.CheckStopOut(Tick{Price: openPrice + 5000.0})
		assert.NoError(t, err)
		assert.Nil(t, closeReq)

		vwap, vol, _ := strategy.GetTrades().GetTradeStatsItems()
		stopOutPrice := float64(vwap) - (maxLoss / float64(vol))

		closeReq, err = account.CheckStopOut(Tick{Price: stopOutPrice})
		assert.NoError(t, err)
		assert.NotNil(t, closeReq)
		assert.Len(t, closeReq, 1)

		_, closeReqVol, _ := closeReq[0].Strategy.GetTrades().GetTradeStatsItems()
		assert.Equal(t, float64(vol), float64(closeReqVol)*closeReq[0].Percent)
		assert.Equal(t, "stop out", closeReq[0].Reason)
	})
}

func TestTradeValidation(t *testing.T) {
	name := "TestTradeValidation"
	balance := 1000.00
	symbol := "btcusd"
	direction := Up
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	timestamp := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)

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
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance, newPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		_, _, err = strategy.NewOpenTrade(id, nil, timestamp, 0.5)
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
	})

	t.Run("errors if checking to placing a trade outside of range", func(t *testing.T) {
		df := NewDatafeed(ManualDatafeed)
		account, err := NewAccount(name, balance, df)
		assert.NoError(t, err)

		strategy, err := NewStrategyDeprecated(name, symbol, direction, balance/2.0, newPriceLevels(), account)
		assert.NoError(t, err)

		err = account.AddStrategy(strategy)
		assert.NoError(t, err)

		// success case
		trade, _, err := strategy.NewOpenTrade(id, nil, timestamp, 1.5)
		assert.NoError(t, err)
		assert.NotNil(t, trade)

		// failure case
		trade, _, err = strategy.NewOpenTrade(id, nil, timestamp, 11.0)
		assert.ErrorIs(t, err, PriceOutsideLimitsErr)
		assert.Nil(t, trade)
	})
}
