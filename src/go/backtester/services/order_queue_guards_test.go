package services

// Tests for the anomaly-guard observation feeds (wire-anomaly-guard-feeds):
// they drive the REAL live order-update pipeline functions
// (UpdateTradierOrderQueue -> DrainTradierOrderQueue ->
// UpdatePendingMarginOrders / fillPendingOrder) with MockBroker + MockDatabase
// and assert the guards saw the observations and the SHARED halt controller
// engaged. Simulation paths must feed nothing.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

type guardPipelineFixture struct {
	broker         *backtester_models.MockBroker
	database       *backtester_models.MockDatabase
	livePlayground *backtester_models.Playground
	updateQueue    *models.FIFOQueue[*backtester_models.TradierOrderUpdateEvent]
	controller     *safety.HaltController
	symbol         models.StockSymbol
	now            time.Time
}

// newGuardPipelineFixture assembles the same MockBroker + MockDatabase +
// reconcile/live playground harness the order-queue tests use, plus a halt
// controller for the guard registry under test.
func newGuardPipelineFixture(t *testing.T) *guardPipelineFixture {
	t.Helper()

	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	now := startTime
	symbol := models.NewStockSymbol("AAPL")

	feed := []*models.PolygonAggregateBarV2{
		{Timestamp: startTime.Add(-time.Minute), Close: 5.0},
		{Timestamp: startTime, Close: 10.0},
		{Timestamp: startTime.Add(time.Minute), Close: 20.0},
		{Timestamp: startTime.Add(2 * time.Minute), Close: 30.0},
	}

	broker := backtester_models.NewMockBroker(1000, nil)
	database := backtester_models.NewMockDatabase()
	newTradesQueue := models.NewFIFOQueue[*backtester_models.TradeRecord]("newTradesFilledQueue", 4)
	liveAccount, err := backtester_models.NewLiveAccount(broker, database)
	require.NoError(t, err)

	// Reconciliation container.
	reconcileID := uuid.New()
	reconcilePlaygroundModel := &backtester_models.Playground{}
	source := backtester_models.CreateAccountRequestSource{
		Broker:      broker.GetSource().GetBroker(),
		AccountID:   broker.GetSource().GetAccountID(),
		AccountRole: broker.GetSource().GetAccountType(),
	}
	require.NoError(t, database.CreatePlayground(reconcilePlaygroundModel, &backtester_models.PopulatePlaygroundRequest{
		ID:             &reconcileID,
		Reconciliation: true,
		Account: backtester_models.CreateAccountRequest{
			Balance: 1000.0,
			Source:  &source,
		},
		Clock: backtester_models.CreateClockRequest{
			StartDate: startTime.Format("2006-01-02"),
			StopDate:  endTime.Format("2006-01-02"),
		},
		CreatedAt:   now,
		LiveAccount: liveAccount,
		SaveToDB:    true,
	}))

	reconcilePlayground, err := backtester_models.NewReconcilePlayground(reconcilePlaygroundModel, liveAccount)
	require.NoError(t, err)
	reconcileSource, err := reconcilePlaygroundModel.GetSource()
	require.NoError(t, err)
	database.SetReconcilePlayground(reconcileSource, reconcilePlayground)

	// Live (Paper-Mode) playground.
	repo, err := backtester_models.NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, models.CandleRepositorySource{})
	require.NoError(t, err)

	liveID := uuid.New()
	clientID := "guard-feed-test"
	accountRequestSource := backtester_models.NewMockLiveAccountSource()
	s := &backtester_models.CreateAccountRequestSource{
		Broker:      accountRequestSource.GetBroker(),
		AccountID:   accountRequestSource.GetAccountID(),
		AccountRole: accountRequestSource.GetAccountType(),
	}

	livePlayground := &backtester_models.Playground{}
	require.NoError(t, backtester_models.PopulatePlayground(livePlayground, &backtester_models.PopulatePlaygroundRequest{
		ID:                  &liveID,
		ClientID:            &clientID,
		Mode:                backtester_models.ModePaper,
		Account:             backtester_models.CreateAccountRequest{Balance: 1000.0, Source: s},
		InitialBalance:      1000.0,
		BackfillOrders:      []*backtester_models.OrderRecord{},
		Tags:                []string{},
		LiveAccount:         liveAccount,
		ReconcilePlayground: reconcilePlayground,
	}, nil, now, newTradesQueue, nil, nil, repo))

	require.NoError(t, database.SavePlaygroundSession(reconcilePlayground.GetPlayground()))
	require.NoError(t, database.SavePlaygroundSession(livePlayground))

	controller, err := safety.NewHaltController(safety.NewMemoryHaltStore())
	require.NoError(t, err)

	return &guardPipelineFixture{
		broker:         broker,
		database:       database,
		livePlayground: livePlayground,
		updateQueue:    models.NewFIFOQueue[*backtester_models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 4),
		controller:     controller,
		symbol:         symbol,
		now:            now,
	}
}

func (f *guardPipelineFixture) placeOrder(t *testing.T, id uint, quantity float64) *backtester_models.OrderRecord {
	t.Helper()
	order, err := backtester_models.NewOrderRecord(id, nil, nil, f.livePlayground.GetId(), backtester_models.OrderRecordClassEquity, backtester_models.AccountRoleMargin, f.now, string(f.symbol), backtester_models.TradierOrderSideBuy, quantity, backtester_models.Market, backtester_models.Day, 0.01, nil, nil, backtester_models.OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := f.livePlayground.PlaceOrder(order)
	require.NoError(t, err)
	for _, change := range changes {
		require.NoError(t, backtester_models.CommitPlaceOrderChanges(f.database, []*backtester_models.PlaceOrderChanges{change}))
	}
	return order
}

func (f *guardPipelineFixture) lastReconcileOrder(t *testing.T) *backtester_models.OrderRecord {
	t.Helper()
	orders := f.livePlayground.GetReconcilePlayground().GetOrders()
	require.NotEmpty(t, orders)
	o := orders[len(orders)-1]
	require.NotNil(t, o.ExternalOrderID)
	return o
}

func (f *guardPipelineFixture) pumpPipeline(t *testing.T) {
	t.Helper()
	require.NoError(t, UpdateTradierOrderQueue(f.updateQueue, f.database, 0))
	_, err := DrainTradierOrderQueue(f.updateQueue, f.database)
	require.NoError(t, err)
	require.NoError(t, UpdatePendingMarginOrders(f.database))
}

func installGuardRegistry(t *testing.T, reg *safety.GuardRegistry) {
	t.Helper()
	safety.SetGuardRegistry(reg)
	t.Cleanup(func() { safety.SetGuardRegistry(nil) })
}

func guardCounter(t *testing.T, name, guard string) float64 {
	t.Helper()
	var total float64
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name == name && p.Labels["guard"] == guard {
			total += p.Value
		}
	}
	return total
}

// A broker rejection flowing through the REAL pipeline (MockBroker rejects ->
// UpdateTradierOrderQueue enqueues the status event -> DrainTradierOrderQueue
// applies it) must reach the rejection-rate guard and trip the shared
// controller.
func TestGuardFeeds_BrokerRejectionReachesGuardAndTripsHalt(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)

	installGuardRegistry(t, safety.BuildGuardRegistry(f.controller, nil, safety.GuardEnvConfig{
		Guards: safety.GuardConfig{
			RejectionWindow:     time.Hour,
			RejectionThreshold:  0.5,
			RejectionMinSamples: 1, // a single rejection is 100% > 50%
		},
		RejectionEnabled: true,
	}, nil))

	f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)

	// The broker rejects the order.
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusRejected)))

	require.NoError(t, UpdateTradierOrderQueue(f.updateQueue, f.database, 0))
	_, err := DrainTradierOrderQueue(f.updateQueue, f.database)
	require.NoError(t, err)

	// The guard observed the rejection and tripped the SHARED controller.
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_observations_total", "rejection-rate guard"))
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_trips_total", "rejection-rate guard"))

	st := f.controller.Status()
	require.True(t, st.Engaged, "a pipeline broker rejection must be able to trip the kill switch")
	require.Equal(t, safety.SourceAuto, st.Source)
	require.Contains(t, st.Reason, "rejection-rate guard")
	require.True(t, st.AckRequired)
	require.ErrorIs(t, f.controller.AllowOrder(), safety.ErrHalted)
}

// A live fill flowing through the REAL pipeline must feed the fill-deviation
// guard (requested vs actual price), the trades-per-hour guard (exactly ONE
// trade per broker fill — the reconciliation-container fill of the same broker
// event must not double-count), and the rejection-rate guard (one accepted
// outcome).
func TestGuardFeeds_LiveFillFeedsDeviationAndTradeGuards(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)

	installGuardRegistry(t, safety.BuildGuardRegistry(f.controller, nil, safety.GuardEnvConfig{
		Guards: safety.GuardConfig{
			RejectionWindow:     time.Hour,
			RejectionThreshold:  0.99,
			RejectionMinSamples: 100, // effectively never trips in this test
			// Requested price is 0.01 and the mock fill lands at 100.0 — a
			// deviation far beyond 2%, so the fill MUST trip this guard.
			FillDeviationPct: 0.02,
			// Unarmed trades-per-hour (0/0): observes without tripping, which
			// lets the test assert the observation count precisely.
			TradesPerHourMean:       0,
			TradesPerHourStdDev:     0,
			TradesPerHourMinSamples: 1,
		},
		RejectionEnabled:     true,
		FillDeviationEnabled: true,
		TradesPerHourEnabled: true,
	}, nil))

	f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)

	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	// The live order is filled.
	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)

	// One broker fill = exactly one observation per guard: the
	// reconciliation-container commit of the same fill must NOT double-count.
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_observations_total", "trades-per-hour guard"))
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_observations_total", "fill-deviation guard"))
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_observations_total", "rejection-rate guard"), "a committed fill is one accepted outcome for the rejection-rate guard")

	// The wild deviation tripped the halt through the shared controller.
	require.Equal(t, 1.0, guardCounter(t, "safety_guard_trips_total", "fill-deviation guard"))
	st := f.controller.Status()
	require.True(t, st.Engaged)
	require.Equal(t, safety.SourceAuto, st.Source)
	require.Contains(t, st.Reason, "fill-deviation guard")
}

// With no guard registry installed the same pipeline flows run untouched —
// the nil hook keeps the wiring inert (simulation/model-diff safety).
func TestGuardFeeds_UnwiredPipelineIsUnaffected(t *testing.T) {
	safety.SetGuardRegistry(nil)
	f := newGuardPipelineFixture(t)

	f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)
	require.False(t, f.controller.Status().Engaged)
}

// Simulation playgrounds (and only realtime/reconciliation ones) participate
// in the live-pipeline gate: this pins the gating predicate the rejection
// hook uses, so a Simulation order applied through any path can never feed
// the guards.
func TestGuardFeeds_SimulationPlaygroundIsGatedOut(t *testing.T) {
	sim := &backtester_models.Playground{Meta: backtester_models.Meta{Mode: backtester_models.ModeSimulation, LegacyEnv: "simulator"}}
	paper := &backtester_models.Playground{Meta: backtester_models.Meta{Mode: backtester_models.ModePaper, LegacyEnv: "live"}}
	margin := &backtester_models.Playground{Meta: backtester_models.Meta{Mode: backtester_models.ModeMargin, LegacyEnv: "live"}}
	reconcile := &backtester_models.Playground{Meta: backtester_models.Meta{LegacyEnv: "reconcile"}}

	require.False(t, isLiveOrderPipelinePlayground(sim))
	require.False(t, isLiveOrderPipelinePlayground(nil))
	require.True(t, isLiveOrderPipelinePlayground(paper))
	require.True(t, isLiveOrderPipelinePlayground(margin))
	require.True(t, isLiveOrderPipelinePlayground(reconcile))
}
