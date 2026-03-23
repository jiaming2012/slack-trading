package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

func TestPostTickProcessing_NoDoubleCloseOnAssignmentAndExpiration(t *testing.T) {
	// This test verifies that when both an OptionAssigned and OptionExpired event
	// fire for the same order in a single tick, the order is only closed once.
	// Previously, the assignment handler would place a pending close order, then
	// the expiration handler would try to close the same order again, failing with
	// "cannot sell to close when no position exists" because the pending close
	// already offset the position to zero.

	optionSymbol := eventmodels.OptionSymbol("O:AAPL250703C00210000")
	stockSymbol := eventmodels.StockSymbol("AAPL")
	period := time.Minute
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	startTime := time.Date(2025, time.July, 3, 9, 30, 0, 0, tz)
	endTime := time.Date(2025, time.July, 4, 16, 0, 0, 0, tz)
	expirationTime := time.Date(2025, time.July, 3, 16, 0, 0, 0, tz)

	stockCandles := []*eventmodels.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 215},
		{Timestamp: startTime.Add(time.Minute), Close: 215},
	}

	optionCandles := []*eventmodels.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 5.0},
		{Timestamp: startTime.Add(time.Minute), Close: 5.5},
	}

	repo1, err := NewCandleRepository(stockSymbol, period, stockCandles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	repo2, err := NewCandleRepository(optionSymbol, period, optionCandles, []string{}, nil, 0, eventmodels.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	balance := 100000.0
	clock := NewClock(startTime, endTime, nil)

	playground, err := NewPlayground(nil, nil, nil, balance, balance, clock, nil, PlaygroundEnvironmentSimulator, startTime, []string{}, nil, repo1, repo2)
	require.NoError(t, err)

	data := make(map[eventmodels.OptionSymbol][]*eventmodels.AggregateBarWithIndicators)
	var bars []*eventmodels.AggregateBarWithIndicators
	for _, c := range optionCandles {
		bars = append(bars, c.ToAggregateBarWithIndicators())
	}
	data[optionSymbol] = bars
	playground.OptionsBroker = &MockOptionsBroker{data: data}

	mockDB := NewMockDatabase()
	err = mockDB.SavePlaygroundSession(playground)
	require.NoError(t, err)

	// Place a sell_to_open order (short call)
	order1, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, startTime, string(optionSymbol), TradierOrderSideSellToOpen, 5, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order1)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit(nil))
	}

	// Tick to fill the order
	delta, err := playground.Tick(0, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	// Verify short position
	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Equal(t, -5.0, pos.Quantity)

	// Construct a TickDelta with BOTH an assignment AND expiration event for the same order
	tickDelta := &TickDelta{
		Events: []*TickDeltaEvent{
			{
				Type: TickDeltaEventTypeOptionAssigned,
				OptionAssignmentEvent: &OptionAssignmentEvent{
					OrderId:          order1.ID,
					Symbol:           order1.GetInstrument(),
					AssignedQuantity: 5.0,
					AssignedPrice:    5.0,
					Timestamp:        expirationTime,
				},
			},
			{
				Type: TickDeltaEventTypeOptionExpired,
				OptionExpirationEvent: &OptionExpirationEvent{
					Symbol:                  optionSymbol,
					UnderlyingPriceAtExpiry: 215.0,
					Timestamp:               expirationTime,
				},
			},
		},
		EquityPlot: &eventmodels.EquityPlot{
			Timestamp: expirationTime,
			Value:     balance,
		},
	}

	// This should NOT error — the expiration handler should skip the order
	// that was already closed by the assignment handler
	_, err = playground.postTickProcessing(tickDelta, mockDB)
	require.NoError(t, err)
}
