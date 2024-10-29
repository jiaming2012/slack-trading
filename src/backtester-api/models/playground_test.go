package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestLiquidation(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)

	t.Run("Buy and sell orders - multiple liquidations", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Second)
		t2 := startTime.Add(10 * time.Second)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{startTime, t1, t2}, []float64{0.0, 10.0, 10.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, []time.Time{startTime, t1, t2}, []float64{0.0, 100.0, 500.0})

		balance := 1000.0
		playground, err := NewPlaygroundMultipleFeeds(balance, clock, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, startTime, symbol1, BacktesterOrderSideBuy, 30, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, Equity, startTime, symbol2, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err := playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		positions := playground.GetPositions()
		assert.Len(t, positions, 2)

		delta, err = playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.Events, 1)
		assert.Equal(t, TickDeltaEventTypeLiquidation, delta.Events[0].Type)
		assert.NotNil(t, delta.Events[0].LiquidationEvent)

		liquidationOrders := delta.Events[0].LiquidationEvent.OrdersPlaced
		assert.Len(t, liquidationOrders, 2)
		assert.Equal(t, symbol2, liquidationOrders[0].Symbol)
		assert.Equal(t, BacktesterOrderStatusFilled, liquidationOrders[0].GetStatus())
		assert.Contains(t, liquidationOrders[0].Tag, "liquidation - equity @")
		assert.Contains(t, liquidationOrders[0].Tag, "maintenance margin @")
		assert.Equal(t, symbol1, liquidationOrders[1].Symbol)
		assert.Equal(t, BacktesterOrderStatusFilled, liquidationOrders[1].GetStatus())
		assert.Contains(t, liquidationOrders[1].Tag, "liquidation - equity @")
		assert.Contains(t, liquidationOrders[1].Tag, "maintenance margin @")

		assert.Len(t, liquidationOrders[0].Trades, 1)
		assert.Equal(t, 10.0, liquidationOrders[0].Trades[0].Quantity)
		assert.Equal(t, 500.0, liquidationOrders[0].Trades[0].Price)

		assert.Len(t, liquidationOrders[1].Trades, 1)
		assert.Equal(t, -30.0, liquidationOrders[1].Trades[0].Quantity)
		assert.Equal(t, 10.0, liquidationOrders[1].Trades[0].Price)

		positions = playground.GetPositions()
		assert.Len(t, positions, 0)
	})

	t.Run("Sell orders - single liquidation", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Second)
		t2 := startTime.Add(10 * time.Second)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{startTime, t1, t2}, []float64{0.0, 10.0, 10.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, []time.Time{startTime, t1, t2}, []float64{0.0, 100.0, 200.0})

		balance := 1000.0
		playground, err := NewPlaygroundMultipleFeeds(balance, clock, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, startTime, symbol1, BacktesterOrderSideSellShort, 30, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, Equity, startTime, symbol2, BacktesterOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err := playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		positions := playground.GetPositions()
		assert.Len(t, positions, 2)

		delta, err = playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.Events, 1)
		assert.Equal(t, TickDeltaEventTypeLiquidation, delta.Events[0].Type)
		assert.NotNil(t, delta.Events[0].LiquidationEvent)

		liquidationOrders := delta.Events[0].LiquidationEvent.OrdersPlaced
		assert.Len(t, liquidationOrders, 1)
		assert.Equal(t, BacktesterOrderStatusFilled, liquidationOrders[0].GetStatus())
		assert.Contains(t, liquidationOrders[0].Tag, "liquidation - equity @")
		assert.Contains(t, liquidationOrders[0].Tag, "maintenance margin @")

		assert.Len(t, liquidationOrders[0].Trades, 1)
		assert.Equal(t, 5.0, liquidationOrders[0].Trades[0].Quantity)
		assert.Equal(t, 200.0, liquidationOrders[0].Trades[0].Price)

		positions = playground.GetPositions()
		assert.Len(t, positions, 1)
	})

	t.Run("Buy orders - no liquidation", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Second)
		t2 := startTime.Add(10 * time.Second)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{startTime, t1, t2}, []float64{0.0, 10.0, 0.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, []time.Time{startTime, t1, t2}, []float64{0.0, 100.0, 0.0})

		balance := 1000.0
		playground, err := NewPlaygroundMultipleFeeds(balance, clock, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, startTime, symbol1, BacktesterOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, Equity, startTime, symbol2, BacktesterOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err := playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		delta, err = playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Nil(t, delta.Events)
	})
}

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
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)
	})

	t.Run("Skip a candle", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)

		delta, err := playground.Tick(20 * time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		candle, err = playground.GetCandle(symbol1)
		assert.NoError(t, err)
		assert.Equal(t, t3_appl, candle.Timestamp)
		assert.Equal(t, 15.0, candle.Close)
	})

	t.Run("Returns new candle/s on state changes", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, []time.Time{t1_goog, t2_goog, t3_goog}, []float64{0.0, 100.0, 200.0})

		balance := 1000000.0
		playground, err := NewPlaygroundMultipleFeeds(balance, clock, feed1, feed2)
		assert.NoError(t, err)

		// new APPL candle, but not GOOG
		delta, err := playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 1)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t2_appl, delta.NewCandles[0].Candle.Timestamp)
		assert.Equal(t, 10.0, delta.NewCandles[0].Candle.Close)

		// new APPL and GOOG candle
		delta, err = playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 2)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t3_appl, delta.NewCandles[0].Candle.Timestamp)
		assert.Equal(t, 15.0, delta.NewCandles[0].Candle.Close)
		assert.Equal(t, symbol2, delta.NewCandles[1].Symbol)
		assert.Equal(t, t2_goog, delta.NewCandles[1].Candle.Timestamp)
		assert.Equal(t, 100.0, delta.NewCandles[1].Candle.Close)

		// no new candle
		delta, err = playground.Tick(5 * time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 0)
	})

	t.Run("GetCandle returns the first candle until Tick is called", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
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

		clock := NewClock(startTime, endTime, nil)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{startTime, startTime.Add(1), startTime.Add(2)}, []float64{100.0, 100.0, 100.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)

		assert.NoError(t, err)
		assert.NotNil(t, delta)

		delta, err = playground.Tick(time.Hour, false)

		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.True(t, delta.IsBacktestComplete)

		// no longer able to tick after backtest is complete
		delta, err = playground.Tick(time.Second, false)
		assert.Error(t, err)
		assert.Nil(t, delta)
	})

	t.Run("Clock remains finished", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		clock.Add(60 * time.Minute)

		assert.True(t, clock.IsExpired())

		clock.Add(time.Minute)

		assert.True(t, clock.IsExpired())
	})

	t.Run("Clock is finished at end time", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

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
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed(symbol, nil, nil))

		assert.NoError(t, err)

		initialBalance := playground.GetBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})

	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		prices := []float64{0.0, 100.0, 115.0}

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{startTime, startTime.Add(time.Second), startTime.Add(2 * time.Second)}, prices)
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 2, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 100.0, delta.NewTrades[0].Price)
		assert.Equal(t, 1000.0, playground.GetBalance())

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 2, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 115.0, delta.NewTrades[0].Price)
		assert.Equal(t, 1030.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - increase and decrease after profitable and unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := startTime
		t2 := startTime.Add(time.Second)
		t3 := startTime.Add(2 * time.Second)
		t4 := startTime.Add(3 * time.Second)
		t5 := startTime.Add(4 * time.Second)
		t6 := startTime.Add(5 * time.Second)

		prices := []float64{0.0, 100.0, 110.0, 90.0, 100.0, 90.0}

		now := startTime

		balance := 100000.0
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4, t5, t6}, prices)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		// open 1st order
		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		// open 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-300.0, playground.GetBalance())

		// open 3rd order
		order4 := NewBacktesterOrder(4, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-300.0, playground.GetBalance())

		// close order
		order5 := NewBacktesterOrder(5, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order5)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-200.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - decrease after unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := startTime
		t2 := startTime.Add(time.Second)
		t3 := startTime.Add(2 * time.Second)
		prices := []float64{0, 100.0, 85.0}

		now := startTime

		balance := 1000000.0
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-150.0, playground.GetBalance())
	})
}

func TestPlaceOrder(t *testing.T) {
	t.Run("Cannot place order if data feed is not imported", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

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
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed(symbol, nil, nil))
		assert.NoError(t, err)
		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
		// assert.Nil(t, position.OpenTrades)
	})

	t.Run("GetPosition - open trades, long", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		balance := 1000000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade
		position1 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		// assert.Len(t, position1.Quantity, 1)
		// assert.Equal(t, 10.0, position1.OpenTrades[0].Quantity)
		assert.Equal(t, 10.0, position1.Quantity)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade volume decreased
		position2 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		// assert.Len(t, position2.OpenTrades, 2)
		// assert.Equal(t, 10.0, position2.OpenTrades[0].Quantity)
		// assert.Equal(t, -5.0, position2.OpenTrades[1].Quantity)
		assert.Equal(t, 5.0, position2.Quantity)
	})

	t.Run("GetPosition - open trades, short", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		balance := 1000000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")

		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade
		position1 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		// assert.Len(t, position1.OpenTrades, 1)
		// assert.Equal(t, -10.0, position1.OpenTrades[0].Quantity)
		assert.Equal(t, -10.0, position1.Quantity)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade volume decreased
		position2 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		// assert.Len(t, position2.OpenTrades, 2)
		// assert.Equal(t, -10.0, position2.OpenTrades[0].Quantity)
		// assert.Equal(t, 5.0, position2.OpenTrades[1].Quantity)
		assert.Equal(t, -5.0, position2.Quantity)
	})

	t.Run("GetPosition - average cost basis - multiple orders - same direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 0, 3, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 0, 4, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4}, []float64{100.0, 200.0, 300.0, 400.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)

		// 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, 10.0, position.OpenTrades[1].Quantity)

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, symbol, BacktesterOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 0)

		// 3rd order - original direction
		order4 := NewBacktesterOrder(4, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
	})

	t.Run("GetPosition - average cost basis - multiple orders - reverse direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 0, 3, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 0, 4, 0, time.UTC)
		t5 := time.Date(2021, time.January, 1, 0, 0, 5, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3, t4, t5}, []float64{100.0, 200.0, 300.0, 400.0, 500.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)

		// 2nd order
		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, -20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, -10.0, position.OpenTrades[1].Quantity)

		// close orders
		order3 := NewBacktesterOrder(3, Equity, now, symbol, BacktesterOrderSideBuyToCover, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order3)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 0)

		// 3rd order - reverse direction
		order4 := NewBacktesterOrder(4, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order4)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)

		// 4th order - reverse direction
		order5 := NewBacktesterOrder(5, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order5)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 450.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, 10.0, position.OpenTrades[1].Quantity)
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2}, []float64{600.0, 300.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 600.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 400.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, 20.0, position.OpenTrades[1].Quantity)
	})

	t.Run("GetPosition - Quantity increase after buy", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{1000.0})

		balance := 100000.0
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 1000.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
	})

	t.Run("GetPosition - Quantity decrease after sell", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlayground(1000000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, -5.0, position.OpenTrades[1].Quantity)
	})

	t.Run("GetPosition - Quantity increase after sell short", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlayground(100000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)

		order = NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, -5.0, position.OpenTrades[1].Quantity)
	})

	t.Run("GetPosition - Quantity decrease after buy to cover", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlayground(100000.0, clock, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)

		order2 := NewBacktesterOrder(2, Equity, now, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, 5.0, position.OpenTrades[1].Quantity)
	})
}

func TestFreeMargin(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 2, 0, 0, time.UTC)

	now := startTime

	feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{startTime, t1, endTime}, []float64{100.0, 200.0, 250.0})

	t.Run("No positions", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, balance*2, freeMargin)
	})

	t.Run("Adjust with unrealized PnL", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		// place order
		order := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		_, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, 1000.0, freeMargin)

		// move price: tick went from 100 to 200
		_, err = playground.Tick(time.Hour, false)
		assert.NoError(t, err)

		freeMargin = playground.GetFreeMargin()
		assert.Equal(t, 3000.0, freeMargin)
	})

	t.Run("Long position reduces free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 100.0, delta.NewTrades[0].Price)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, 1900.0, freeMargin)
	})

	t.Run("Short position reduces free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideSellShort, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		_, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, 1850.0, freeMargin)
	})

	t.Run("Trade rejected if insufficient free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlayground(balance, clock, feed)
		assert.NoError(t, err)

		// place order equal to free margin
		order := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 19, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)

		assert.Len(t, delta.InvalidOrders, 0)

		// place order above free margin
		order = NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)

		assert.Len(t, delta.InvalidOrders, 1)
		assert.Equal(t, BacktesterOrderStatusRejected, delta.InvalidOrders[0].Status)
		assert.NotNil(t, delta.InvalidOrders[0].RejectReason)
		assert.Equal(t, ErrInsufficientFreeMargin.Error(), *delta.InvalidOrders[0].RejectReason)
	})
}

func TestOrders(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime, nil)

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

		_, err = playground.Tick(time.Second, false)
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

		_, err = playground.Tick(time.Second, false)
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
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		now := startTime

		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 1)

		orders := playground.GetOrders()
		assert.Len(t, orders, 2)
		assert.Equal(t, BacktesterOrderStatusFilled, orders[0].GetStatus())
		assert.Equal(t, BacktesterOrderStatusRejected, orders[1].GetStatus())
	})

	t.Run("Tick", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)
		clock := NewClock(startTime, endTime, nil)

		now := startTime
		playground, err := NewPlayground(100000.0, clock, feed)
		assert.NoError(t, err)
		quantity := 10.0
		order1 := NewBacktesterOrder(1, Equity, now, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order1)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[1], delta.NewTrades[0].Price)

		order2 := NewBacktesterOrder(2, Equity, now, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		err = playground.PlaceOrder(order2)
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Second, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[2], delta.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
