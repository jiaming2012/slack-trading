package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestFeed(t *testing.T) {
	// startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	// endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	// clock := NewClock(startTime, endTime)

	t.Run("Tick returns new candle", func(t *testing.T) {
		// feed := mock.NewMockBacktesterDataFeed()

		// playground := NewPlayground(1000.0, clock, feed)

		// candle, err := feed.FetchCandle(startTime, eventmodels.StockSymbol("AAPL"))
		// assert.NoError(t, err)
		// assert.NotNil(t, candle)

		// stateChange, err := playground.Tick(time.Second)
		// assert.NoError(t, err)
		// assert.NotNil(t, stateChange)

		// assert.Equal(t, candle, stateChange.NewCandle)
	})
}

// func TestClock(t *testing.T) {
// 	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
// 	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)

// 	t.Run("Backtester is complete", func(t *testing.T) {
// 		clock := NewClock(startTime, endTime)

// 		feed := mock.NewMockBacktesterDataFeed()

// 		playground := NewPlayground(1000.0, clock, feed)

// 		stateChange, err := playground.Tick(time.Second)

// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		stateChange, err = playground.Tick(time.Hour)

// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		assert.True(t, stateChange.IsBacktestComplete)

// 		// no longer able to tick after backtest is complete
// 		stateChange, err = playground.Tick(time.Second)
// 		assert.Error(t, err)
// 		assert.Nil(t, stateChange)
// 	})

// 	t.Run("Tick() errors after end time", func(t *testing.T) {

// 	})

// 	// todo: compose a stop loss: order + stop order + tag linking orders

// 	t.Run("Clock is finished at end time", func(t *testing.T) {
// 		clock := NewClock(startTime, endTime)

// 		assert.False(t, clock.IsFinished())

// 		clock.Add(59 * time.Minute)

// 		assert.False(t, clock.IsFinished())

// 		clock.Add(time.Minute)

// 		assert.True(t, clock.IsFinished())
// 	})

// 	t.Run("Clock is finished after end time", func(t *testing.T) {
// 		clock := NewClock(startTime, endTime)

// 		assert.False(t, clock.IsFinished())

// 		clock.Add(61 * time.Minute)

// 		assert.True(t, clock.IsFinished())
// 	})
// }

// func TestBalance(t *testing.T) {
// 	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
// 	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
// 	clock := NewClock(startTime, endTime)

// 	t.Run("GetAccountBalance", func(t *testing.T) {
// 		playground := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed())
// 		initialBalance := playground.GetAccountBalance()

// 		assert.Equal(t, 1000.0, initialBalance)
// 	})

// 	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
// 		feed := mock.NewMockBacktesterDataFeed()

// 		playground := NewPlayground(1000.0, clock, feed)

// 		order1 := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
// 		err := playground.AddOrder(order1)
// 		assert.NoError(t, err)

// 		feed.SetPrice(100.0)

// 		stateChange, err := playground.Tick(time.Second)
// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		feed.SetPrice(115.0)

// 		assert.Equal(t, 1000.0, playground.GetAccountBalance())

// 		order2 := NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, nil)
// 		err = playground.AddOrder(order2)
// 		assert.NoError(t, err)

// 		stateChange, err = playground.Tick(time.Second)
// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		assert.Equal(t, 1150.0, playground.GetAccountBalance())
// 	})

// 	t.Run("GetAccountBalance - decrease after unprofitable trade", func(t *testing.T) {
// 		feed := mock.NewMockBacktesterDataFeed()

// 		playground := NewPlayground(1000.0, clock, feed)

// 		order1 := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
// 		err := playground.AddOrder(order1)
// 		assert.NoError(t, err)

// 		feed.SetPrice(100.0)

// 		stateChange, err := playground.Tick(time.Second)
// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		feed.SetPrice(85.0)

// 		assert.Equal(t, 1000.0, playground.GetAccountBalance())

// 		order2 := NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, nil)
// 		err = playground.AddOrder(order2)
// 		assert.NoError(t, err)

// 		stateChange, err = playground.Tick(time.Second)
// 		assert.NoError(t, err)
// 		assert.NotNil(t, stateChange)

// 		assert.Equal(t, 850.0, playground.GetAccountBalance())
// 	})
// }

func TestPositions(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime)

	t.Run("GetPosition", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, mock.NewMockBacktesterDataFeed(symbol, nil, nil))
		assert.NoError(t, err)
		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		t1 := time.Date(2021, time.January, 1, 0, 0, 1, 0, time.UTC)
		t2 := time.Date(2021, time.January, 1, 0, 0, 2, 0, time.UTC)
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2}, []float64{600.0, 300.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order1 := NewBacktesterOrder(1, Equity, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order1)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(symbol)
		assert.Equal(t, 600.0, position.CostBasis)

		order2 := NewBacktesterOrder(2, Equity, symbol, BacktesterOrderSideBuy, 20, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{1000.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
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

		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		order = NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, 5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after sell short", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order = NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after buy to cover", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{250.0})

		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition(eventmodels.StockSymbol("AAPL"))
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order = NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
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

	feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{endTime}, []float64{100.0})

	t.Run("AddOrder", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, symbol, BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		assert.Len(t, playground.GetOrders(), 0)

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		assert.Len(t, playground.GetOrders(), 1)
	})

	t.Run("AddOrder - cannot buy after short sell", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		order = NewBacktesterOrder(2, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - cannot buy to cover when not short", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuyToCover, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - cannot sell before buy", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideSell, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid class", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		order := NewBacktesterOrder(1, "invalid", eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid price", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		price := float64(0)
		order := NewBacktesterOrder(1, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, &price, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid id", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)

		id := uint(1)

		order1 := NewBacktesterOrder(id, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(id, Equity, eventmodels.StockSymbol("AAPL"), BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 1, 1, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime)

	prices := []float64{100.0, 105.0, 110.0}
	t1 := startTime
	t2 := startTime.Add(time.Second)
	t3 := startTime.Add(2 * time.Second)
	feed := mock.NewMockBacktesterDataFeed(symbol, []time.Time{t1, t2, t3}, prices)

	t.Run("Tick", func(t *testing.T) {
		playground, err := NewPlayground(1000.0, clock, feed)
		assert.NoError(t, err)
		quantity := 10.0
		order1 := NewBacktesterOrder(1, Equity, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order1)
		assert.NoError(t, err)


		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, stateChange.NewTrades[0].TransactionDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, prices[1], stateChange.NewTrades[0].Price)

		order2 := NewBacktesterOrder(2, Equity, symbol, BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock.CurrentTime, stateChange.NewTrades[0].TransactionDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, prices[2], stateChange.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
