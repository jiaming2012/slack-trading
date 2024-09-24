package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBacktesterOrderStatus(t *testing.T) {
	now := time.Time{}

	t.Run("Open", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		assert.Equal(t, BacktesterOrderStatusOpen, order.GetStatus())
	})

	t.Run("PartiallyFilled", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 5, 1)
		err := order.Fill(trade)
		assert.NoError(t, err)

		assert.Equal(t, BacktesterOrderStatusPartiallyFilled, order.GetStatus())
	})

	t.Run("Filled", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 10, 1)
		order.Fill(trade)
		assert.Equal(t, BacktesterOrderStatusFilled, order.GetStatus())
	})

	t.Run("Filled - invalid price", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 10, 0)
		err := order.Fill(trade)
		assert.Error(t, err)
	})

	t.Run("Filled - quantity exceeds order quantity", func(t *testing.T) {
		quantity := 10.0
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", quantity, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, quantity, 1)
		err := order.Fill(trade)
		assert.NoError(t, err)

		trade = NewBacktesterTrade(now, 1, 1)
		err = order.Fill(trade)
		assert.Error(t, err)
	})

	t.Run("Filled - invalid quantity", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 0, 1)
		err := order.Fill(trade)
		assert.Error(t, err)
	})

	t.Run("Filled - multiple trades", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 5, 1)
		order.Fill(trade)

		trade = NewBacktesterTrade(now, 5, 1)
		order.Fill(trade)

		assert.Equal(t, BacktesterOrderStatusFilled, order.GetStatus())
	})

	t.Run("Cancelled", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		order.Cancel()
		assert.Equal(t, BacktesterOrderStatusCancelled, order.GetStatus())
	})

	t.Run("Fill is rejected after order is cancelled", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		order.Cancel()
		trade := NewBacktesterTrade(now, 10, 1)
		err := order.Fill(trade)
		assert.Error(t, err)
	})

	t.Run("Rejected", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		order.Reject()
		assert.Equal(t, BacktesterOrderStatusRejected, order.GetStatus())
	})

	t.Run("Fill is rejected after order is rejected", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		order.Reject()
		trade := NewBacktesterTrade(now, 10, 1)
		err := order.Fill(trade)
		assert.Error(t, err)
	})

	t.Run("Fill is rejected after order is filled", func(t *testing.T) {
		order := NewBacktesterOrder(1, Equity, "AAPL", "buy", 10, Market, Day, nil, nil, nil)
		trade := NewBacktesterTrade(now, 10, 1)
		order.Fill(trade)

		trade = NewBacktesterTrade(now, 1, 1)
		err := order.Fill(trade)
		assert.Error(t, err)
	})
}
