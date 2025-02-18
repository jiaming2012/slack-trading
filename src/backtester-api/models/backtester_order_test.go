package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestBacktesterOrderStatus(t *testing.T) {
	now := time.Time{}

	t.Run("Open", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		require.Equal(t, BacktesterOrderStatusOpen, order.GetStatus())
	})

	t.Run("PartiallyFilled", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 5, 1)
		err := order.Fill(trade)
		require.NoError(t, err)

		require.Equal(t, BacktesterOrderStatusPartiallyFilled, order.GetStatus())
	})

	t.Run("Filled", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 10, 1)
		order.Fill(trade)
		require.Equal(t, BacktesterOrderStatusFilled, order.GetStatus())
	})

	t.Run("Filled - invalid price", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 10, 0)
		err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - quantity exceeds order quantity", func(t *testing.T) {
		quantity := 10.0
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, quantity, 1)
		err := order.Fill(trade)
		require.NoError(t, err)

		trade = NewTradeRecord(order, now, 1, 1)
		err = order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - invalid quantity", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 0, 1)
		err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - multiple trades", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 5, 1)
		order.Fill(trade)

		trade = NewTradeRecord(order, now, 5, 1)
		order.Fill(trade)

		require.Equal(t, BacktesterOrderStatusFilled, order.GetStatus())
	})

	t.Run("Cancelled", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		order.Cancel()
		require.Equal(t, BacktesterOrderStatusCancelled, order.GetStatus())
	})

	t.Run("Fill is rejected after order is cancelled", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		order.Cancel()
		trade := NewTradeRecord(order, now, 10, 1)
		err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Rejected", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		order.Reject(fmt.Errorf("something happened"))
		require.Equal(t, BacktesterOrderStatusRejected, order.GetStatus())
	})

	t.Run("Fill is rejected after order is rejected", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		order.Reject(fmt.Errorf("something happened"))
		trade := NewTradeRecord(order, now, 10, 1)
		err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Fill is rejected after order is filled", func(t *testing.T) {
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, nil, nil, BacktesterOrderStatusPending, "")
		trade := NewTradeRecord(order, now, 10, 1)
		order.Fill(trade)

		trade = NewTradeRecord(order, now, 1, 1)
		err := order.Fill(trade)
		require.Error(t, err)
	})
}
