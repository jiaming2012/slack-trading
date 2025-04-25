package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestOrderRecordStatus(t *testing.T) {
	now := time.Time{}

	t.Run("New", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		require.Equal(t, OrderRecordStatusNew, order.GetStatus())
	})

	t.Run("PartiallyFilled", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 5, 1)
		_, err := order.Fill(trade)
		require.NoError(t, err)

		require.Equal(t, OrderRecordStatusPartiallyFilled, order.GetStatus())
	})

	t.Run("Filled", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 10, 1)
		order.Fill(trade)
		require.Equal(t, OrderRecordStatusFilled, order.GetStatus())
	})

	t.Run("Filled - invalid price", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 10, 0)
		_, err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - quantity exceeds order quantity", func(t *testing.T) {
		quantity := 10.0
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", quantity, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, quantity, 1)
		_, err := order.Fill(trade)
		require.NoError(t, err)

		trade = NewTradeRecord(order, now, 1, 1)
		_, err = order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - invalid quantity", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 0, 1)
		_, err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Filled - multiple trades", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 5, 1)
		order.Fill(trade)

		trade = NewTradeRecord(order, now, 5, 1)
		order.Fill(trade)

		require.Equal(t, OrderRecordStatusFilled, order.GetStatus())
	})

	t.Run("Cancelled", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		order.Cancel()
		require.Equal(t, OrderRecordStatusCanceled, order.GetStatus())
	})

	t.Run("Fill is rejected after order is cancelled", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		order.Cancel()
		trade := NewTradeRecord(order, now, 10, 1)
		_, err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Rejected", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		order.Reject(fmt.Errorf("something happened"))
		require.Equal(t, OrderRecordStatusRejected, order.GetStatus())
	})

	t.Run("Fill is rejected after order is rejected", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		order.Reject(fmt.Errorf("something happened"))
		trade := NewTradeRecord(order, now, 10, 1)
		_, err := order.Fill(trade)
		require.Error(t, err)
	})

	t.Run("Fill is rejected after order is filled", func(t *testing.T) {
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, now, eventmodels.StockSymbol("AAPL"), "buy", 10, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, now, 10, 1)
		order.Fill(trade)

		trade = NewTradeRecord(order, now, 1, 1)
		_, err := order.Fill(trade)
		require.Error(t, err)
	})
}
