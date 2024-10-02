package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestFeed(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	t1_appl := startTime
	t2_appl := startTime.Add(5 * time.Second)
	t3_appl := startTime.Add(10 * time.Second)
	t1_goog := startTime
	t2_goog := startTime.Add(10 * time.Second)
	t3_goog := startTime.Add(20 * time.Second)

	t.Run("Returns previous candle until new candle is available", func(t *testing.T) {
		clock := NewClock(startTime, endTime)
		feed := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)
	})

	t.Run("Skip a candle", func(t *testing.T) {
		clock := NewClock(startTime, endTime)
		feed := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)

		stateChange, err := playground.Tick(20 * time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		candle, err = playground.GetCandle(symbol1)
		assert.NoError(t, err)
		assert.Equal(t, t3_appl, candle.Timestamp)
		assert.Equal(t, 15.0, candle.Close)
	})

	t.Run("Returns new candle/s on state changes", func(t *testing.T) {
		clock := NewClock(startTime, endTime)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, []time.Time{t1_goog, t2_goog, t3_goog}, []float64{0.0, 100.0, 200.0})

		playground, err := NewPlaygroundMultipleFeeds(1000.0, clock, feed1, feed2)
		assert.NoError(t, err)

		// new APPL candle, but not GOOG
		stateChange, err := playground.Tick(5 * time.Second)
		assert.NoError(t, err)
		assert.Len(t, stateChange.NewCandles, 1)
		assert.Equal(t, symbol1, stateChange.NewCandles[0].Symbol)
		assert.Equal(t, t2_appl, stateChange.NewCandles[0].Candle.Timestamp)
		assert.Equal(t, 10.0, stateChange.NewCandles[0].Candle.Close)

		// new APPL and GOOG candle
		stateChange, err = playground.Tick(5 * time.Second)
		assert.NoError(t, err)
		assert.Len(t, stateChange.NewCandles, 2)
		assert.Equal(t, symbol1, stateChange.NewCandles[0].Symbol)
		assert.Equal(t, t3_appl, stateChange.NewCandles[0].Candle.Timestamp)
		assert.Equal(t, 15.0, stateChange.NewCandles[0].Candle.Close)
		assert.Equal(t, symbol2, stateChange.NewCandles[1].Symbol)
		assert.Equal(t, t2_goog, stateChange.NewCandles[1].Candle.Timestamp)
		assert.Equal(t, 100.0, stateChange.NewCandles[1].Candle.Close)

		// no new candle
		stateChange, err = playground.Tick(5 * time.Second)
		assert.NoError(t, err)
		assert.Len(t, stateChange.NewCandles, 0)
	})

	t.Run("GetCandle returns the first candle until Tick is called", func(t *testing.T) {
		clock := NewClock(startTime, endTime)
		feed := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{startTime, startTime.Add(5), startTime.Add(10)}, []float64{0.0, 10.0, 15.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1)
		assert.NoError(t, err)
		assert.Equal(t, startTime, candle.Timestamp)
		assert.Equal(t, 0.0, candle.Close)
	})
}

func TestClock(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)

	t.Run("Backtester is complete", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")

		clock := NewClock(startTime, endTime)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{startTime, startTime.Add(1), startTime.Add(2)}, []float64{100.0, 100.0, 100.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)

		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		stateChange, err = playground.Tick(time.Hour)

		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.True(t, stateChange.IsBacktestComplete)

		// no longer able to tick after backtest is complete
		stateChange, err = playground.Tick(time.Second)
		assert.Error(t, err)
		assert.Nil(t, stateChange)
	})

	t.Run("Clock remains finished", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		clock.Add(60 * time.Minute)

		assert.True(t, clock.IsExpired())

		clock.Add(time.Minute)

		assert.True(t, clock.IsExpired())
	})

	t.Run("Clock is finished at end time", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		assert.False(t, clock.IsExpired())

		clock.Add(59 * time.Minute)

		assert.False(t, clock.IsExpired())

		clock.Add(time.Minute)

		assert.True(t, clock.IsExpired())
	})
}

func TestBalance(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)

	t.Run("GetAccountBalance", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		playground, err := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed(symbol, nil, nil))

		assert.NoError(t, err)

		initialBalance := playground.GetBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})

	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		now := startTime

		prices := []float64{0, 100.0, 115.0}

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{startTime, startTime.Add(time.Second), startTime.Add(2 * time.Second)}, prices)
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1000.0, playground.GetBalance())

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1150.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - increase and decrease after profitable and unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		t1 := startTime
		t2 := startTime.Add(time.Second)
		t3 := startTime.Add(2 * time.Second)
		t4 := startTime.Add(3 * time.Second)
		t5 := startTime.Add(4 * time.Second)
		t6 := startTime.Add(5 * time.Second)

		prices := []float64{0.0, 100.0, 110.0, 90.0, 100.0, 90.0}

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4, t5, t6}, prices)
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		// open 1st order
		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1000.0, playground.GetBalance())

		// open 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1000.0, playground.GetBalance())

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 700.0, playground.GetBalance())

		// open 3rd order
		order4 := NewBacktesterOrder(4, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 700.0, playground.GetBalance())

		// close order
		order5 := NewBacktesterOrder(5, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order5)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 800.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - decrease after unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		t1 := startTime
		t2 := startTime.Add(time.Second)
		t3 := startTime.Add(2 * time.Second)
		prices := []float64{0, 100.0, 85.0}

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1000.0, playground.GetBalance())

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 850.0, playground.GetBalance())
	})
}

func TestPlaceOrder(t *testing.T) {
	t.Run("Cannot place order if data feed is not imported", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{100.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("GOOG"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})
}

func TestPositions(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)

	t.Run("GetPosition", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		playground, err := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed(symbol, nil, nil))
		assert.NoError(t, err)
		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis - multiple orders - same direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 0, 3, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 0, 4, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4}, []float64{100.0, 200.0, 300.0, 400.0})

		now := startTime

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		// 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, symbol, BacktesterOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)

		// 3rd order - original direction
		order4 := NewBacktesterOrder(4, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis - multiple orders - reverse direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 0, 3, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 0, 4, 0, time.UTC)
		t5 := time.Date(2021, time.January, 1, 0, 0, 5, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4, t5}, []float64{100.0, 200.0, 300.0, 400.0, 500.0})

		now := startTime

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(symbol)
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		// 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, -20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, symbol, BacktesterOrderSideBuyToCover, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)

		// 3rd order - reverse direction
		order4 := NewBacktesterOrder(4, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)

		// 4th order - reverse direction
		order5 := NewBacktesterOrder(5, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order5)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 450.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2}, []float64{600.0, 300.0})

		now := startTime

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 600.0, position.CostBasis)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 400.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after buy", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{1000.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 1000.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after sell", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after sell short", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after buy to cover", func(t *testing.T) {
		clock := NewClock(startTime, endTime)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})
}

func TestOrders(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime)

	now := startTime

	feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{100.0})

	t.Run("PlaceOrder - market", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		orders := playground.GetOrders()
		assert.Len(t, orders, 1)
		assert.Equal(t, BacktesterOrderStatusOpen, orders[0].GetStatus())

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		orders = playground.GetOrders()
		assert.Len(t, orders, 1)
		assert.Equal(t, BacktesterOrderStatusFilled, orders[0].GetStatus())
	})

	t.Run("PlaceOrder - cannot buy after short sell", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot buy to cover when not short", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot sell before buy", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid class", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClass("invalid"), now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid price", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		price := float64(0)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, &price, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid id", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		id := uint(1)

		order1 := NewBacktesterOrder(id, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(id, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)

	prices := []float64{100.0, 105.0, 110.0}
	t1 := startTime
	t2 := startTime.Add(time.Second)
	t3 := startTime.Add(2 * time.Second)

	t.Run("Short order rejected if long position exists", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)
		clock := NewClock(startTime, endTime)

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		now := startTime

		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.Len(t, stateChange.NewTrades, 1)

		orders := playground.GetOrders()
		assert.Len(t, orders, 2)
		assert.Equal(t, BacktesterOrderStatusFilled, orders[0].GetStatus())
		assert.Equal(t, BacktesterOrderStatusRejected, orders[1].GetStatus())
	})

	t.Run("Tick", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)
		clock := NewClock(startTime, endTime)

		now := startTime
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		quantity := 10.0
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, stateChange.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, prices[1], stateChange.NewTrades[0].Price)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, stateChange.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, prices[2], stateChange.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
