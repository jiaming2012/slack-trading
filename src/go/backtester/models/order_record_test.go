package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRealizedPL(t *testing.T) {
	t.Run("calculate realized P&L for a filled equity order (sell to close)", func(t *testing.T) {
		createDate1, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-27 09:30:00 -0400 EDT")
		createDate2, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-19 10:30:00 -0400 EDT")
		extOrderID := uint(7)
		closeOrderID := uint(2)

		order := &OrderRecord{
			Model: gorm.Model{
				ID: 7,
			},
			Class:            OrderRecordClassEquity,
			Symbol:           "AAPL",
			Side:             TradierOrderSideSell,
			AbsoluteQuantity: 10,
			Duration:         "day",
			RequestedPrice:   0,
			Tag:              "test-order",
			Trades: []*TradeRecord{
				{
					Model: gorm.Model{
						ID: 3,
					},
					Timestamp: createDate1,
					Quantity:  -10,
					Price:     150,
				},
			},
			Status:    OrderRecordStatusFilled,
			Timestamp: createDate1,
			ClosedBy:  []*TradeRecord{},
			Closes: []*OrderRecord{
				{
					Model: gorm.Model{
						ID: 2,
					},
					Class:            OrderRecordClassEquity,
					Symbol:           "AAPL",
					Side:             TradierOrderSideBuy,
					AbsoluteQuantity: 10,
					OrderType:        "market",
					Duration:         "day",
					RequestedPrice:   0,
					Tag:              "",
					Trades: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 2},
							Timestamp: createDate2,
							Quantity:  10,
							Price:     160,
						},
					},
					Status:    "filled",
					Timestamp: createDate2,
					ClosedBy: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 3},
							Timestamp: createDate1,
							Quantity:  -10,
							Price:     150,
						},
					},
					PreviousPosition: Position{
						Quantity:          0,
						CostBasis:         0,
						PL:                0,
						MaintenanceMargin: 0,
						CurrentPrice:      0,
						Timestamp:         "",
					},
					Attributes: map[string]string{},
				},
			},
			ExternalOrderID: &extOrderID,
			CloseOrderId:    &closeOrderID,
			Attributes:      map[string]string{},
			PreviousPosition: Position{
				Quantity:          20,
				CostBasis:         150,
				PL:                100,
				MaintenanceMargin: 0,
				CurrentPrice:      0.01,
				Timestamp:         "2025-05-23T15:58:00-04:00",
			},
		}

		pl := order.CalcRealizedPL()
		require.Equal(t, -100.0, pl)
	})

	t.Run("calculate realized P&L for a filled equity order (sell short)", func(t *testing.T) {
		createDate1, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-27 09:30:00 -0400 EDT")
		createDate2, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-19 10:30:00 -0400 EDT")
		extOrderID := uint(7)
		closeOrderID := uint(2)

		order := &OrderRecord{
			Model: gorm.Model{
				ID: 7,
			},
			Class:            OrderRecordClassEquity,
			Symbol:           "AAPL",
			Side:             TradierOrderSideBuyToCover,
			AbsoluteQuantity: 10,
			Duration:         "day",
			RequestedPrice:   0,
			Tag:              "test-order",
			Trades: []*TradeRecord{
				{
					Model: gorm.Model{
						ID: 3,
					},
					Timestamp: createDate1,
					Quantity:  10,
					Price:     150,
				},
			},
			Status:    OrderRecordStatusFilled,
			Timestamp: createDate1,
			ClosedBy:  []*TradeRecord{},
			Closes: []*OrderRecord{
				{
					Model: gorm.Model{
						ID: 2,
					},
					Class:            OrderRecordClassEquity,
					Symbol:           "AAPL",
					Side:             TradierOrderSideSellShort,
					AbsoluteQuantity: 10,
					OrderType:        "market",
					Duration:         "day",
					RequestedPrice:   0,
					Tag:              "",
					Trades: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 2},
							Timestamp: createDate2,
							Quantity:  -10,
							Price:     160,
						},
					},
					Status:    "filled",
					Timestamp: createDate2,
					ClosedBy: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 3},
							Timestamp: createDate1,
							Quantity:  10,
							Price:     150,
						},
					},
					PreviousPosition: Position{
						Quantity:          0,
						CostBasis:         0,
						PL:                0,
						MaintenanceMargin: 0,
						CurrentPrice:      0,
						Timestamp:         "",
					},
					Attributes: map[string]string{},
				},
			},
			ExternalOrderID: &extOrderID,
			CloseOrderId:    &closeOrderID,
			Attributes:      map[string]string{},
			PreviousPosition: Position{
				Quantity:          -20,
				CostBasis:         150,
				PL:                100,
				MaintenanceMargin: 0,
				CurrentPrice:      0.01,
				Timestamp:         "2025-05-23T15:58:00-04:00",
			},
		}

		pl := order.CalcRealizedPL()
		require.Equal(t, 100.0, pl)
	})

	t.Run("calculate realized P&L for a filled option order", func(t *testing.T) {
		createDate1, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-27 09:30:00 -0400 EDT")
		createDate2, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", "2025-05-19 10:30:00 -0400 EDT")
		extOrderID := uint(7)
		closeOrderID := uint(2)

		order := &OrderRecord{
			Model: gorm.Model{
				ID: 7,
			},
			Class:            OrderRecordClassOption,
			Symbol:           "O:COIN250523C00267500",
			Side:             "buy_to_close",
			AbsoluteQuantity: 1,
			Duration:         "day",
			RequestedPrice:   0,
			Tag:              "auto-closed-on-expiration",
			Trades: []*TradeRecord{
				{
					Model: gorm.Model{
						ID: 3,
					},
					Timestamp: createDate1,
					Quantity:  1,
					Price:     0,
				},
			},
			Status:    OrderRecordStatusFilled,
			Timestamp: createDate1,
			ClosedBy:  []*TradeRecord{},
			Closes: []*OrderRecord{
				{
					Model: gorm.Model{
						ID: 2,
					},
					Class:            OrderRecordClassOption,
					Symbol:           "O:COIN250523C00267500",
					Side:             "sell_to_open",
					AbsoluteQuantity: 1,
					OrderType:        "market",
					Duration:         "day",
					RequestedPrice:   0,
					Tag:              "",
					Trades: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 2},
							Timestamp: createDate2,
							Quantity:  -1,
							Price:     7,
						},
					},
					Status:    "filled",
					Timestamp: createDate2,
					ClosedBy: []*TradeRecord{
						{
							Model:     gorm.Model{ID: 3},
							Timestamp: createDate1,
							Quantity:  1,
							Price:     0,
						},
					},
					PreviousPosition: Position{
						Quantity:          0,
						CostBasis:         0,
						PL:                0,
						MaintenanceMargin: 0,
						CurrentPrice:      0,
						Timestamp:         "",
					},
					Attributes: map[string]string{
						"ev":          "6.181553375764889",
						"stock_price": "266.075",
					},
				},
			},
			ExternalOrderID: &extOrderID,
			CloseOrderId:    &closeOrderID,
			Attributes:      map[string]string{},
			PreviousPosition: Position{
				Quantity:          -2,
				CostBasis:         7,
				PL:                1398,
				MaintenanceMargin: 21,
				CurrentPrice:      0.01,
				Timestamp:         "2025-05-23T15:58:00-04:00",
			},
		}

		pl := order.CalcRealizedPL()
		require.Equal(t, 700.0, pl)
	})
}

func TestRollbackOrder(t *testing.T) {
	t.Run("rollback an order", func(t *testing.T) {
		symbol1 := "AAPL"
		startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
		order, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassEquity, AccountRoleMock, startTime, symbol1, TradierOrderSideBuy, 30, Market, Day, 0.01, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
		trade := NewTradeRecord(order, startTime, 30, 0.01)
		_, err = order.Fill(trade)

		require.NoError(t, err)

		assert.Equal(t, order.Status, OrderRecordStatusFilled)
		assert.Len(t, order.Trades, 1)

		order.Rollback(trade)

		assert.Equal(t, order.Status, OrderRecordStatusRejected)
		assert.Len(t, order.Trades, 0)
	})
}
