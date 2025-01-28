package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestOpenOrdersCache(t *testing.T) {
	symbol1 := eventmodels.StockSymbol("AAPL")
	symbol2 := eventmodels.StockSymbol("GOOG")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator
	source := eventmodels.CandleRepositorySource{
		Type: "test",
	}

	createPlayground := func() (*Playground, error) {
		period := time.Minute
		endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)

		clock := NewClock(startTime, endTime, nil)
		t_minus_1 := startTime.Add(-time.Minute)
		t1 := startTime.Add(time.Minute)
		t2 := startTime.Add(2 * time.Minute)

		feed1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t_minus_1,
				Close:     5.0,
			},
			{
				Timestamp: startTime,
				Close:     10.0,
			},
			{
				Timestamp: t1,
				Close:     20.0,
			},
			{
				Timestamp: t2,
				Close:     30.0,
			},
		}

		feed2 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t_minus_1,
				Close:     100.0,
			},
			{
				Timestamp: startTime,
				Close:     110.0,
			},
			{
				Timestamp: t1,
				Close:     120.0,
			},
			{
				Timestamp: t2,
				Close:     130.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, feed1, []string{}, nil, 0, source)
		assert.NoError(t, err)

		repo2, err := NewCandleRepository(symbol2, period, feed2, []string{}, nil, 0, source)
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1, repo2)
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

		candles1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     10.0,
			},
			{
				Timestamp: t1,
				Close:     10.0,
			},
		}

		candles2 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     500.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles1, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		repo2, err := NewCandleRepository(symbol2, period, candles2, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1, repo2)
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

		positions, err := playground.GetPositions()
		assert.NoError(t, err)
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

		positions, err = playground.GetPositions()
		assert.NoError(t, err)
		assert.Len(t, positions, 0)
	})

	t.Run("Sell orders - single liquidation", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime.Add(5 * time.Minute)

		candles1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     10.0,
			},
			{
				Timestamp: t1,
				Close:     10.0,
			},
		}

		candles2 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     200.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles1, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		repo2, err := NewCandleRepository(symbol2, period, candles2, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1, repo2)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, startTime, symbol1, TradierOrderSideSellShort, 25, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, startTime, symbol2, TradierOrderSideSellShort, 4, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
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

		positions, err := playground.GetPositions()
		assert.NoError(t, err)
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
		assert.Equal(t, 4.0, liquidationOrders[0].Trades[0].Quantity)
		assert.Equal(t, 200.0, liquidationOrders[0].Trades[0].Price)

		positions, err = playground.GetPositions()
		assert.NoError(t, err)

		assert.Len(t, positions, 1)
	})

	t.Run("Buy orders - no liquidation", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1 := startTime
		t2 := startTime.Add(5 * time.Minute)

		candles1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1,
				Close:     10.0,
			},
			{
				Timestamp: t2,
				Close:     0.0,
			},
		}

		candles2 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1,
				Close:     100.0,
			},
			{
				Timestamp: t2,
				Close:     0.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles1, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		repo2, err := NewCandleRepository(symbol2, period, candles2, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1, repo2)
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

	env := PlaygroundEnvironmentSimulator

	t.Run("Returns previous candle until new candle is available", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1_appl := startTime
		t2_appl := startTime.Add(5 * time.Minute)

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1_appl,
				Close:     10.0,
			},
			{
				Timestamp: t2_appl,
				Close:     15.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)

		assert.Equal(t, t1_appl, candle.Timestamp)
	})

	t.Run("Skip a candle", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1_appl := startTime
		t2_appl := startTime.Add(5 * time.Minute)
		t3_appl := startTime.Add(10 * time.Minute)

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1_appl,
				Close:     10.0,
			},
			{
				Timestamp: t2_appl,
				Close:     15.0,
			},
			{
				Timestamp: t3_appl,
				Close:     20.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)

		assert.Equal(t, t1_appl, candle.Timestamp)

		delta, err := playground.Tick(20*time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		candle, err = playground.GetCandle(symbol1, period)
		assert.NoError(t, err)
		assert.Equal(t, t3_appl, candle.Timestamp)
		assert.Equal(t, 20.0, candle.Close)
	})

	t.Run("Returns new candle/s on state changes", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		t1_appl := startTime
		t2_appl := startTime.Add(5 * time.Minute)
		t3_appl := startTime.Add(10 * time.Minute)
		t4_appl := startTime.Add(20 * time.Minute)

		t1_goog := startTime
		t2_goog := startTime.Add(10 * time.Minute)
		t3_goog := startTime.Add(20 * time.Minute)

		candles1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1_appl,
				Close:     10.0,
			},
			{
				Timestamp: t2_appl,
				Close:     15.0,
			},
			{
				Timestamp: t3_appl,
				Close:     20.0,
			},
			{
				Timestamp: t4_appl,
				Close:     25.0,
			},
		}

		candles2 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1_goog,
				Close:     100.0,
			},
			{
				Timestamp: t2_goog,
				Close:     200.0,
			},
			{
				Timestamp: t3_goog,
				Close:     300.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles1, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		repo2, err := NewCandleRepository(symbol2, period, candles2, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1, repo2)
		assert.NoError(t, err)

		// initial tick: new APPL and GOOG candles
		delta, err := playground.Tick(0*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 2)

		var applDelta, googDelta *BacktesterCandle
		for _, d := range delta.NewCandles {
			if d.Symbol == symbol1 {
				applDelta = d
			} else if d.Symbol == symbol2 {
				googDelta = d
			}
		}

		assert.Equal(t, symbol1, applDelta.Symbol)
		assert.Equal(t, t1_appl, applDelta.Bar.Timestamp)
		assert.Equal(t, 10.0, applDelta.Bar.Close)
		assert.Equal(t, symbol2, googDelta.Symbol)
		assert.Equal(t, t1_goog, googDelta.Bar.Timestamp)
		assert.Equal(t, 100.0, googDelta.Bar.Close)

		// new APPL candle, but not GOOG
		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 1)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t2_appl, delta.NewCandles[0].Bar.Timestamp)
		assert.Equal(t, 15.0, delta.NewCandles[0].Bar.Close)

		// new APPL and GOOG candle
		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 2)
		assert.Equal(t, symbol1, delta.NewCandles[0].Symbol)
		assert.Equal(t, t3_appl, delta.NewCandles[0].Bar.Timestamp)
		assert.Equal(t, 20.0, delta.NewCandles[0].Bar.Close)
		assert.Equal(t, symbol2, delta.NewCandles[1].Symbol)
		assert.Equal(t, t2_goog, delta.NewCandles[1].Bar.Timestamp)
		assert.Equal(t, 200.0, delta.NewCandles[1].Bar.Close)

		// no new candle
		delta, err = playground.Tick(5*time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.NewCandles, 0)
	})

	t.Run("GetCandle returns first candle", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		candles1 := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     10.0,
			},
			{
				Timestamp: startTime.Add(5 * time.Minute),
				Close:     15.0,
			},
		}

		repo1, err := NewCandleRepository(symbol1, period, candles1, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo1)
		assert.NoError(t, err)

		candle, err := playground.GetCandle(symbol1, period)
		assert.NoError(t, err)
		assert.Equal(t, startTime, candle.Timestamp)
		assert.Equal(t, 10.0, candle.Close)
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

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: startTime.Add(1 * time.Minute),
				Close:     100.0,
			},
			{
				Timestamp: startTime.Add(2 * time.Minute),
				Close:     100.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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
	source := eventmodels.CandleRepositorySource{Type: "test"}

	t.Run("GetAccountBalance", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime)
		assert.NoError(t, err)

		initialBalance := playground.GetBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})

	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: startTime.Add(2 * time.Minute),
				Close:     115.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 2, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(2*time.Minute, false)
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

		delta, err = playground.Tick(0, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, 115.0, delta.NewTrades[0].Price)
		assert.Equal(t, 1030.0, playground.GetBalance())
	})

	t.Run("GetAccountBalance - increase and decrease after profitable and unprofitable trade", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := startTime
		t2 := startTime.Add(1 * time.Minute)
		t3 := startTime.Add(2 * time.Minute)
		t4 := startTime.Add(3 * time.Minute)
		t5 := startTime.Add(4 * time.Minute)

		prices := []float64{100.0, 110.0, 90.0, 100.0, 90.0}

		now := startTime

		balance := 100000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1,
				Close:     prices[0],
			},
			{
				Timestamp: t2,
				Close:     prices[1],
			},
			{
				Timestamp: t3,
				Close:     prices[2],
			},
			{
				Timestamp: t4,
				Close:     prices[3],
			},
			{
				Timestamp: t5,
				Close:     prices[4],
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
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
		prices := []float64{50.0, 100.0, 85.0}

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: t1,
				Close:     prices[0],
			},
			{
				Timestamp: t2,
				Close:     prices[1],
			},
			{
				Timestamp: t3,
				Close:     prices[2],
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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

		gain := (prices[1] - prices[0]) * 10

		assert.Equal(t, balance+gain, playground.GetBalance())
	})
}

func TestPlaceOrder(t *testing.T) {
	t.Run("Cannot place order if data feed is not imported", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)
		env := PlaygroundEnvironmentSimulator

		now := startTime

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})
}

func TestPositions(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator
	source := eventmodels.CandleRepositorySource{Type: "test"}

	t.Run("GetPosition", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		balance := 1000.0
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime)
		assert.NoError(t, err)

		position, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.ErrorContains(t, err, "position not found")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - open trades, long", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)
		now := startTime
		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     250.0,
			},
			{
				Timestamp: startTime.Add(1 * time.Minute),
				Close:     260.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		// Create a new playground
		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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
		position1, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)

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
		position2, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, 5.0, position2.Quantity)
	})

	t.Run("GetPosition - open trades, short", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     250.0,
			},
			{
				Timestamp: startTime.Add(1 * time.Minute),
				Close:     260.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
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
		position1, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
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
		position2, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, -5.0, position2.Quantity)
	})

	t.Run("GetPosition - average cost basis - multiple orders - same direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 1, 0, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 2, 0, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 3, 0, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 4, 0, 0, time.UTC)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     200.0,
			},
			{
				Timestamp: t2,
				Close:     300.0,
			},
			{
				Timestamp: t3,
				Close:     400.0,
			},
			{
				Timestamp: t4,
				Close:     500.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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

		position, err := playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		// 2nd order
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		// close orders
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSell, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.ErrorContains(t, err, "position not found")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)

		// 3rd order - original direction
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 400.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 1)
		// assert.Equal(t, 10.0, position.OpenTrades[0].Quantity)
	})

	t.Run("GetPosition - average cost basis - partial closes", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 1, 0, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 2, 0, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 3, 0, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 4, 0, 0, time.UTC)
		t5 := time.Date(2021, time.January, 1, 0, 5, 0, 0, time.UTC)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     200.0,
			},
			{
				Timestamp: t2,
				Close:     300.0,
			},
			{
				Timestamp: t3,
				Close:     400.0,
			},
			{
				Timestamp: t4,
				Close:     500.0,
			},
			{
				Timestamp: t5,
				Close:     600.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
		assert.NoError(t, err)

		// 1st order
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 15, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		openOrders := playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 1)
		assert.Equal(t, order1, openOrders[0])

		// close 1st partial
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		// close 2nd partial
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, -5.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		// close 3nd partial
		order5 := NewBacktesterOrder(5, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order5)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.ErrorContains(t, err, "position not found")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis - partial closes LONG", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 1, 0, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 2, 0, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 3, 0, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 4, 0, 0, time.UTC)
		t5 := time.Date(2021, time.January, 1, 0, 5, 0, 0, time.UTC)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     200.0,
			},
			{
				Timestamp: t2,
				Close:     300.0,
			},
			{
				Timestamp: t3,
				Close:     400.0,
			},
			{
				Timestamp: t4,
				Close:     500.0,
			},
			{
				Timestamp: t5,
				Close:     600.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
		assert.NoError(t, err)

		// open 1st order
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 15, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 15.0, position.Quantity)
		assert.Equal(t, 100.0, position.CostBasis)

		openOrders := playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 1)
		assert.Equal(t, order1, openOrders[0])

		// open 2nd order
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 15, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		newTrade := delta.NewTrades[0]
		assert.Equal(t, 200.0, newTrade.Price)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 30.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		openOrders = playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 2)
		assert.Equal(t, order1, openOrders[0])
		assert.Equal(t, order2, openOrders[1])

		// close 1st partial
		order3 := NewBacktesterOrder(3, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSell, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order3)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 25.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		// close 2nd partial
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSell, 15, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 150.0, position.CostBasis)

		// close 3nd partial
		order5 := NewBacktesterOrder(5, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order5)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.ErrorContains(t, err, "position not found")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis - multiple orders - reverse direction", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 1, 0, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 2, 0, 0, time.UTC)
		t3 := time.Date(2021, time.January, 1, 0, 3, 0, 0, time.UTC)
		t4 := time.Date(2021, time.January, 1, 0, 4, 0, 0, time.UTC)
		t5 := time.Date(2021, time.January, 1, 0, 5, 0, 0, time.UTC)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     200.0,
			},
			{
				Timestamp: t2,
				Close:     300.0,
			},
			{
				Timestamp: t3,
				Close:     400.0,
			},
			{
				Timestamp: t4,
				Close:     500.0,
			},
			{
				Timestamp: t5,
				Close:     600.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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

		position, err := playground.GetPosition(symbol)
		assert.NoError(t, err)
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

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
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

		position, err = playground.GetPosition(symbol)
		assert.ErrorContains(t, err, "position not found")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)

		orders := playground.GetOrders()
		assert.Len(t, orders, 3)
		assert.ElementsMatch(t, orders[2].Closes, []*BacktesterOrder{order1, order2})

		assert.Len(t, order3.Trades, 1)

		assert.Len(t, order1.ClosedBy, 1)
		assert.Equal(t, order3.Trades[0].Symbol, order1.ClosedBy[0].Symbol)
		assert.Equal(t, order3.Trades[0].Price, order1.ClosedBy[0].Price)
		assert.Equal(t, order3.Trades[0].CreateDate, order1.ClosedBy[0].CreateDate)
		assert.Equal(t, 10.0, order1.ClosedBy[0].Quantity)

		assert.Len(t, order2.ClosedBy, 1)
		assert.Equal(t, order3.Trades[0].Symbol, order2.ClosedBy[0].Symbol)
		assert.Equal(t, order3.Trades[0].Price, order2.ClosedBy[0].Price)
		assert.Equal(t, order3.Trades[0].CreateDate, order2.ClosedBy[0].CreateDate)
		assert.Equal(t, 10.0, order2.ClosedBy[0].Quantity)

		// 3rd order - reverse direction
		order4 := NewBacktesterOrder(4, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order4)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
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

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		assert.Equal(t, 20.0, position.Quantity)
		assert.Equal(t, 450.0, position.CostBasis)

		openOrders = playground.GetOpenOrders(symbol)
		assert.Len(t, openOrders, 2)
		assert.ElementsMatch(t, openOrders, []*BacktesterOrder{order4, order5})
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		t1 := time.Date(2021, time.January, 1, 0, 1, 0, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 2, 0, 0, time.UTC)

		now := startTime

		balance := 1000000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     100.0,
			},
			{
				Timestamp: t1,
				Close:     600.0,
			},
			{
				Timestamp: t2,
				Close:     300.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(symbol)
		assert.NoError(t, err)
		costBasis := 100.0
		assert.Equal(t, costBasis, position.CostBasis)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 20, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(symbol)
		assert.NoError(t, err)
		costBasis = ((10 / 30.0) * 100.0) + ((20 / 30.0) * 600.0)
		assert.Equal(t, costBasis, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after buy", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		balance := 100000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     1000.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 1000.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after sell", func(t *testing.T) {
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		balance := 100000.0

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     250.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, startTime, repo)
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

		position, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, 5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after sell short", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		now := startTime

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     250.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 100000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
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

		position, err = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
		// assert.Len(t, position.OpenTrades, 2)
		// assert.Equal(t, -10.0, position.OpenTrades[0].Quantity)
		// assert.Equal(t, -5.0, position.OpenTrades[1].Quantity)
	})

	t.Run("GetPosition - Quantity decrease after buy to cover", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)
		now := startTime

		candles := []*eventmodels.PolygonAggregateBarV2{
			{
				Timestamp: startTime,
				Close:     250.0,
			},
		}

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 100000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 5, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		position, err = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.NoError(t, err)
		assert.Equal(t, -5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
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
	source := eventmodels.CandleRepositorySource{
		Type: "test",
	}

	candles := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: startTime,
			Close:     100.0,
		},
		{
			Timestamp: t1,
			Close:     200.0,
		},
		{
			Timestamp: endTime,
			Close:     250.0,
		},
	}

	repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
	assert.NoError(t, err)

	t.Run("No positions", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		freeMargin, err := playground.GetFreeMargin()
		assert.NoError(t, err)
		assert.Equal(t, balance, freeMargin)
	})

	t.Run("Adjust with unrealized PnL", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		// place order
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		_, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		positions, err := playground.GetPositions()
		assert.NoError(t, err)
		usedMargin := positions[symbol].MaintenanceMargin
		assert.Equal(t, 500.0, usedMargin)

		freeMargin, err := playground.GetFreeMargin()
		assert.NoError(t, err)

		assert.Equal(t, balance-usedMargin, freeMargin)

		// move price: price change 100 -> 200 => unrealized PnL = 1,000
		previousFreeMargin := freeMargin
		_, err = playground.Tick(time.Hour, false)
		assert.NoError(t, err)

		freeMargin, err = playground.GetFreeMargin()
		assert.NoError(t, err)

		assert.Equal(t, previousFreeMargin+1000.0, freeMargin)
	})

	t.Run("Long position reduces free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		tradeQty := 1.0
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, tradeQty, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		assert.Len(t, delta.NewTrades, 1)
		tradePrc := delta.NewTrades[0].Price
		assert.Equal(t, 100.0, tradePrc)

		freeMargin, err := playground.GetFreeMargin()
		assert.NoError(t, err)

		assert.Equal(t, balance-(tradePrc*tradeQty*0.5), freeMargin)
	})

	t.Run("Short position reduces free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		tradeQty := 1.0
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, tradeQty, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)

		assert.Len(t, delta.NewTrades, 1)
		tradePrc := delta.NewTrades[0].Price
		assert.Equal(t, 100.0, tradePrc)

		freeMargin, err := playground.GetFreeMargin()
		assert.NoError(t, err)

		assert.Equal(t, balance-(tradePrc*tradeQty*1.5), freeMargin)
	})

	t.Run("Trade rejected if insufficient free margin", func(t *testing.T) {
		balance := 1000.0
		clock := NewClock(startTime, endTime, nil)

		playground, err := NewPlayground(nil, balance, clock, nil, env, nil, nil, now, repo)
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
		assert.Contains(t, *delta.InvalidOrders[0].RejectReason, ErrInsufficientFreeMargin.Error())
	})
}

func TestOrders(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime, nil)
	env := PlaygroundEnvironmentSimulator
	source := eventmodels.CandleRepositorySource{Type: "test"}

	now := startTime

	candles := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: startTime,
			Close:     100.0,
		},
	}

	t.Run("PlaceOrder - market", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
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
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 10000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSellShort, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err := playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.Len(t, delta.InvalidOrders, 0)

		order = NewBacktesterOrder(2, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot buy to cover when not short", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuyToCover, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - cannot sell before buy", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideSell, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid class", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		order := NewBacktesterOrder(1, BacktesterOrderClass("invalid"), now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid price", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		price := float64(0)
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, &price, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order)
		assert.Error(t, err)
	})

	t.Run("PlaceOrder - invalid id", func(t *testing.T) {
		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
		assert.NoError(t, err)

		id := uint(1)

		order1 := NewBacktesterOrder(id, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err := playground.PlaceOrder(order1)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(id, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), TradierOrderSideBuy, 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		_, err = playground.PlaceOrder(order2)
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	period := 1 * time.Minute
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	env := PlaygroundEnvironmentSimulator
	source := eventmodels.CandleRepositorySource{Type: "test"}

	prices := []float64{100.0, 105.0, 110.0}
	t1 := startTime
	t2 := startTime.Add(time.Minute)
	t3 := startTime.Add(2 * time.Minute)

	candles := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: t1,
			Close:     prices[0],
		},
		{
			Timestamp: t2,
			Close:     prices[1],
		},
		{
			Timestamp: t3,
			Close:     prices[2],
		},
	}

	t.Run("Short order rejected if long position exists", func(t *testing.T) {
		clock := NewClock(startTime, endTime, nil)

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, startTime, repo)
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
		clock := NewClock(startTime, endTime, nil)

		now := startTime

		repo, err := NewCandleRepository(symbol, period, candles, []string{}, nil, 0, source)
		assert.NoError(t, err)

		playground, err := NewPlayground(nil, 1000.0, clock, nil, env, nil, nil, now, repo)
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
		assert.Equal(t, t1, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[0], delta.NewTrades[0].Price)

		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		changes, err = playground.PlaceOrder(order2)
		assert.NoError(t, err)
		err = changes.Commit()
		assert.NoError(t, err)

		delta, err = playground.Tick(time.Minute, false)
		assert.NoError(t, err)
		assert.NotNil(t, delta)

		assert.Len(t, delta.NewTrades, 1)
		assert.Equal(t, t2, delta.NewTrades[0].CreateDate)
		assert.Equal(t, quantity, delta.NewTrades[0].Quantity)
		assert.Equal(t, prices[1], delta.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
