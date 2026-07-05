//go:build integration

package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openTestDB connects to the local playground database for integration tests.
// Skip the test if the database is not available.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "host=localhost user=grodt password=test747 dbname=playground port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping remap test: cannot connect to local Postgres: %v", err)
	}

	// Auto-migrate the tables we need
	if err := db.AutoMigrate(&Playground{}, &OrderRecord{}, &TradeRecord{}, &EquityPlotRecord{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	return db
}

func TestRemap_SimpleOrderWithTrade(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()
	now := time.Now().Truncate(time.Millisecond)

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 100000,
	}

	// Simulate in-memory nonce-assigned IDs
	order := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideBuy,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now,
	}
	order.ID = 1 // nonce-assigned

	trade := &TradeRecord{
		Timestamp: now,
		Quantity:  10,
		Price:     150.0,
	}
	trade.ID = 1 // nonce-assigned
	order.Trades = []*TradeRecord{trade}

	// Set up playground's in-memory state
	playground.Orders = []*OrderRecord{order}
	playground.account = NewBacktesterAccount(100000, []*OrderRecord{order})

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// Verify new IDs were assigned (not the original nonce IDs)
	assert.NotEqual(t, uint(1), order.ID, "order should get a new GORM-assigned ID")
	assert.NotEqual(t, uint(0), order.ID, "order ID should not be 0")
	assert.NotEqual(t, uint(1), trade.ID, "trade should get a new GORM-assigned ID")
	assert.NotEqual(t, uint(0), trade.ID, "trade ID should not be 0")

	// Verify trade's OrderID FK is correct
	require.NotNil(t, trade.OrderID)
	assert.Equal(t, order.ID, *trade.OrderID, "trade.OrderID should point to the new order ID")

	// Verify we can load the order back from DB
	var loaded OrderRecord
	err = tx.Preload("Trades").First(&loaded, order.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "AAPL", loaded.Symbol)
	require.Len(t, loaded.Trades, 1)
	assert.Equal(t, 150.0, loaded.Trades[0].Price)
}

func TestRemap_CloseOrderIdFixup(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()
	now := time.Now().Truncate(time.Millisecond)

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 100000,
	}

	// Order 1: open position
	openOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideBuy,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now,
		Trades: []*TradeRecord{
			{Timestamp: now, Quantity: 10, Price: 150.0},
		},
	}
	openOrder.ID = 1
	openOrder.Trades[0].ID = 1

	// Order 2: close position (references order 1 via CloseOrderId)
	oldOpenOrderID := uint(1)
	closeOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideSell,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now.Add(time.Hour),
		CloseOrderId:     &oldOpenOrderID,
		Trades: []*TradeRecord{
			{Timestamp: now.Add(time.Hour), Quantity: -10, Price: 155.0},
		},
	}
	closeOrder.ID = 2
	closeOrder.Trades[0].ID = 2

	orders := []*OrderRecord{openOrder, closeOrder}
	playground.Orders = orders
	playground.account = NewBacktesterAccount(100000, orders)

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// CloseOrderId should now point to openOrder's NEW ID
	require.NotNil(t, closeOrder.CloseOrderId)
	assert.Equal(t, openOrder.ID, *closeOrder.CloseOrderId,
		"CloseOrderId should be remapped to the new open order ID")

	// Verify from DB
	var loadedClose OrderRecord
	err = tx.First(&loadedClose, closeOrder.ID).Error
	require.NoError(t, err)
	require.NotNil(t, loadedClose.CloseOrderId)
	assert.Equal(t, openOrder.ID, *loadedClose.CloseOrderId)
}

func TestRemap_ClosedByM2M(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()
	now := time.Now().Truncate(time.Millisecond)

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 100000,
	}

	// Open order with a trade
	openTrade := &TradeRecord{Timestamp: now, Quantity: 10, Price: 150.0}
	openTrade.ID = 1

	openOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideBuy,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now,
		Trades:           []*TradeRecord{openTrade},
		ClosedBy:         []*TradeRecord{}, // will be populated below
	}
	openOrder.ID = 1

	// Close order whose trade also appears in openOrder.ClosedBy
	closeTrade := &TradeRecord{Timestamp: now.Add(time.Hour), Quantity: -10, Price: 155.0}
	closeTrade.ID = 2

	closeOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideSell,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now.Add(time.Hour),
		Trades:           []*TradeRecord{closeTrade},
	}
	closeOrder.ID = 2

	// The close trade closes the open order
	openOrder.ClosedBy = []*TradeRecord{closeTrade}

	orders := []*OrderRecord{openOrder, closeOrder}
	playground.Orders = orders
	playground.account = NewBacktesterAccount(100000, orders)

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// Verify ClosedBy M2M from DB
	var loaded OrderRecord
	err = tx.Preload("ClosedBy").First(&loaded, openOrder.ID).Error
	require.NoError(t, err)
	require.Len(t, loaded.ClosedBy, 1, "open order should have 1 ClosedBy trade")
	assert.Equal(t, closeTrade.ID, loaded.ClosedBy[0].ID,
		"ClosedBy trade ID should match the remapped close trade ID")
}

func TestRemap_ParentTradeID(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()
	now := time.Now().Truncate(time.Millisecond)

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 100000,
	}

	// Parent trade (full fill)
	parentTrade := &TradeRecord{Timestamp: now, Quantity: -10, Price: 155.0}
	parentTrade.ID = 1

	// Partial trade (child of parent)
	oldParentID := uint(1)
	childTrade := &TradeRecord{
		Timestamp:     now,
		Quantity:      -5,
		Price:         155.0,
		ParentTradeID: &oldParentID,
	}
	childTrade.ID = 2

	order := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideSell,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now,
		Trades:           []*TradeRecord{parentTrade, childTrade},
	}
	order.ID = 1

	orders := []*OrderRecord{order}
	playground.Orders = orders
	playground.account = NewBacktesterAccount(100000, orders)

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// ParentTradeID should be remapped to the new parent trade ID
	require.NotNil(t, childTrade.ParentTradeID)
	assert.Equal(t, parentTrade.ID, *childTrade.ParentTradeID,
		"child trade's ParentTradeID should point to the new parent trade ID")

	// Verify from DB
	var loadedChild TradeRecord
	err = tx.First(&loadedChild, childTrade.ID).Error
	require.NoError(t, err)
	require.NotNil(t, loadedChild.ParentTradeID)
	assert.Equal(t, parentTrade.ID, *loadedChild.ParentTradeID)
}

func TestRemap_NoOrders(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 50000,
	}
	playground.account = NewBacktesterAccount(50000, nil)

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// Verify playground was saved
	var loaded Playground
	err = tx.First(&loaded, "id = ?", playgroundID).Error
	require.NoError(t, err)
	assert.Equal(t, float64(50000), loaded.Balance)
}

func TestRemap_ClosesM2M(t *testing.T) {
	db := openTestDB(t)

	tx := db.Begin()
	defer tx.Rollback()

	playgroundID := uuid.New()
	now := time.Now().Truncate(time.Millisecond)

	playground := &Playground{
		ID: playgroundID,
		Meta: Meta{
			Mode:      ModeSimulation,
			Role:      AccountRoleSimulator,
			LegacyEnv: LegacyEnvSimulator,
		},
		Balance: 100000,
	}

	// Open order
	openOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideBuy,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now,
		Trades: []*TradeRecord{
			{Timestamp: now, Quantity: 10, Price: 150.0},
		},
	}
	openOrder.ID = 1
	openOrder.Trades[0].ID = 1

	// Close order that references the open order via Closes M2M
	closeOrder := &OrderRecord{
		PlaygroundID:     playgroundID,
		AccountRole:      AccountRoleSimulator,
		Class:            OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             TradierOrderSideSell,
		AbsoluteQuantity: 10,
		OrderType:        Market,
		Duration:         Day,
		Status:           OrderRecordStatusFilled,
		Timestamp:        now.Add(time.Hour),
		Closes:           []*OrderRecord{openOrder},
		Trades: []*TradeRecord{
			{Timestamp: now.Add(time.Hour), Quantity: -10, Price: 160.0},
		},
	}
	closeOrder.ID = 2
	closeOrder.Trades[0].ID = 2

	orders := []*OrderRecord{openOrder, closeOrder}
	playground.Orders = orders
	playground.account = NewBacktesterAccount(100000, orders)

	err := RemapAndSavePlayground(tx, playground)
	require.NoError(t, err)

	// Verify Closes M2M from DB
	var loaded OrderRecord
	err = tx.Preload("Closes").First(&loaded, closeOrder.ID).Error
	require.NoError(t, err)
	require.Len(t, loaded.Closes, 1, "close order should have 1 Closes entry")
	assert.Equal(t, openOrder.ID, loaded.Closes[0].ID,
		fmt.Sprintf("Closes[0] should point to remapped open order (got %d, want %d)", loaded.Closes[0].ID, openOrder.ID))
}
