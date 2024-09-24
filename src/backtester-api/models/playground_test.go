package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/backtester-api/mock"
)

func TestBalance(t *testing.T) {
	feed := mock.NewMockBacktesterDataFeed()

	t.Run("GetAccountBalance", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		initialBalance := playground.GetAccountBalance()

		assert.Equal(t, 1000.0, initialBalance)
	})
}

func TestOrders(t *testing.T) {
	feed := mock.NewMockBacktesterDataFeed()

	t.Run("AddOrder", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(*order)
		assert.NoError(t, err)

		assert.Len(t, playground.GetOrders(), 1)
	})

	t.Run("AddOrder - invalid class", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, "invalid", "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(*order)
		assert.Error(t, err)
	})

	t.Run("AddOrder - invalid price", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		price := float64(0)
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, &price, nil, nil)
		err := playground.AddOrder(*order)
		assert.Error(t, err)
	})
}

func TestTrades(t *testing.T) {
	feed := mock.NewMockBacktesterDataFeed()

	t.Run("Tick", func(t *testing.T) {
		playground := NewPlayground(1000.0, time.Time{}, feed)
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		err := playground.AddOrder(*order)
		assert.NoError(t, err)

		stateChange, err := playground.Tick(time.Second)
		assert.NoError(t, err)

		assert.Len(t, stateChange.NewTrades, 1)
	})
}
