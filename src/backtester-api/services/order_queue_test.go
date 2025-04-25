package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestLiveAccount(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	// clock := models.NewClock(startTime, endTime, nil)
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

	createReconcilePlayground := func(t *testing.T, playground *models.Playground, liveAccount *models.LiveAccount, database *models.MockDatabase) *models.ReconcilePlayground {
		reconcilePlayground, err := models.NewReconcilePlayground(playground, liveAccount)
		require.NoError(t, err)

		source, err := playground.GetSource()
		require.NoError(t, err)

		database.SetReconcilePlayground(source, reconcilePlayground)

		return reconcilePlayground
	}

	createLivePlayground := func(t *testing.T, playgroundId uuid.UUID, reconcilePlayground models.IReconcilePlayground, liveAccount models.ILiveAccount, broker models.IBroker, database models.IDatabaseService, newTradesQueue *eventmodels.FIFOQueue[*models.TradeRecord]) *models.Playground {
		clientId := "test"
		startingBalance := 1000.0
		orders := []*models.OrderRecord{}
		tags := []string{}

		repo, err := models.NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, eventmodels.CandleRepositorySource{})
		require.NoError(t, err)

		accountRequestSource := models.NewMockLiveAccountSource()
		s := &models.CreateAccountRequestSource{
			Broker:          accountRequestSource.GetBroker(),
			AccountID:       accountRequestSource.GetAccountID(),
			LiveAccountType: accountRequestSource.GetAccountType(),
		}

		repositories := []*models.CandleRepository{repo}
		env := models.PlaygroundEnvironmentLive

		req := &models.PopulatePlaygroundRequest{
			ID:                  &playgroundId,
			ClientID:            &clientId,
			Env:                 env,
			Account:             models.CreateAccountRequest{Balance: startingBalance, Source: s},
			InitialBalance:      startingBalance,
			BackfillOrders:      orders,
			Tags:                tags,
			LiveAccount:         liveAccount,
			ReconcilePlayground: reconcilePlayground,
		}

		livePlayground := &models.Playground{}
		err = models.PopulatePlayground(livePlayground, req, nil, now, newTradesQueue, repositories...)
		require.NoError(t, err)

		return livePlayground
	}

	createMockReconcilePlayground := func(playgroundID *uuid.UUID, database models.IDatabaseService, broker models.IBroker, liveAccount *models.LiveAccount) (*models.Playground, error) {
		playground := &models.Playground{}
		source := models.CreateAccountRequestSource{
			Broker:          broker.GetSource().GetBroker(),
			AccountID:       broker.GetSource().GetAccountID(),
			LiveAccountType: broker.GetSource().GetAccountType(),
		}

		err := database.CreatePlayground(playground, &models.PopulatePlaygroundRequest{
			ID:  playgroundID,
			Env: models.PlaygroundEnvironmentReconcile,
			Account: models.CreateAccountRequest{
				Balance: 1000.0,
				Source:  &source,
			},
			Clock: models.CreateClockRequest{
				StartDate: startTime.Format("2006-01-02"),
				StopDate:  endTime.Format("2006-01-02"),
			},
			Repositories: nil,
			CreatedAt:    now,
			LiveAccount:  liveAccount,
			SaveToDB:     true,
		})

		return playground, err
	}

	t.Run("place buy order", func(t *testing.T) {
		broker := models.NewMockBroker(1000, nil)
		database := models.NewMockDatabase()
		newTradesQueue := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 1)
		liveAccount, err := models.NewLiveAccount(broker, database)
		require.NoError(t, err)

		playgroundID, err := uuid.Parse("b352821e-8317-4ee6-aeb8-5b5772e1087a")
		require.NoError(t, err)
		playground, err := createMockReconcilePlayground(&playgroundID, database, broker, liveAccount)
		require.NoError(t, err)

		require.NoError(t, err)
		reconcilePlayground := createReconcilePlayground(t, playground, liveAccount, database)

		playgroundID, err = uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)
		livePlayground := createLivePlayground(t, playgroundID, reconcilePlayground, liveAccount, broker, database, newTradesQueue)

		err = database.SavePlaygroundSession(reconcilePlayground.GetPlayground())
		require.NoError(t, err)

		err = database.SavePlaygroundSession(livePlayground)
		require.NoError(t, err)

		// place buy order
		order := models.NewOrderRecord(1, nil, nil, livePlayground.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMock, now, symbol, models.TradierOrderSideBuy, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges, err := livePlayground.PlaceOrder(order)
		require.NoError(t, err)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		orders := livePlayground.GetAllOrders()
		require.Len(t, orders, 1)
		liveOrder := orders[0]
		require.Equal(t, order, liveOrder)

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetAllOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetAllOrders()[0]

		require.NotEqual(t, order.ExternalOrderID, reconcileOrder.ExternalOrderID)
		require.Equal(t, order.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order.Side, reconcileOrder.Side)
		require.Equal(t, order.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order.OrderType, reconcileOrder.OrderType)
		require.Equal(t, order.Status, reconcileOrder.Status)
		require.Equal(t, order.Tag, reconcileOrder.Tag)
	})

	t.Run("fill buy order - with existing long order", func(t *testing.T) {
		reconcileOrderIdx := uint(1000)
		broker := models.NewMockBroker(reconcileOrderIdx, nil)
		database := models.NewMockDatabase()
		newTradesQueue1 := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 2)
		liveAccount, err := models.NewLiveAccount(broker, database)
		require.NoError(t, err)

		playgroundID, err := uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)
		playground1, err := createMockReconcilePlayground(&playgroundID, database, broker, liveAccount)
		require.NoError(t, err)

		reconcilePlayground := createReconcilePlayground(t, playground1, liveAccount, database)
		liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 2)

		playgroundID, err = uuid.Parse("c59a5f72-7989-4457-9120-f281924e7e0e")
		require.NoError(t, err)
		livePlayground1 := createLivePlayground(t, playgroundID, reconcilePlayground, liveAccount, broker, database, newTradesQueue1)

		// save playgrounds
		err = database.SavePlaygroundSession(reconcilePlayground.GetPlayground())
		require.NoError(t, err)

		err = database.SavePlaygroundSession(livePlayground1)
		require.NoError(t, err)

		// place buy order
		order1 := models.NewOrderRecord(1, nil, nil, livePlayground1.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMargin, now, symbol, models.TradierOrderSideBuy, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges, err := livePlayground1.PlaceOrder(order1)
		require.NoError(t, err)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetAllOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetAllOrders()[0]

		require.NotNil(t, reconcileOrder.ExternalOrderID)
		require.Equal(t, reconcileOrderIdx, *reconcileOrder.ExternalOrderID)
		require.Equal(t, order1.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order1.Side, reconcileOrder.Side)
		require.Equal(t, order1.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order1.OrderType, reconcileOrder.OrderType)

		// assert - reconcile order appears in live playground
		reconcileOrders := livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 1)
		require.Equal(t, reconcileOrder, reconcileOrders[0])

		// assert - live order is placed
		liveOrders := livePlayground1.GetAllOrders()
		require.Len(t, liveOrders, 1)
		require.Equal(t, order1, liveOrders[0])

		// assert - order status is pending
		require.Equal(t, models.OrderRecordStatusPending, reconcileOrders[0].Status)
		require.Equal(t, models.OrderRecordStatusPending, liveOrders[0].Status)

		// fill buy order
		require.NotNil(t, reconcileOrders[0].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[0].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err := DrainTradierOrderQueue(liveOrdersUpdateQueue, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = UpdatePendingMarginOrders(database)
		require.NoError(t, err)

		// assert - live order is filled
		liveOrders = livePlayground1.GetAllOrders()
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
		require.NotNil(t, trRec)
		require.NotNil(t, trRec.ReconcileOrderID)

		require.Equal(t, order1.ID, *trRec.ReconcileOrderID)
		require.Equal(t, order1.GetQuantity(), trRec.Quantity)

		_, ok = newTradesQueue1.Dequeue()
		require.False(t, ok)

		// place sell order
		playgroundID, err = uuid.Parse("3b208041-9c52-4221-b514-8d15385d310f")
		require.NoError(t, err)
		newTradesQueue2 := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 2)

		livePlayground2 := createLivePlayground(t, playgroundID, reconcilePlayground, liveAccount, broker, database, newTradesQueue2)

		order2 := models.NewOrderRecord(2, nil, nil, livePlayground2.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMargin, now, symbol, models.TradierOrderSideSellShort, 20, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

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
		require.Len(t, livePlayground2.GetAllOrders(), 1)
		liveOrder := livePlayground2.GetAllOrders()[0]
		require.Equal(t, order2, liveOrder)

		// assert - reconciliation order is placed
		reconcileOrders = reconcilePlayground.GetPlayground().GetAllOrders()
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
		require.NotNil(t, reconcileOrders[1].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[1].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		require.NotNil(t, reconcileOrders[2].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[2].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err = DrainTradierOrderQueue(liveOrdersUpdateQueue, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = UpdatePendingMarginOrders(database)
		require.NoError(t, err)

		// assert - live order is filled
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[1].Status)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[2].Status)
		require.Equal(t, models.OrderRecordStatusFilled, liveOrder.Status)

		// assert - new trades are created
		trRec, ok = newTradesQueue2.Dequeue()
		require.True(t, ok)
		require.NotNil(t, trRec)
		require.NotNil(t, trRec.ReconcileOrderID)

		require.Equal(t, order2.ID, *trRec.ReconcileOrderID)
		require.Equal(t, order1.GetQuantity()*-1, trRec.Quantity)

		trRec, ok = newTradesQueue2.Dequeue()
		require.True(t, ok)
		require.NotNil(t, trRec)
		require.NotNil(t, trRec.OrderID)

		require.Equal(t, order2.ID, *trRec.OrderID)
		diff := order1.GetQuantity() + order2.GetQuantity()
		require.Equal(t, diff, trRec.Quantity)
	})

	t.Run("fill orders out of sequence", func(t *testing.T) {
		reconcileOrderIdx := uint(1000)
		broker := models.NewMockBroker(reconcileOrderIdx, nil)
		database := models.NewMockDatabase()
		newTradesQueue1 := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 3)
		liveAccount, err := models.NewLiveAccount(broker, database)
		require.NoError(t, err)

		playgroundID, err := uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)
		playground1, err := createMockReconcilePlayground(&playgroundID, database, broker, liveAccount)
		require.NoError(t, err)

		reconcilePlayground := createReconcilePlayground(t, playground1, liveAccount, database)
		liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 2)

		playgroundID, err = uuid.Parse("c59a5f72-7989-4457-9120-f281924e7e0e")
		require.NoError(t, err)
		livePlayground1 := createLivePlayground(t, playgroundID, reconcilePlayground, liveAccount, broker, database, newTradesQueue1)

		// save playgrounds
		err = database.SavePlaygroundSession(reconcilePlayground.GetPlayground())
		require.NoError(t, err)

		err = database.SavePlaygroundSession(livePlayground1)
		require.NoError(t, err)

		// place buy order
		order1 := models.NewOrderRecord(1, nil, nil, livePlayground1.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMargin, now, symbol, models.TradierOrderSideBuy, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges, err := livePlayground1.PlaceOrder(order1)
		require.NoError(t, err)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// fill buy order
		reconcileOrders := livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 1)

		require.NotNil(t, reconcileOrders[0].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[0].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err := DrainTradierOrderQueue(liveOrdersUpdateQueue, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = UpdatePendingMarginOrders(database)
		require.NoError(t, err)

		// assert - live order is filled
		reconcileOrders = livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 1)
		require.Equal(t, models.TradierOrderSideBuy, reconcileOrders[0].Side)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[0].Status)
		require.Equal(t, 19.0, reconcileOrders[0].AbsoluteQuantity)
		require.Len(t, reconcileOrders[0].Reconciles, 1)
		require.Equal(t, order1.ID, reconcileOrders[0].Reconciles[0].ID)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[0].Reconciles[0].Status)

		liveOrders := livePlayground1.GetAllOrders()
		require.Len(t, liveOrders, 1)
		require.Equal(t, models.OrderRecordStatusFilled, liveOrders[0].Status)

		// place sell order
		order2 := models.NewOrderRecord(2, nil, nil, livePlayground1.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMargin, now, symbol, models.TradierOrderSideSell, 19, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges2, err := livePlayground1.PlaceOrder(order2)
		require.NoError(t, err)

		for _, change := range placeOrderChanges2 {
			err := change.Commit()
			require.NoError(t, err)
		}

		liveOrders = livePlayground1.GetAllOrders()
		require.Len(t, liveOrders, 2)
		require.Equal(t, models.OrderRecordStatusPending, liveOrders[1].Status)

		// do not fill order - yet
		reconcileOrders = livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 2)
		require.Equal(t, models.TradierOrderSideSell, reconcileOrders[1].Side)
		require.Equal(t, models.OrderRecordStatusPending, reconcileOrders[1].Status)
		require.Equal(t, 19.0, reconcileOrders[1].AbsoluteQuantity)

		// place sell short order
		order3 := models.NewOrderRecord(3, nil, nil, livePlayground1.GetId(), models.OrderRecordClassEquity, models.LiveAccountTypeMargin, now, symbol, models.TradierOrderSideSellShort, 5, models.Market, models.Day, 0.01, nil, nil, models.OrderRecordStatusPending, "", nil)

		placeOrderChanges3, err := livePlayground1.PlaceOrder(order3)
		require.NoError(t, err)

		for _, change := range placeOrderChanges3 {
			err := change.Commit()
			require.NoError(t, err)
		}

		liveOrders = livePlayground1.GetAllOrders()
		require.Len(t, liveOrders, 3)
		require.Equal(t, models.OrderRecordStatusNew, liveOrders[2].Status)

		// order #3 (sell short) not available to fill
		reconcileOrders = livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 2)
		allOrders := livePlayground1.GetAllOrders()
		require.Len(t, allOrders, 3)
		require.Equal(t, order3.ID, allOrders[2].ID)

		position, err := livePlayground1.GetPosition(symbol, true)
		require.NoError(t, err)
		require.Equal(t, 19.0, position.Quantity)

		// fill order #2
		require.NotNil(t, reconcileOrders[1].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[1].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err = DrainTradierOrderQueue(liveOrdersUpdateQueue, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = UpdatePendingMarginOrders(database)
		require.NoError(t, err)

		// assert - live order #2 is filled
		reconcileOrders = livePlayground1.GetReconcilePlayground().GetOrders()
		require.Len(t, reconcileOrders, 3)

		require.Equal(t, models.TradierOrderSideSell, reconcileOrders[1].Side)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[1].Status)
		require.Equal(t, 19.0, reconcileOrders[1].AbsoluteQuantity)
		require.Len(t, reconcileOrders[1].Reconciles, 1)
		require.Equal(t, order2.ID, reconcileOrders[1].Reconciles[0].ID)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[1].Reconciles[0].Status)

		require.Equal(t, models.OrderRecordStatusPending, reconcileOrders[2].Status)

		// fill order #3
		require.NotNil(t, reconcileOrders[2].ExternalOrderID)
		err = broker.FillOrder(*reconcileOrders[2].ExternalOrderID, 100.0, string(models.OrderRecordStatusFilled))
		require.NoError(t, err)

		err = UpdateTradierOrderQueue(liveOrdersUpdateQueue, database, 0)
		require.NoError(t, err)

		hasUpdates, err = DrainTradierOrderQueue(liveOrdersUpdateQueue, database)
		require.NoError(t, err)
		require.True(t, hasUpdates)

		err = UpdatePendingMarginOrders(database)
		require.NoError(t, err)

		// assert - live order #3 is filled
		require.Equal(t, models.TradierOrderSideSellShort, reconcileOrders[2].Side)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[2].Status)
		require.Equal(t, 5.0, reconcileOrders[2].AbsoluteQuantity)
		require.Len(t, reconcileOrders[2].Reconciles, 1)
		require.Equal(t, order3.ID, reconcileOrders[2].Reconciles[0].ID)
		require.Equal(t, models.OrderRecordStatusFilled, reconcileOrders[2].Reconciles[0].Status)
	})
}
