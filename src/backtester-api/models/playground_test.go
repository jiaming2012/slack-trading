package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
)

func TestBalance(t *testing.T) {
	t.Run("GetAccountBalance", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, mock.NewMockBacktesterDataFeed())
		initialBalance := playground.GetAccountBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})

	t.Run("GetAccountBalance - increase after profitable trade", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)

		order1 := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order1)
		assert.NoError(t, err)

		feed.SetPrice(100.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		feed.SetPrice(115.0)

		assert.Equal(t, 1000.0, playground.GetAccountBalance())

		order2 := NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideSell, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Equal(t, 1150.0, playground.GetAccountBalance())
	})
}

func TestPositions(t *testing.T) {
	t.Run("GetPosition", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, mock.NewMockBacktesterDataFeed())
		position := playground.GetPosition("AAPL")
		assert.Equal(t, 0.0, position.Quantity)
		assert.Equal(t, 0.0, position.CostBasis)
	})

	t.Run("GetPosition - average cost basis", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)
		order1 := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order1)
		assert.NoError(t, err)

		feed.SetPrice(600.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition("AAPL")
		assert.Equal(t, 600.0, position.CostBasis)

		order2 := NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideBuy, 20, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.NoError(t, err)

		feed.SetPrice(300.0)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition("AAPL")
		assert.Equal(t, 400.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after buy", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		feed.SetPrice(1000.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition("AAPL")
		assert.Equal(t, 10.0, position.Quantity)
		assert.Equal(t, 1000.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after sell", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		feed.SetPrice(250.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		order = NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideSell, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition("AAPL")
		assert.Equal(t, 5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity increase after sell short", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		feed.SetPrice(250.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition("AAPL")
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order = NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideSellShort, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition("AAPL")
		assert.Equal(t, -15.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})

	t.Run("GetPosition - Quantity decrease after buy to cover", func(t *testing.T) {
		feed := mock.NewMockBacktesterDataFeed()

		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		feed.SetPrice(250.0)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position := playground.GetPosition("AAPL")
		assert.Equal(t, -10.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)

		order = NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideBuyToCover, 5, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		position = playground.GetPosition("AAPL")
		assert.Equal(t, -5.0, position.Quantity)
		assert.Equal(t, 250.0, position.CostBasis)
	})
}

func TestOrders(t *testing.T) {
	feed := mock.NewMockBacktesterDataFeed()
	feed.SetPrice(100.0)

	t.Run("AddOrder", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		assert.Len(t, playground.GetOrders(), 0)

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		assert.Len(t, playground.GetOrders(), 1)
	})

	t.Run("AddOrder - cannot buy after short sell", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideSellShort, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.NoError(t, err)

		_, err = playground.Tick(time.Second)
		assert.NoError(t, err)

		order = NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - cannot buy to cover when not short", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuyToCover, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - cannot sell before buy", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideSell, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid class", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, "invalid", "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid price", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		price := float64(0)
		order := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, &price, nil, nil)
		err := playground.AddOrder(order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid id", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)

		id := uint(1)

		order1 := NewBacktesterOrder(id, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order1)
		assert.NoError(t, err)

		order2 := NewBacktesterOrder(id, Equity, "AAPL", BacktesterOrderSideBuy, 10, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	feed := mock.NewMockBacktesterDataFeed()

	t.Run("Tick", func(t *testing.T) {
		clock := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

		playground := NewPlayground(1000.0, clock, feed)
		quantity := 10.0
		order1 := NewBacktesterOrder(1, Equity, "AAPL", BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, nil)
		err := playground.AddOrder(order1)
		assert.NoError(t, err)

		price := 105.0
		feed.SetPrice(price)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock, stateChange.NewTrades[0].TransactionDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, price, stateChange.NewTrades[0].Price)

		feed.SetPrice(110.0)

		order2 := NewBacktesterOrder(2, Equity, "AAPL", BacktesterOrderSideBuy, quantity, Market, Day, nil, nil, nil)
		err = playground.AddOrder(order2)
		assert.NoError(t, err)

		stateChange, err = playground.Tick(time.Second)
		assert.NoError(t, err)
		assert.NotNil(t, stateChange)

		assert.Len(t, stateChange.NewTrades, 1)
		assert.Equal(t, clock.Add(time.Second), stateChange.NewTrades[0].TransactionDate)
		assert.Equal(t, quantity, stateChange.NewTrades[0].Quantity)
		assert.Equal(t, 110.0, stateChange.NewTrades[0].Price)

		assert.Equal(t, 2, len(playground.GetOrders()))
	})
}
