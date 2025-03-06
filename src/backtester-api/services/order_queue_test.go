package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func createPlayground(t *testing.T, symbol eventmodels.Instrument, clock *models.Clock, feed []*eventmodels.PolygonAggregateBarV2, env models.PlaygroundEnvironment) (*models.Playground, error) {
	period := time.Minute
	source := eventmodels.CandleRepositorySource{
		Type: "test",
	}

	repo, err := models.NewCandleRepository(symbol, period, feed, []string{}, nil, 0, source)
	require.NoError(t, err)

	balance := 1000.0
	playground, err := models.NewPlayground(nil, nil, nil, balance, balance, clock, nil, env, clock.CurrentTime, []string{}, repo)
	return playground, err
}

func TestLiveAccount(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	clock := models.NewClock(startTime, endTime, nil)
	t_minus_1 := startTime.Add(-time.Minute)
	t1 := startTime.Add(time.Minute)
	t2 := startTime.Add(2 * time.Minute)

	feed := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: t_minus_1,
			Close:     5.0,
		},
		{
			Timestamp: startTime,
			Close:     10.0,
		},
		{
			Timestamp: t1,
			Close:     20.0,
		},
		{
			Timestamp: t2,
			Close:     30.0,
		},
	}

	// source := models.NewMockLiveAccountSource()
	now := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	symbol := eventmodels.NewStockSymbol("AAPL")

	createReconcilePlayground := func(t *testing.T, playground *models.Playground, liveAccount *models.LiveAccount) *models.ReconcilePlayground {
		reconcilePlayground, err := models.NewReconcilePlayground(playground, liveAccount)
		require.NoError(t, err)

		return reconcilePlayground
	}

	createLivePlayground := func(t *testing.T, playgroundId uuid.UUID, broker models.IBroker, database models.IDatabaseService, newTradesQueue *eventmodels.FIFOQueue[*models.TradeRecord]) *models.Playground {
		clientId := "test"
		startingBalance := 1000.0
		orders := []*models.OrderRecord{}
		tags := []string{}

		repo, err := models.NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, eventmodels.CandleRepositorySource{})
		require.NoError(t, err)

		repositories := []*models.CandleRepository{repo}
		env := models.PlaygroundEnvironmentLive
		livePlayground, err := models.NewPlayground(&playgroundId, nil, &clientId, startingBalance, startingBalance, nil, orders, env, now, tags, repositories...)
		require.NoError(t, err)

		livePlayground.SetNewTradesQueue(newTradesQueue)

		return livePlayground
	}

	t.Run("place buy order", func(t *testing.T) {
		broker := models.NewMockBroker(1000)
		database := models.NewMockDatabase()
		newTradesQueue := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 1)
		liveAccount, err := models.NewLiveAccount(broker, database)
		require.NoError(t, err)

		playground, err := createPlayground(t, symbol, clock, feed, models.PlaygroundEnvironmentReconcile)
		require.NoError(t, err)

		reconcilePlayground := createReconcilePlayground(t, playground, liveAccount)

		playgroundID, err := uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)
		livePlayground := createLivePlayground(t, playgroundID, broker, database, newTradesQueue)

		err = database.SavePlaygroundSession(reconcilePlayground.GetPlayground())
		require.NoError(t, err)

		err = database.SavePlaygroundSession(livePlayground)
		require.NoError(t, err)

		// place buy order
		order := models.NewOrderRecord(1, nil, livePlayground.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMock, now, symbol, models.TradierOrderSideBuy, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges, err := livePlayground.PlaceOrder(order)
		require.NoError(t, err)
		require.Len(t, placeOrderChanges, 3)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		orders := livePlayground.GetOrders()
		require.Len(t, orders, 1)
		liveOrder := orders[0]
		require.Equal(t, order, liveOrder)

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetOrders()[0]

		require.NotEqual(t, order.ID, reconcileOrder.ID)
		require.Equal(t, order.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order.Side, reconcileOrder.Side)
		require.Equal(t, order.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order.OrderType, reconcileOrder.OrderType)
		require.Equal(t, order.Status, reconcileOrder.Status)
		require.Equal(t, order.Tag, reconcileOrder.Tag)
	})

	t.Run("fill buy order - with existing long order", func(t *testing.T) {
		reconcileOrderIdx := uint(1000)
		broker := models.NewMockBroker(reconcileOrderIdx)
		database := models.NewMockDatabase()
		newTradesQueue1 := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 2)
		liveAccount1, err := models.NewLiveAccount(broker, database)
		require.NoError(t, err)

		playgroundID, err := uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)
		livePlayground1 := createLivePlayground(t, playgroundID, broker, database, newTradesQueue1)

		reconcilePlayground := createReconcilePlayground(t, livePlayground1, liveAccount1)
		liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 2)
		cache := models.NewOrderCache()

		// save playgrounds
		err = database.SavePlaygroundSession(reconcilePlayground.GetPlayground())
		require.NoError(t, err)

		err = database.SavePlaygroundSession(livePlayground1)
		require.NoError(t, err)

		// place buy order
		order1 := models.NewOrderRecord(1, nil, livePlayground1.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMock, now, symbol, models.TradierOrderSideBuy, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		broker.SetFillOrderExecutionPrice(100.0)

		placeOrderChanges, err := livePlayground1.PlaceOrder(order1)
		require.NoError(t, err)
		require.Len(t, placeOrderChanges, 3)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetOrders()[0]

		require.Equal(t, reconcileOrderIdx, reconcileOrder.ID)
		require.Equal(t, order1.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order1.Side, reconcileOrder.Side)
		require.Equal(t, order1.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order1.OrderType, reconcileOrder.OrderType)

		// assert - reconcile order appears in live playground
		reconcileOrders := livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 1)
		require.Equal(t, reconcileOrder, reconcileOrders[0])

		// assert - live order is placed
		liveOrders := livePlayground1.GetOrders()
		require.Len(t, liveOrders, 1)
		require.Equal(t, order1, liveOrders[0])

		// assert - order status is pending
		require.Equal(t, models.OrderRecordStatusPending, reconcileOrders[0].Status)
		require.Equal(t, models.OrderRecordStatusPending, liveOrders[0].Status)

		// apiService :=
		// fill buy order
		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err := DrainTradierOrderQueue(liveOrdersUpdateQueue, cache, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = CommitPendingOrders(cache, database)
		require.NoError(t, err)

		// assert - live order is filled
		liveOrders = livePlayground1.GetOrders()
		require.Len(t, liveOrders, 1)
		require.Equal(t, models.OrderRecordStatusFilled, liveOrders[0].Status)

		// assert - reconciliation order is filled
		reconcileOrders = livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 1)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[0].Status)

		reconcilePos, err := reconcilePlayground.GetPlayground().GetPosition(order1.GetInstrument(), true)
		require.NoError(t, err)
		require.Equal(t, order1.GetQuantity(), reconcilePos.Quantity)

		// assert - new trades are created
		trRec, ok := newTradesQueue1.Dequeue()
		require.True(t, ok)
		require.Equal(t, order1.ID, trRec.OrderID)
		require.Equal(t, order1.GetQuantity(), trRec.Quantity)

		_, ok = newTradesQueue1.Dequeue()
		require.False(t, ok)

		// place sell order
		playgroundID, err = uuid.Parse("3b208041-9c52-4221-b514-8d15385d310f")
		require.NoError(t, err)
		newTradesQueue2 := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 2)
		livePlayground2 := createLivePlayground(t, playgroundID, broker, database, newTradesQueue2)
		order2 := models.NewOrderRecord(2, nil, livePlayground2.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMock, now, symbol, models.TradierOrderSideSellShort, 20, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		// save playground
		err = database.SavePlaygroundSession(livePlayground2)
		require.NoError(t, err)

		placeOrderChanges, err = livePlayground2.PlaceOrder(order2)
		require.NoError(t, err)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		require.Len(t, livePlayground2.GetOrders(), 1)
		liveOrder := livePlayground2.GetOrders()[0]
		require.Equal(t, order2, liveOrder)

		// assert - reconciliation order is placed
		reconcileOrders = reconcilePlayground.GetPlayground().GetOrders()
		require.Len(t, reconcileOrders, 3)

		require.Equal(t, order1.Symbol, reconcileOrders[1].Symbol)
		require.Equal(t, models.TradierOrderSideSell, reconcileOrders[1].Side)
		require.Equal(t, 19.0, reconcileOrders[1].AbsoluteQuantity)
		require.Equal(t, order1.OrderType, reconcileOrders[1].OrderType)

		require.Equal(t, order2.Symbol, reconcileOrders[2].Symbol)
		require.Equal(t, models.TradierOrderSideSellShort, reconcileOrders[2].Side)
		require.Equal(t, 1.0, reconcileOrders[2].AbsoluteQuantity)
		require.Equal(t, order2.OrderType, reconcileOrders[2].OrderType)

		// fill sell short order
		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err = DrainTradierOrderQueue(liveOrdersUpdateQueue, cache, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = CommitPendingOrders(cache, database)
		require.NoError(t, err)

		// assert - live order is filled
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[1].Status)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[2].Status)
		require.Equal(t, models.OrderRecordStatusFilled, liveOrder.Status)

		// assert - new trades are created
		trRec, ok = newTradesQueue2.Dequeue()
		require.True(t, ok)
		require.Equal(t, order2.ID, trRec.OrderID)
		require.Equal(t, order1.GetQuantity()*-1, trRec.Quantity)

		trRec, ok = newTradesQueue2.Dequeue()
		require.True(t, ok)
		require.Equal(t, order2.ID, trRec.OrderID)
		diff := order1.GetQuantity() + order2.GetQuantity()
		require.Equal(t, diff, trRec.Quantity)
	})
}
