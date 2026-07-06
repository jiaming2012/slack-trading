package models

// Tests for graceful option auto-close deferral during an engaged halt
// (wire-companion-stops, review nit e): an assignment or expiration auto-close
// generated while the kill-switch order gate is engaged must NOT error the
// tick — the constructed close request is retained with its fill parameters
// and committed on the first tick after the halt clears. With the halt clear,
// auto-closes behave exactly as today (pinned by
// TestPostTickProcessing_NoDoubleCloseOnAssignmentAndExpiration).

import (
	"fmt"
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

// Adversarial-review finding 2: deferrals originate from drain-once events,
// so they are persisted on deferral and must survive a process restart — a
// halt + restart must not silently drop the close. This drives the full
// mechanism: defer (persisted with record IDs) → memory wiped (restart) →
// restore from the persisted records → halt clears → the closes commit and
// their persisted rows are deleted.
func TestDeferredAutoClose_SurvivesRestartViaPersistedRecords(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	_, err := f.playground.postTickProcessing(f.assignmentDelta(), f.mockDB)
	require.NoError(t, err)
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2)

	// Both deferrals were persisted with record IDs at deferral time.
	persisted, err := f.mockDB.LoadDeferredAutoCloses(f.playground.GetId())
	require.NoError(t, err)
	require.Len(t, persisted, 2)
	for _, d := range persisted {
		require.NotZero(t, d.RecordID, "a deferral must be persisted when it is created, not later")
		require.NotNil(t, d.Request)
	}

	// Process restart: the in-memory deferral list dies with the process; the
	// load path restores it from the persisted records.
	f.playground.deferredAutoCloses = nil
	stale := f.playground.RestoreDeferredAutoCloses(persisted)
	require.Empty(t, stale, "the source order is still open, so nothing is stale")
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2, "deferred closes must survive the restart")

	// The halt clears after the restart: the RESTORED closes commit with
	// their retained fill parameters, and their persisted rows are deleted.
	SetOrderGate(nil)
	delta, err := f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)
	require.Len(t, delta.NewTrades, 2)
	require.Empty(t, f.playground.GetDeferredAutoCloses())

	remaining, err := f.mockDB.LoadDeferredAutoCloses(f.playground.GetId())
	require.NoError(t, err)
	require.Empty(t, remaining, "committed deferrals must delete their persisted rows so a later restart cannot replay them")

	pos := f.playground.positionCache.Get(f.optionSymbol.GetTicker())
	require.Equal(t, 0.0, pos.Quantity)
}

// A persisted deferral whose source order no longer has remaining open
// quantity (the close already committed) must be reported stale at restore —
// replaying it would double-close into a reversal.
func TestDeferredAutoClose_StaleRecordsNotRestored(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	_, err := f.playground.postTickProcessing(f.assignmentDelta(), f.mockDB)
	require.NoError(t, err)

	// Capture the persisted records BEFORE the closes commit (simulating a
	// crash after commit but before the row deletes were themselves durable).
	persisted, err := f.mockDB.LoadDeferredAutoCloses(f.playground.GetId())
	require.NoError(t, err)
	require.Len(t, persisted, 2)

	SetOrderGate(nil)
	_, err = f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)
	require.Empty(t, f.playground.GetDeferredAutoCloses())

	// Restart replay of the stale records: the source order is fully closed,
	// so nothing may be restored.
	f.playground.deferredAutoCloses = nil
	stale := f.playground.RestoreDeferredAutoCloses(persisted)
	require.Len(t, stale, 2, "records for an already-closed order must be reported stale, never restored")
	require.Empty(t, f.playground.GetDeferredAutoCloses())
}

// equityRejectingDB wraps MockDatabase and rejects equity order placement,
// simulating the exercise stock leg persistently failing while the option
// close succeeds.
type equityRejectingDB struct {
	*MockDatabase
}

func (m *equityRejectingDB) PlaceOrders(playgroundID uuid.UUID, requests []*CreateOrderRequest) ([]*OrderRecord, error) {
	if len(requests) == 1 && requests[0].Class == OrderRecordClassEquity {
		return nil, fmt.Errorf("transient equity placement failure")
	}
	return m.MockDatabase.PlaceOrders(playgroundID, requests)
}

// Re-review refinement 1 (compound path): the option close commits and fills
// while the equity EXERCISE leg keeps failing; after a restart the source
// option order has no remaining open quantity — but the exercise leg's
// staleness is independent, and it MUST be restored and retried. Keying its
// staleness on the option order's remaining quantity silently lost the
// exercised stock delivery.
func TestDeferredAutoClose_ExerciseLegRestoredAfterOptionCloseCommitted(t *testing.T) {
	telemetry.Init()
	f := newDeferredAutoCloseFixture(t)

	SetOrderGate(engagedGate{reason: "drill"})
	t.Cleanup(func() { SetOrderGate(nil) })

	// Both legs deferred (option close with CloseOrderId + equity exercise leg).
	_, err := f.playground.postTickProcessing(f.assignmentDelta(), f.mockDB)
	require.NoError(t, err)
	require.Len(t, f.playground.GetDeferredAutoCloses(), 2)

	// Halt clears, but equity placement persistently fails: the option close
	// is placed (its persisted row deleted), the equity leg stays deferred and
	// persisted, and the tick errors loudly.
	SetOrderGate(nil)
	failingDB := &equityRejectingDB{MockDatabase: f.mockDB}
	_, err = f.playground.postTickProcessing(&TickDelta{}, failingDB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient equity placement failure")
	require.Len(t, f.playground.GetDeferredAutoCloses(), 1, "only the equity exercise leg remains deferred")

	// The next tick fills the placed option close (still failing the equity
	// retry): the source option order is now FULLY closed.
	_, err = f.playground.Tick(0, false, failingDB)
	require.Error(t, err)
	sourceOrder, err := f.playground.GetOrder(f.order.ID)
	require.NoError(t, err)
	remaining, err := sourceOrder.GetRemainingOpenQuantity()
	require.NoError(t, err)
	require.Zero(t, remaining, "the option close must be fully committed for the compound path")

	// Process restart: only the equity exercise leg is persisted.
	persisted, err := f.mockDB.LoadDeferredAutoCloses(f.playground.GetId())
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Nil(t, persisted[0].Request.CloseOrderId, "the surviving deferral is the exercise leg")

	f.playground.deferredAutoCloses = nil
	stale := f.playground.RestoreDeferredAutoCloses(persisted)
	require.Empty(t, stale, "the exercise leg must NOT be judged by the option order's remaining quantity")
	require.Len(t, f.playground.GetDeferredAutoCloses(), 1, "the exercised stock delivery must be restored, not silently lost")

	// A healthy retry commits the stock delivery and clears the persisted row.
	_, err = f.playground.postTickProcessing(&TickDelta{}, f.mockDB)
	require.NoError(t, err)
	require.Empty(t, f.playground.GetDeferredAutoCloses())

	exerciseLegs := f.closeOrders("exercise-call-option-1")
	require.Len(t, exerciseLegs, 1, "the exercised stock delivery must eventually commit")

	remainingRecords, err := f.mockDB.LoadDeferredAutoCloses(f.playground.GetId())
	require.NoError(t, err)
	require.Empty(t, remainingRecords)

	// Replaying the old persisted record now IS stale: the leg's tagged order
	// exists, so a duplicate stock delivery is refused.
	stale = f.playground.RestoreDeferredAutoCloses(persisted)
	require.Len(t, stale, 1, "an already-placed exercise leg must be discarded on replay")
	require.Empty(t, f.playground.GetDeferredAutoCloses())
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
