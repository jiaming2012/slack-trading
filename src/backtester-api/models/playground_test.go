package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestOpenOrdersCache(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator

	createPlayground := func() (*Playground, error) {
		period := time.Minute
		endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)

		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(time.Minute)
		t2 := startTime.Add(2 * time.Minute)

		feed1 := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{startTime, t1, t2}, []float64{10.0, 20.0, 30.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, period, []time.Time{startTime, t1, t2}, []float64{110.0, 120.0, 130.0})

		balance := 1000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed1, feed2)
		return playground, err
	}

	t.Run("Open and close a trade", func(t *testing.T) {
		playground, err := createPlayground()
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, startTime, symbol1, TradierOrderSideBuy, 30, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 1)

		// check the cache
		symbol1Orders := playground.GetOpenOrders(symbol1)
		assert.Len(t, symbol1Orders, 1)

		symbol2Orders := playground.GetOpenOrders(symbol2)
		assert.Len(t, symbol2Orders, 0)
	})

	t.Run("Initial state", func(t *testing.T) {
		playground, err := createPlayground()
		assert.NoError(t, err)

		symbol1Orders := playground.GetOpenOrders(symbol1)
		assert.Len(t, symbol1Orders, 0)

		symbol2Orders := playground.GetOpenOrders(symbol2)
		assert.Len(t, symbol2Orders, 0)
	})
}

func TestLiquidation(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	period := time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator

	t.Run("Buy and sell orders - multiple liquidations", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Minute)
		// t2 := startTime.Add(10 * time.Minute)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{startTime, t1}, []float64{10.0, 10.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, period, []time.Time{startTime, t1}, []float64{100.0, 500.0})

		balance := 1000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, startTime, symbol1, TradierOrderSideBuy, 30, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, startTime, symbol2, TradierOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		positions := playground.GetPositions()
		assert.Len(t, positions, 2)

		delta, err = playground.Tick(5*time.Minute, false)
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
		assert.Equal(t, 5.0, liquidationOrders[0].Trades[0].Quantity)
		assert.Equal(t, 500.0, liquidationOrders[0].Trades[0].Price)

		assert.Len(t, liquidationOrders[1].Trades, 1)
		assert.Equal(t, -30.0, liquidationOrders[1].Trades[0].Quantity)
		assert.Equal(t, 10.0, liquidationOrders[1].Trades[0].Price)

		positions = playground.GetPositions()
		assert.Len(t, positions, 0)
	})

	t.Run("Sell orders - single liquidation", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Minute)
		t2 := startTime.Add(10 * time.Minute)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{startTime, t1, t2}, []float64{0.0, 10.0, 10.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, period, []time.Time{startTime, t1, t2}, []float64{0.0, 100.0, 200.0})

		balance := 1000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, startTime, symbol1, TradierOrderSideSellShort, 30, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, startTime, symbol2, TradierOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		positions := playground.GetPositions()
		assert.Len(t, positions, 2)

		delta, err = playground.Tick(5*time.Minute, false)
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
		t1 := startTime.Add(5 * time.Minute)
		t2 := startTime.Add(10 * time.Minute)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{startTime, t1, t2}, []float64{0.0, 10.0, 0.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, period, []time.Time{startTime, t1, t2}, []float64{0.0, 100.0, 0.0})

		balance := 1000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed1, feed2)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, startTime, symbol1, TradierOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, startTime, symbol2, TradierOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 2)
		assert.Equal(t, symbol1, delta.NewTrades[0].Symbol)
		assert.Equal(t, 10.0, delta.NewTrades[0].Price)
		assert.Equal(t, symbol2, delta.NewTrades[1].Symbol)
		assert.Equal(t, 100.0, delta.NewTrades[1].Price)

		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Nil(t, delta.Events)
	})
}

func TestFeed(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	period := time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	t1_appl := startTime
	t2_appl := startTime.Add(5 * time.Minute)
	t3_appl := startTime.Add(10 * time.Minute)
	t1_goog := startTime
	t2_goog := startTime.Add(10 * time.Minute)
	t3_goog := startTime.Add(20 * time.Minute)
	env := PlaygroundEnvironmentSimulator

	t.Run("Returns previous candle until new candle is available", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)
	})

	t.Run("Skip a candle", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)

		assert.Equal(t, startTime, candle.Timestamp)

		delta, err := playground.Tick(20*time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		candle, err = playground.GetCandle(symbol1, period)
		assert.NoError(t, err)
		assert.Equal(t, t3_appl, candle.Timestamp)
		assert.Equal(t, 15.0, candle.Close)
	})

	t.Run("Returns new candle/s on state changes", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed1 := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{t1_appl, t2_appl, t3_appl}, []float64{0.0, 10.0, 15.0})
		feed2 := mock.NewMockBacktesterDataFeed(symbol2, period, []time.Time{t1_goog, t2_goog, t3_goog}, []float64{0.0, 100.0, 200.0})

		balance := 1000000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed1, feed2)
		assert.NoError(t, err)

		// new APPL candle, but not GOOG
		delta, err := playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 1)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t2_appl, delta.NewCandles[0].Bar.Timestamp)
		assert.Equal(t, 10.0, delta.NewCandles[0].Bar.Close)

		// new APPL and GOOG candle
		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 2)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t3_appl, delta.NewCandles[0].Bar.Timestamp)
		assert.Equal(t, 15.0, delta.NewCandles[0].Bar.Close)
		assert.Equal(t, symbol2, delta.NewCandles[1].Symbol)
		assert.Equal(t, t2_goog, delta.NewCandles[1].Bar.Timestamp)
		assert.Equal(t, 100.0, delta.NewCandles[1].Bar.Close)

		// no new candle
		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 0)
	})

	t.Run("GetCandle returns the first candle until Tick is called", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol1, period, []time.Time{startTime, startTime.Add(5), startTime.Add(10)}, []float64{0.0, 10.0, 15.0})

		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)
		assert.Equal(t, startTime, candle.Timestamp)
		assert.Equal(t, 0.0, candle.Close)
	})
}

func TestClock(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	period := time.Minute
	env := PlaygroundEnvironmentSimulator

	t.Run("Backtester is complete", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")

		clock := NewClock(startTime, endTime, nil)

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{startTime, startTime.Add(1), startTime.Add(2)}, []float64{100.0, 100.0, 100.0})

		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)

		assert.NoError(t, err)
		assert.NotNil(t, delta)

		delta, err = playground.Tick(time.Hour, false)

		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.True(t, delta.IsBacktestComplete)

		// no longer able to tick after backtest is complete
		delta, err = playground.Tick(time.Minute, false)
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
	period := time.Minute
	env := PlaygroundEnvironmentSimulator

	t.Run("GetAccountBalance", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		feed := mock.NewMockBacktesterDataFeed(symbol, period, nil, nil)

		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)

		assert.NoError(t, err)

		initialBalance := playground.GetBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})

	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		prices := []float64{0.0, 100.0, 115.0}

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{startTime, startTime.Add(time.Minute), startTime.Add(2 * time.Minute)}, prices)
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 2, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 100.0, delta.NewTrades[0].Price)
		assert.Equal(t, 1000.0, playground.GetBalance())

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 2, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 115.0, delta.NewTrades[0].Price)
		assert.Equal(t, 1030.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - increase and decrease after profitable and unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := startTime
		t2 := startTime.Add(time.Minute)
		t3 := startTime.Add(2 * time.Minute)
		t4 := startTime.Add(3 * time.Minute)
		t5 := startTime.Add(4 * time.Minute)
		t6 := startTime.Add(5 * time.Minute)

		prices := []float64{0.0, 100.0, 110.0, 90.0, 100.0, 90.0}

		now := startTime

		balance := 100000.0
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3, t4, t5, t6}, prices)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// open 1st order
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		// open 2nd order
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		// close orders
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-300.0, playground.GetBalance())

		// open 3rd order
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-300.0, playground.GetBalance())

		// close order
		order5 := NewBacktesterOrder(5, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order5)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-200.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - decrease after unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := startTime
		t2 := startTime.Add(time.Minute)
		t3 := startTime.Add(2 * time.Minute)
		prices := []float64{0, 100.0, 85.0}

		now := startTime

		balance := 1000000.0
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3}, prices)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance, playground.GetBalance())

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Equal(t, balance-150.0, playground.GetBalance())
	})
}

func TestPlaceOrder(t *testing.T) {
	t.Run("Cannot place order if data feed is not imported", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")
		period := time.Minute
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)
		env := PlaygroundEnvironmentSimulator

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{100.0})

		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("GOOG"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})
}

func TestPositions(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator

	t.Run("GetPosition", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		feed := mock.NewMockBacktesterDataFeed(symbol, period, nil, nil)
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
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
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{250.0})
		balance := 1000000.0

		// Create a new playground
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// Place a buy order
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		// Tick the playground
		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade
		position1 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 10.0, position1.Quantity)

		// Place a sell order
		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade volume decreased
		position2 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 5.0, position2.Quantity)
	})

	t.Run("GetPosition - open trades, short", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{250.0})

		balance := 1000000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")

		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		// assert single open trade
		position1 := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		// assert.Len(t, position1.OpenTrades, 1)
		// assert.Equal(t, -10.0, position1.OpenTrades[0].Quantity)
		assert.Equal(t, -10.0, position1.Quantity)

		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3, t4}, []float64{100.0, 200.0, 300.0, 400.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)

		// 2nd order
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, 10.0, position.OpenTrades[1].Quantity)

		// close orders
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 0)

		// 3rd order - original direction
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3, t4, t5}, []float64{100.0, 200.0, 300.0, 400.0, 500.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		openOrders := playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 1)
		assert.Equal(t, order1, openOrders[0])

		// 2nd order
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, -20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		openOrders = playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 2)
		assert.ElementsMatch(t, openOrders, []*BacktesterOrder{order1, order2})

		// close orders
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuyToCover, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)

		orders := playground.GetOrders()
		assert.Len(t, orders, 3)
		assert.ElementsMatch(t, orders[2].Closes, []*BacktesterOrder{order1, order2})

		assert.Len(t, order3.Trades, 1)

		assert.Len(t, order1.ClosedBy, 1)
		assert.Equal(t, order3.Trades[0], order1.ClosedBy[0])

		assert.Len(t, order2.ClosedBy, 1)
		assert.Equal(t, order3.Trades[0], order2.ClosedBy[0])

		// 3rd order - reverse direction
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)

		openOrders = playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 1)
		assert.Equal(t, order4, openOrders[0])

		// 4th order - continue in same direction
		order5 := NewBacktesterOrder(5, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order5)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position = playground.GetPosition(symbol)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 450.0, position.CostBasis)

		openOrders = playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 2)
		assert.ElementsMatch(t, openOrders, []*BacktesterOrder{order4, order5})
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2}, []float64{600.0, 300.0})

		now := startTime

		balance := 1000000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 600.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{1000.0})

		balance := 100000.0
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlaygroundDeprecated(1000000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlaygroundDeprecated(100000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)

		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{250.0})

		now := startTime

		playground, err := NewPlaygroundDeprecated(100000.0, clock, env, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
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
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 2, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator

	now := startTime

	feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{startTime, t1, endTime}, []float64{100.0, 200.0, 250.0})

	t.Run("No positions", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, balance*2, freeMargin)
	})

	t.Run("Adjust with unrealized PnL", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// place order
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		_, err = playground.Tick(time.Minute, false)
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
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 100.0, delta.NewTrades[0].Price)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, 1900.0, freeMargin)
	})

	t.Run("Short position reduces free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		_, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		freeMargin := playground.GetFreeMargin()
		assert.Equal(t, 1850.0, freeMargin)
	})

	t.Run("Trade rejected if insufficient free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)
		playground, err := NewPlaygroundDeprecated(balance, clock, env, feed)
		assert.NoError(t, err)

		// place order equal to free margin
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 19, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		assert.Len(t, delta.InvalidOrders, 0)

		// place order above free margin
		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 1, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		assert.Len(t, delta.InvalidOrders, 1)
		assert.Equal(t, BacktesterOrderStatusRejected, delta.InvalidOrders[0].Status)
		assert.NotNil(t, delta.InvalidOrders[0].RejectReason)
		assert.Equal(t, ErrInsufficientFreeMargin.Error(), *delta.InvalidOrders[0].RejectReason)
	})
}

func TestOrders(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime, nil)
	env := PlaygroundEnvironmentSimulator

	now := startTime

	feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{endTime}, []float64{100.0})

	t.Run("PlaceOrder - market", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		orders := playground.GetOrders()
		assert.Len(t, orders, 1)
		assert.Equal(t, BacktesterOrderStatusOpen, orders[0].GetStatus())

		_, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		orders = playground.GetOrders()
		assert.Len(t, orders, 1)
		assert.Equal(t, BacktesterOrderStatusFilled, orders[0].GetStatus())
	})

	t.Run("PlaceOrder - cannot buy after short sell", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		_, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot buy to cover when not short", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot sell before buy", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid class", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClass("invalid"), now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid price", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)
		price := float64(0)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, &price, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid id", func(t *testing.T) {
		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		id := uint(1)

		order1 := NewBacktesterOrder(id, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(id, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator

	prices := []float64{100.0, 105.0, 110.0}
	t1 := startTime
	t2 := startTime.Add(time.Minute)
	t3 := startTime.Add(2 * time.Minute)

	t.Run("Short order rejected if long position exists", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3}, prices)
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlaygroundDeprecated(1000.0, clock, env, feed)
		assert.NoError(t, err)

		now := startTime

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewTrades, 1)

		orders := playground.GetOrders()
		assert.Len(t, orders, 2)
		assert.Equal(t, BacktesterOrderStatusFilled, orders[0].GetStatus())
		assert.Equal(t, BacktesterOrderStatusRejected, orders[1].GetStatus())
	})

	t.Run("Tick", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, period, []time.Time{t1, t2, t3}, prices)
		clock := NewClock(startTime, endTime, nil)

		now := startTime
		playground, err := NewPlaygroundDeprecated(100000.0, clock, env, feed)
		assert.NoError(t, err)
		quantity := 10.0
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[1], delta.NewTrades[0].Price)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[2], delta.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
