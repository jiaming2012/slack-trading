package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestRollbackOrder(t *testing.T) {
	t.Run("rollback an order", func(t *testing.T) {
		symbol1 := eventmodels.NewStockSymbol("AAPL")
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		order := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, LiveAccountTypeMock, startTime, symbol1, TradierOrderSideBuy, 30, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil)
		trade := NewTradeRecord(order, startTime, 30, 0.01)
		_, err := order.Fill(trade)

		require.NoError(t, err)

		assert.Equal(t, order.Status, OrderRecordStatusFilled)
		assert.Len(t, order.Trades, 1)

		order.Rollback(trade)

		assert.Equal(t, order.Status, OrderRecordStatusRejected)
		assert.Len(t, order.Trades, 0)
	})
}
