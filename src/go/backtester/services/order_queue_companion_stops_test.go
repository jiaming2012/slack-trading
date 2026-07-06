package services

// End-to-end tests for the wired companion-stop invocation
// (wire-companion-stops): they drive the REAL live order-update pipeline
// (UpdateTradierOrderQueue -> DrainTradierOrderQueue ->
// UpdatePendingMarginOrders -> fillPendingOrder) with MockBroker +
// MockDatabase and assert that every eligible Paper-Mode entry fill produces
// exactly one broker-held protective stop — including while the kill switch is
// engaged — while placement failures stay loud and never fail the fill. No
// real broker is contacted anywhere.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// installCompanionStopper arms companion stops for one test and cleans up.
func installCompanionStopper(t *testing.T, distance float64) *safety.CompanionStopper {
	t.Helper()
	stopper := safety.NewCompanionStopper(safety.CompanionStopConfig{StopDistance: distance})
	safety.SetCompanionStopper(stopper)
	t.Cleanup(func() { safety.SetCompanionStopper(nil) })
	return stopper
}

// placeSidedOrderOn mirrors the fixture's placeOrder with a configurable
// playground, side, and tag.
func placeSidedOrderOn(t *testing.T, f *guardPipelineFixture, playground *backtester_models.Playground, id uint, quantity float64, side backtester_models.TradierOrderSide, tag string) *backtester_models.OrderRecord {
	t.Helper()
	order, err := backtester_models.NewOrderRecord(id, nil, nil, playground.GetId(), backtester_models.OrderRecordClassEquity, backtester_models.AccountRoleMargin, f.now, string(f.symbol), side, quantity, backtester_models.Market, backtester_models.Day, 0.01, nil, nil, backtester_models.OrderRecordStatusPending, tag, nil, false, nil, nil)
	require.NoError(t, err)

	changes, err := playground.PlaceOrder(order)
	require.NoError(t, err)
	for _, change := range changes {
		require.NoError(t, backtester_models.CommitPlaceOrderChanges(f.database, []*backtester_models.PlaceOrderChanges{change}))
	}
	return order
}

// placeSidedOrder places on the fixture's default live playground.
func placeSidedOrder(t *testing.T, f *guardPipelineFixture, id uint, quantity float64, side backtester_models.TradierOrderSide, tag string) *backtester_models.OrderRecord {
	t.Helper()
	return placeSidedOrderOn(t, f, f.livePlayground, id, quantity, side, tag)
}

// addSecondLivePlayground builds a second Paper-Mode playground bound to the
// SAME live account and reconcile container as the fixture's — the real
// multi-strategy topology in which one strategy's order nets against another
// strategy's broker position and splits into multiple reconciliation trades.
func addSecondLivePlayground(t *testing.T, f *guardPipelineFixture) *backtester_models.Playground {
	t.Helper()

	feed := []*models.PolygonAggregateBarV2{
		{Timestamp: f.now.Add(-time.Minute), Close: 5.0},
		{Timestamp: f.now, Close: 10.0},
		{Timestamp: f.now.Add(time.Minute), Close: 20.0},
	}
	repo, err := backtester_models.NewCandleRepository(f.symbol, time.Minute, feed, []string{}, nil, 0, models.CandleRepositorySource{})
	require.NoError(t, err)

	id := uuid.New()
	clientID := "companion-stop-second-strategy"
	accountRequestSource := backtester_models.NewMockLiveAccountSource()
	source := &backtester_models.CreateAccountRequestSource{
		Broker:      accountRequestSource.GetBroker(),
		AccountID:   accountRequestSource.GetAccountID(),
		AccountRole: accountRequestSource.GetAccountType(),
	}

	newTradesQueue := models.NewFIFOQueue[*backtester_models.TradeRecord]("newTradesFilledQueue-second", 4)

	p := &backtester_models.Playground{}
	require.NoError(t, backtester_models.PopulatePlayground(p, &backtester_models.PopulatePlaygroundRequest{
		ID:                  &id,
		ClientID:            &clientID,
		Mode:                backtester_models.ModePaper,
		Account:             backtester_models.CreateAccountRequest{Balance: 1000.0, Source: source},
		InitialBalance:      1000.0,
		BackfillOrders:      []*backtester_models.OrderRecord{},
		Tags:                []string{},
		LiveAccount:         f.livePlayground.GetLiveAccount(),
		ReconcilePlayground: f.livePlayground.GetReconcilePlayground(),
	}, nil, f.now, newTradesQueue, nil, nil, repo))

	require.NoError(t, f.database.SavePlaygroundSession(p))
	return p
}

func companionCounter(t *testing.T, name string) float64 {
	t.Helper()
	var total float64
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name == name {
			total += p.Value
		}
	}
	return total
}

// A committed Paper-Mode long entry fill must place exactly one sell stop at
// the configured distance below the fill price, tagged with the entry order's
// durable association.
func TestCompanionStop_LongEntryPlacesSellStopBelowFill(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)
	installCompanionStopper(t, 5.0)

	liveOrder := f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)

	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	// The live order filled...
	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)

	// ...and exactly one protective stop reached the Broker seam.
	stops := f.broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, backtester_models.TradierOrderSideSell, stops[0].Sides[0], "a long entry is protected by a sell stop")
	require.Equal(t, 19, stops[0].Quantities[0], "the stop is sized to the filled quantity")
	require.NotNil(t, stops[0].StopPrice)
	require.InDelta(t, 95.0, *stops[0].StopPrice, 1e-9, "stop 5 below the 100 fill")
	require.Equal(t, safety.CompanionStopTagForEntry(liveOrder.ID), stops[0].Tag)

	require.Equal(t, 1.0, companionCounter(t, "safety_companion_stops_placed_total"))
	require.Equal(t, 0.0, companionCounter(t, "safety_companion_stop_failures_total"))
}

// A committed short entry fill must place a buy_to_cover stop above the fill.
func TestCompanionStop_ShortEntryPlacesBuyToCoverStopAboveFill(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)
	installCompanionStopper(t, 8.0)

	placeSidedOrder(t, f, 1, 7, backtester_models.TradierOrderSideSellShort, "")
	reconcileOrder := f.lastReconcileOrder(t)

	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 200.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	stops := f.broker.StopOrders()
	require.Len(t, stops, 1)
	require.Equal(t, backtester_models.TradierOrderSideBuyToCover, stops[0].Sides[0], "a short entry is protected by a buy_to_cover stop")
	require.Equal(t, 7, stops[0].Quantities[0])
	require.InDelta(t, 208.0, *stops[0].StopPrice, 1e-9, "stop 8 above the 200 fill")
}

// THE halt-bypass requirement: with the kill switch engaged, the fill's
// companion stop is still placed (it routes through the IBroker seam below the
// gated path), while ordinary new-order submission stays rejected by the gate.
func TestCompanionStop_PlacedWhileHaltEngaged_OrdinarySubmissionStaysRejected(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)
	installCompanionStopper(t, 5.0)

	// Entry placed while clear; the halt engages before the fill event lands.
	f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))

	backtester_models.SetOrderGate(f.controller)
	t.Cleanup(func() { backtester_models.SetOrderGate(nil) })
	require.NoError(t, f.controller.Engage("companion-stop halt drill"))

	f.pumpPipeline(t)

	// The fill committed and the protective stop was STILL placed.
	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)
	stops := f.broker.StopOrders()
	require.Len(t, stops, 1, "an engaged halt must never strand an open position without its protective exit")
	require.InDelta(t, 95.0, *stops[0].StopPrice, 1e-9)

	// Ordinary submission remains uniformly rejected by the gate.
	order2, err := backtester_models.NewOrderRecord(2, nil, nil, f.livePlayground.GetId(), backtester_models.OrderRecordClassEquity, backtester_models.AccountRoleMargin, f.now, string(f.symbol), backtester_models.TradierOrderSideBuy, 5, backtester_models.Market, backtester_models.Day, 0.01, nil, nil, backtester_models.OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)
	_, err = f.livePlayground.PlaceOrder(order2)
	require.Error(t, err)
	require.ErrorIs(t, err, safety.ErrHalted, "the gate carries no companion-stop exemption; ordinary submission is rejected while engaged")
}

// A companion-stop-tagged fill fed through the same pipeline must not spawn a
// second stop (the recursion guard), and a redelivered fill event for an entry
// that already has its stop must not place a duplicate.
func TestCompanionStop_NoStopForCompanionStopFillAndNoDuplicateOnRedelivery(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)
	installCompanionStopper(t, 5.0)

	liveOrder := f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)
	require.Len(t, f.broker.StopOrders(), 1)

	// A buy fill tagged as this entry's companion stop — otherwise eligible —
	// flows through the pipeline: the tag guard must exclude it.
	placeSidedOrder(t, f, 2, 19, backtester_models.TradierOrderSideBuy, safety.CompanionStopTagForEntry(liveOrder.ID))
	reconcileOrder2 := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder2.ExternalOrderID, 95.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	require.Len(t, f.broker.StopOrders(), 1, "a companion stop's own fill must never spawn another companion stop")

	// A redelivered fill event for the original entry re-invokes the hook: the
	// per-entry idempotency must keep the protective size single.
	maybePlaceCompanionStop(f.livePlayground, liveOrder, 100.0, 19.0)
	require.Len(t, f.broker.StopOrders(), 1, "a redelivered fill event must not double the protective size")

	require.Equal(t, 1.0, companionCounter(t, "safety_companion_stops_placed_total"))
}

// BLOCKER regression (adversarial-review finding 1): one strategy order that
// nets into MULTIPLE broker trades must end up FULLY protected. A buy 10
// against an existing short 4 splits into buy_to_cover 4 + buy 6 at the
// reconcile layer; each trade commits through fillPendingOrder separately, and
// the second trade must top up the stop coverage — total stop quantity must
// equal total filled quantity, with nothing skipped as a "redelivery".
func TestCompanionStop_MultiTradeNettedFillFullyProtected(t *testing.T) {
	telemetry.Init()
	f := newGuardPipelineFixture(t)
	installCompanionStopper(t, 5.0)

	// Step 1: establish a short 4 position (entry order 1).
	placeSidedOrder(t, f, 1, 4, backtester_models.TradierOrderSideSellShort, "")
	r1 := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*r1.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	// Step 2: a SECOND strategy (own playground, same broker account) buys 10
	// (entry order 2) — the reconcile layer nets it against the account's
	// short 4 into buy_to_cover 4 + buy 6: TWO reconciliation orders, TWO
	// broker fills, TWO trades committing separately for the same live entry.
	second := addSecondLivePlayground(t, f)
	placeSidedOrderOn(t, f, second, 2, 10, backtester_models.TradierOrderSideBuy, "")

	reconciles := f.livePlayground.GetReconcilePlayground().GetOrders()
	require.GreaterOrEqual(t, len(reconciles), 3, "the netted buy must produce two reconciliation orders")
	nettedOrders := reconciles[len(reconciles)-2:]
	require.Equal(t, backtester_models.TradierOrderSideBuyToCover, nettedOrders[0].Side)
	require.Equal(t, 4.0, nettedOrders[0].AbsoluteQuantity)
	require.Equal(t, backtester_models.TradierOrderSideBuy, nettedOrders[1].Side)
	require.Equal(t, 6.0, nettedOrders[1].AbsoluteQuantity)

	for _, ro := range nettedOrders {
		require.NotNil(t, ro.ExternalOrderID)
		require.NoError(t, f.broker.FillOrder(*ro.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	}
	f.pumpPipeline(t)

	// The live entry committed both trades.
	var liveOrder2 *backtester_models.OrderRecord
	for _, o := range second.GetAllOrders() {
		if o.ID == 2 {
			liveOrder2 = o
		}
	}
	require.NotNil(t, liveOrder2)
	require.Len(t, liveOrder2.Trades, 2, "the netted entry must carry both trades")

	// EVERY filled share is protected: the stops tagged to entry 2 must sum
	// to the full 10 — trade 2 must NOT have been skipped as a redelivery.
	tag2 := safety.CompanionStopTagForEntry(2)
	var stops2Total int
	var stops2 int
	for _, s := range f.broker.StopOrders() {
		if s.Tag != tag2 {
			continue
		}
		stops2++
		require.Equal(t, backtester_models.TradierOrderSideSell, s.Sides[0], "a long entry is protected by sell stops")
		require.InDelta(t, 95.0, *s.StopPrice, 1e-9)
		stops2Total += s.Quantities[0]
	}
	require.Equal(t, 10, stops2Total, "total stop quantity must equal total filled quantity — no unprotected remainder")
	require.Equal(t, 2, stops2, "each netting trade tops up coverage for its delta")
	require.Empty(t, safety.UnprotectedPositions())
}

// stopRejectingBroker delegates everything to MockBroker but rejects stop
// orders, simulating a broker that accepted the entry but refuses the
// protective stop.
type stopRejectingBroker struct {
	*backtester_models.MockBroker
}

func (b *stopRejectingBroker) PlaceOrder(ctx context.Context, req *backtester_models.PlaceOrderRequest) (map[string]interface{}, error) {
	if req.OrderType == backtester_models.TradierOrderTypeStop {
		return nil, fmt.Errorf("broker rejected stop order")
	}
	return b.MockBroker.PlaceOrder(ctx, req)
}

// A companion-stop placement failure must leave the fill committed and be
// loud: failure counter plus an unprotected-position record naming the symbol
// and quantity for the operator alert.
func TestCompanionStop_BrokerErrorLeavesFillCommittedAndAlerts(t *testing.T) {
	telemetry.Init()
	mock := backtester_models.NewMockBroker(1000, nil)
	f := newGuardPipelineFixtureWithBroker(t, &stopRejectingBroker{MockBroker: mock}, mock)
	stopper := installCompanionStopper(t, 5.0)

	liveOrder := f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	// The fill stays committed — a stop failure never fails or rolls back the
	// fill.
	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)

	// No stop reached the broker; the failure is loud.
	require.Empty(t, f.broker.StopOrders())
	require.Equal(t, 0.0, companionCounter(t, "safety_companion_stops_placed_total"))
	require.Equal(t, 1.0, companionCounter(t, "safety_companion_stop_failures_total"))

	unprotected := stopper.UnprotectedPositions()
	require.Len(t, unprotected, 1)
	require.Equal(t, string(f.symbol), unprotected[0].Symbol)
	require.Equal(t, 19, unprotected[0].Quantity)
	require.Equal(t, liveOrder.ID, unprotected[0].EntryOrderID)

	// The alert-engine provider surfaces the same record.
	require.Len(t, safety.UnprotectedPositions(), 1)
}

// With no stopper installed (COMPANION_STOP_DISTANCE unset) the pipeline is
// byte-for-byte unchanged: fills commit, nothing reaches the stop path.
func TestCompanionStop_UnarmedPipelinePlacesNoStops(t *testing.T) {
	telemetry.Init()
	safety.SetCompanionStopper(nil)
	f := newGuardPipelineFixture(t)

	f.placeOrder(t, 1, 19)
	reconcileOrder := f.lastReconcileOrder(t)
	require.NoError(t, f.broker.FillOrder(*reconcileOrder.ExternalOrderID, 100.0, string(backtester_models.OrderRecordStatusFilled)))
	f.pumpPipeline(t)

	liveOrders := f.livePlayground.GetAllOrders()
	require.Len(t, liveOrders, 1)
	require.Equal(t, backtester_models.OrderRecordStatusFilled, liveOrders[0].Status)
	require.Empty(t, f.broker.StopOrders())
	require.Equal(t, 0.0, companionCounter(t, "safety_companion_stops_placed_total"))
}

// Simulation-Mode fills must never reach the stop path even with the stopper
// armed (eligibility excludes Simulation), keeping replays byte-for-byte
// unchanged.
func TestCompanionStop_SimulationFillPlacesNoStop(t *testing.T) {
	telemetry.Init()
	stopper := installCompanionStopper(t, 5.0)

	sim := &backtester_models.Playground{Meta: backtester_models.Meta{Mode: backtester_models.ModeSimulation, LegacyEnv: "simulator"}}
	order := &backtester_models.OrderRecord{
		Class:            backtester_models.OrderRecordClassEquity,
		Side:             backtester_models.TradierOrderSideBuy,
		Symbol:           "AAPL",
		AbsoluteQuantity: 10,
	}
	order.ID = 1

	maybePlaceCompanionStop(sim, order, 100.0, 10.0)

	require.Equal(t, 0.0, companionCounter(t, "safety_companion_stops_placed_total"))
	require.Equal(t, 0.0, companionCounter(t, "safety_companion_stop_failures_total"))
	require.Empty(t, stopper.UnprotectedPositions())
}

// Interface conformance guard for the wrapping broker.
var _ backtester_models.IBroker = (*stopRejectingBroker)(nil)
