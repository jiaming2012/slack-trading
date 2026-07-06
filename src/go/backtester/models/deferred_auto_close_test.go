package models

// Tests for graceful option auto-close deferral during an engaged halt
// (wire-companion-stops, review nit e): an assignment or expiration auto-close
// generated while the kill-switch order gate is engaged must NOT error the
// tick — the constructed close request is retained with its fill parameters
// and committed on the first tick after the halt clears. With the halt clear,
// auto-closes behave exactly as today (pinned by
// TestPostTickProcessing_NoDoubleCloseOnAssignmentAndExpiration).

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// engagedGate is a stub order gate that always rejects, standing in for an
// engaged halt controller at the models layer (the safety package cannot be
// imported here without a cycle).
type engagedGate struct{ reason string }

func (g engagedGate) AllowOrder() error {
	return &testHaltError{reason: g.reason}
}

type testHaltError struct{ reason string }

func (e *testHaltError) Error() string { return "order submission halted by kill switch: " + e.reason }

// deferredAutoCloseFixture builds the same short-call playground the
// post-tick-processing regression test uses: one short option position whose
// assignment/expiration events drive auto-closes.
type deferredAutoCloseFixture struct {
	playground     *Playground
	mockDB         *MockDatabase
	order          *OrderRecord
	optionSymbol   models.OptionSymbol
	expirationTime time.Time
}

func newDeferredAutoCloseFixture(t *testing.T) *deferredAutoCloseFixture {
	t.Helper()

	optionSymbol := models.OptionSymbol("O:AAPL250703C00210000")
	stockSymbol := models.StockSymbol("AAPL")
	period := time.Minute
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	startTime := time.Date(2025, time.July, 3, 9, 30, 0, 0, tz)
	endTime := time.Date(2025, time.July, 4, 16, 0, 0, 0, tz)
	expirationTime := time.Date(2025, time.July, 3, 16, 0, 0, 0, tz)

	stockCandles := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 215},
		{Timestamp: startTime.Add(time.Minute), Close: 215},
	}

	optionCandles := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime, Close: 5.0},
		{Timestamp: startTime.Add(time.Minute), Close: 5.5},
	}

	repo1, err := NewCandleRepository(stockSymbol, period, stockCandles, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	repo2, err := NewCandleRepository(optionSymbol, period, optionCandles, []string{}, nil, 0, models.CandleRepositorySource{Type: "test"})
	require.NoError(t, err)

	playground, err := NewPlayground(PlaygroundConfig{
		Balance: 100000.0,
		Clock:   NewClock(startTime, endTime, nil),
		Mode:    ModeSimulation,
		Now:     startTime,
		Feeds:   []*CandleRepository{repo1, repo2},
	})
	require.NoError(t, err)

	data := make(map[models.OptionSymbol][]*models.AggregateBarWithIndicators)
	var bars []*models.AggregateBarWithIndicators
	for _, c := range optionCandles {
		bars = append(bars, c.ToAggregateBarWithIndicators())
	}
	data[optionSymbol] = bars
	playground.OptionsBroker = &MockOptionsBroker{data: data}

	mockDB := NewMockDatabase()
	require.NoError(t, mockDB.SavePlaygroundSession(playground))

	// Open a short call.
	order, err := NewOrderRecord(1, nil, nil, uuid.Nil, OrderRecordClassOption, AccountRoleMock, startTime, string(optionSymbol), TradierOrderSideSellToOpen, 5, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}

	delta, err := playground.Tick(0, false, mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 1)

	pos := playground.positionCache.Get(optionSymbol.GetTicker())
	require.Equal(t, -5.0, pos.Quantity)

	return &deferredAutoCloseFixture{
		playground:     playground,
		mockDB:         mockDB,
		order:          order,
		optionSymbol:   optionSymbol,
		expirationTime: expirationTime,
	}
}

func (f *deferredAutoCloseFixture) assignmentDelta() *TickDelta {
	return &TickDelta{
		Events: []*TickDeltaEvent{
			{
				Type: TickDeltaEventTypeOptionAssigned,
				OptionAssignmentEvent: &OptionAssignmentEvent{
					OrderId:          f.order.ID,
					Symbol:           f.order.GetInstrument(),
					AssignedQuantity: 5.0,
					AssignedPrice:    5.0,
					Timestamp:        f.expirationTime,
				},
			},
		},
	}
}

func (f *deferredAutoCloseFixture) expirationDelta() *TickDelta {
	return &TickDelta{
		Events: []*TickDeltaEvent{
			{
				Type: TickDeltaEventTypeOptionExpired,
				OptionExpirationEvent: &OptionExpirationEvent{
					Symbol:                  f.optionSymbol,
					UnderlyingPriceAtExpiry: 215.0,
					Timestamp:               f.expirationTime,
				},
			},
		},
	}
}

// closeOrders returns the playground's orders carrying the given auto-close tag.
func (f *deferredAutoCloseFixture) closeOrders(tag string) []*OrderRecord {
	var out []*OrderRecord
	for _, o := range f.playground.GetAllOrders() {
		if o.Tag == tag {
			out = append(out, o)
		}
	}
	return out
}

func deferredGaugeTotal(t *testing.T) float64 {
	t.Helper()
	var total float64
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name == telemetry.MetricDeferredAutoCloses {
			total += p.Value
		}
	}
	return total
}

func TestDeferredAutoClose_AssignmentDuringHaltDefersAndCommitsOnRelease(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	// The tick with the assignment event completes WITHOUT error while halted.
	_, err := f.playground.postTickProcessing(f.assignmentDelta(), f.mockDB)
	require.NoError(t, err, "an assignment auto-close during a halt must not fail the tick")

	// The closes were deferred, not placed. An assigned short call constructs
	// TWO close requests — the option close plus the equity exercise leg —
	// and both must be retained.
	require.Empty(t, f.closeOrders("auto-closed-on-early-assignment"), "no close order may be placed while the halt is engaged")
	deferred := f.playground.GetDeferredAutoCloses()
	require.Len(t, deferred, 2)
	for _, d := range deferred {
		require.Equal(t, f.order.ID, d.SourceOrderID)
		require.Contains(t, d.Reason, "halted by kill switch")
	}
	require.Equal(t, 2.0, deferredGaugeTotal(t), "the internal-registry gauge must reflect the outstanding deferrals")

	// While the halt stays engaged, subsequent ticks keep deferring.
	_, err = f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2)

	// Release the halt: the next tick places and commits the deferred close
	// with its retained fill parameters.
	SetOrderGate(nil)
	delta, err := f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)

	closes := f.closeOrders("auto-closed-on-early-assignment")
	require.Len(t, closes, 1, "the deferred auto-close must commit once the halt clears")
	require.Equal(t, OrderRecordStatusFilled, closes[0].Status)
	require.Len(t, closes[0].Trades, 1)
	require.Len(t, delta.NewTrades, 2, "both retained closes (option close + equity exercise leg) must commit")

	// The retained fill parameters are reproduced: the equity exercise leg
	// fills at the strike, exactly as it would have originally.
	exerciseLegs := f.closeOrders("exercise-call-option-1")
	require.Len(t, exerciseLegs, 1)
	require.Equal(t, OrderRecordStatusFilled, exerciseLegs[0].Status)
	require.Len(t, exerciseLegs[0].Trades, 1)
	require.Equal(t, 210.0, exerciseLegs[0].Trades[0].Price, "the retained fill price (the strike) must be used")

	require.Empty(t, f.playground.GetDeferredAutoCloses())
	require.Equal(t, 0.0, deferredGaugeTotal(t))

	// The position is flat.
	pos := f.playground.positionCache.Get(f.optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity)
}

func TestDeferredAutoClose_ExpirationDuringHaltDefersAndCommitsOnRelease(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	_, err := f.playground.postTickProcessing(f.expirationDelta(), f.mockDB)
	require.NoError(t, err, "an expiration auto-close during a halt must not fail the tick")

	require.Empty(t, f.closeOrders("auto-closed-on-expiration"))
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2, "the option close and the equity exercise leg are both retained")

	SetOrderGate(nil)
	_, err = f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)

	closes := f.closeOrders("auto-closed-on-expiration")
	require.Len(t, closes, 1)
	require.Equal(t, OrderRecordStatusFilled, closes[0].Status)
	require.Empty(t, f.playground.GetDeferredAutoCloses())

	pos := f.playground.positionCache.Get(f.optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity)
}

// An expiration event arriving on a LATER tick, while the assignment's
// auto-close is still deferred, must not queue a second close for the same
// order — that would double-close (flip the position) on release.
func TestDeferredAutoClose_CrossTickAssignmentThenExpirationDefersOnce(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	_, err := f.playground.postTickProcessing(f.assignmentDelta(), f.mockDB)
	require.NoError(t, err)
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2)

	// Next tick, still halted: the expiration event fires for the same order.
	_, err = f.playground.postTickProcessing(f.expirationDelta(), f.mockDB)
	require.NoError(t, err)
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2, "the same order must not accumulate additional deferred closes")

	SetOrderGate(nil)
	_, err = f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)

	require.Empty(t, f.playground.GetDeferredAutoCloses())
	pos := f.playground.positionCache.Get(f.optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity, "exactly one close must commit — never a reversal")
}
