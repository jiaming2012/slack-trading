package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

// setupOptionPlayground creates a playground with a stock and option repo,
// places an initial order, and ticks to fill it, returning the playground and mock DB.
func setupOptionPlayground(t *testing.T, optionSymbol models.OptionSymbol, initialSide TradierOrderSide, quantity float64) (*Playground, *MockDatabase) {
	t.Helper()

	stockSymbol := models.StockSymbol("AAPL")
	period := time.Minute
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	startTime := time.Date(2025, time.June, 3, 9, 30, 0, 0, tz)
	endTime := time.Date(2025, time.June, 10, 16, 0, 0, 0, tz)

	stockCandles := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 200},
		{Timestamp: startTime.Add(time.Minute), Close: 201},
	}

	optionCandles := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 5.0},
		{Timestamp: startTime.Add(time.Minute), Close: 5.5},
	}

	repo1, err := NewCandleRepository(stockSymbol, period, stockCandles, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	repo2, err := NewCandleRepository(optionSymbol, period, optionCandles, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	balance := 100000.0
	clock := NewClock(startTime, endTime, nil)

	playground, err := NewPlayground(PlaygroundConfig{
		Balance: balance,
		Clock:   clock,
		Env:     PlaygroundEnvironmentSimulator,
		Now:     startTime,
		Feeds:   []*CandleRepository{repo1, repo2},
	})
	require.NoError(t, err)

	// Set up mock options broker
	data := make(map[models.OptionSymbol][]*models.AggregateBarWithIndicators)
	var bars []*models.AggregateBarWithIndicators
	for _, c := range optionCandles {
		bars = append(bars, c.ToAggregateBarWithIndicators())
	}
	data[optionSymbol] = bars
	playground.OptionsBroker = &MockOptionsBroker{data: data}

	mockDB := NewMockDatabase()
	err = mockDB.SavePlaygroundSession(playground)
	require.NoError(t, err)

	// Place initial order
	order, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, startTime, string(optionSymbol), initialSide, quantity, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}

	// Tick to fill the order
	delta, err := playground.Tick(0, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	return playground, mockDB
}

func TestIsSideAllowed_SellToOpenBlockedWhenLong(t *testing.T) {
	optionSymbol := models.OptionSymbol("O:AAPL250703C00210000")

	// Create playground with a long option position (buy_to_open)
	playground, _ := setupOptionPlayground(t, optionSymbol, TradierOrderSideBuyToOpen, 5)

	// Verify we have a long position
	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Greater(t, pos.Quantity, 0.0, "expected long position after buy_to_open")

	// Attempt to sell_to_open the same contract — should be blocked
	order2, err := NewOrderRecord(2, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, playground.GetCurrentTime(), string(optionSymbol), TradierOrderSideSellToOpen, 5, Market, Day, 5.5, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	_, err = playground.PlaceOrder(order2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot sell to open when long position")
	require.Contains(t, err.Error(), "must sell to close")
}

func TestIsSideAllowed_BuyToOpenBlockedWhenShort(t *testing.T) {
	optionSymbol := models.OptionSymbol("O:AAPL250703C00210000")

	// Create playground with a short option position (sell_to_open)
	playground, _ := setupOptionPlayground(t, optionSymbol, TradierOrderSideSellToOpen, 5)

	// Verify we have a short position
	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Less(t, pos.Quantity, 0.0, "expected short position after sell_to_open")

	// Attempt to buy_to_open the same contract — should be blocked
	order2, err := NewOrderRecord(2, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, playground.GetCurrentTime(), string(optionSymbol), TradierOrderSideBuyToOpen, 5, Market, Day, 5.5, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	_, err = playground.PlaceOrder(order2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot buy to open when short position")
	require.Contains(t, err.Error(), "must buy to close")
}

func TestIsSideAllowed_SellToCloseAllowedWhenLong(t *testing.T) {
	optionSymbol := models.OptionSymbol("O:AAPL250703C00210000")

	// Create playground with a long option position (buy_to_open)
	playground, mockDB := setupOptionPlayground(t, optionSymbol, TradierOrderSideBuyToOpen, 5)

	// Verify we have a long position
	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Greater(t, pos.Quantity, 0.0)

	// sell_to_close should be allowed
	order2, err := NewOrderRecord(2, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, playground.GetCurrentTime(), string(optionSymbol), TradierOrderSideSellToClose, 5, Market, Day, 5.5, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order2)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}

	// Tick to fill the close order
	delta, err := playground.Tick(time.Minute, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	// Position should be zero after close
	pos = playground.positionCache.Get(optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity)
}

func TestIsSideAllowed_BuyToCloseAllowedWhenShort(t *testing.T) {
	optionSymbol := models.OptionSymbol("O:AAPL250703C00210000")

	// Create playground with a short option position (sell_to_open)
	playground, mockDB := setupOptionPlayground(t, optionSymbol, TradierOrderSideSellToOpen, 5)

	// Verify we have a short position
	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Less(t, pos.Quantity, 0.0)

	// buy_to_close should be allowed
	order2, err := NewOrderRecord(2, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, playground.GetCurrentTime(), string(optionSymbol), TradierOrderSideBuyToClose, 5, Market, Day, 5.5, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order2)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}

	// Tick to fill the close order
	delta, err := playground.Tick(time.Minute, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	// Position should be zero after close
	pos = playground.positionCache.Get(optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity)
}

func TestIsSideAllowed_DifferentSymbolsAllowed(t *testing.T) {
	optionSymbol1 := models.OptionSymbol("O:AAPL250703C00210000")
	optionSymbol2 := models.OptionSymbol("O:AAPL250703C00205000")

	stockSymbol := models.StockSymbol("AAPL")
	period := time.Minute
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	startTime := time.Date(2025, time.June, 3, 9, 30, 0, 0, tz)
	endTime := time.Date(2025, time.June, 10, 16, 0, 0, 0, tz)

	stockCandles := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 200},
		{Timestamp: startTime.Add(time.Minute), Close: 201},
		{Timestamp: startTime.Add(2 * time.Minute), Close: 202},
	}

	optionCandles1 := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 3.0},
		{Timestamp: startTime.Add(time.Minute), Close: 3.5},
		{Timestamp: startTime.Add(2 * time.Minute), Close: 4.0},
	}

	optionCandles2 := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 5.0},
		{Timestamp: startTime.Add(time.Minute), Close: 5.5},
		{Timestamp: startTime.Add(2 * time.Minute), Close: 6.0},
	}

	repo1, err := NewCandleRepository(stockSymbol, period, stockCandles, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)
	repo2, err := NewCandleRepository(optionSymbol1, period, optionCandles1, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)
	repo3, err := NewCandleRepository(optionSymbol2, period, optionCandles2, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	balance := 100000.0
	clock := NewClock(startTime, endTime, nil)

	playground, err := NewPlayground(PlaygroundConfig{
		Balance: balance,
		Clock:   clock,
		Env:     PlaygroundEnvironmentSimulator,
		Now:     startTime,
		Feeds:   []*CandleRepository{repo1, repo2, repo3},
	})
	require.NoError(t, err)

	data := make(map[models.OptionSymbol][]*models.AggregateBarWithIndicators)
	for sym, candles := range map[models.OptionSymbol][]*models.PolygonAggregateBarV2{optionSymbol1: optionCandles1, optionSymbol2: optionCandles2} {
		var bars []*models.AggregateBarWithIndicators
		for _, c := range candles {
			bars = append(bars, c.ToAggregateBarWithIndicators())
		}
		data[sym] = bars
	}
	playground.OptionsBroker = &MockOptionsBroker{data: data}

	mockDB := NewMockDatabase()
	err = mockDB.SavePlaygroundSession(playground)
	require.NoError(t, err)

	// Buy to open symbol1
	order1, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, startTime, string(optionSymbol1), TradierOrderSideBuyToOpen, 5, Market, Day, 3.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)
	changes, err := playground.PlaceOrder(order1)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}

	delta, err := playground.Tick(0, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	// Sell to open on a DIFFERENT symbol should be allowed (no conflicting position)
	order2, err := NewOrderRecord(2, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, playground.GetCurrentTime(), string(optionSymbol2), TradierOrderSideSellToOpen, 5, Market, Day, 5.5, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err = playground.PlaceOrder(order2)
	require.NoError(t, err, "sell_to_open on different symbol should be allowed")
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}
}
