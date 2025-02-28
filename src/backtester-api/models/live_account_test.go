package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func createPlayground(t *testing.T, symbol eventmodels.Instrument, clock *Clock, feed []*eventmodels.PolygonAggregateBarV2, env PlaygroundEnvironment) (*Playground, error) {
	period := time.Minute
	source := eventmodels.CandleRepositorySource{
		Type: "test",
	}

	repo, err := NewCandleRepository(symbol, period, feed, []string{}, nil, 0, source)
	require.NoError(t, err)

	balance := 1000.0
	playground, err := NewPlayground(nil, nil, balance, balance, clock, nil, env, clock.CurrentTime, []string{}, repo)
	return playground, err
}

func TestLiveAccount(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	clock := NewClock(startTime, endTime, nil)
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

	source := NewMockLiveAccountSource()
	now := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	symbol := eventmodels.NewStockSymbol("AAPL")

	createReconcilePlayground := func(t *testing.T) *ReconcilePlayground {
		playground, err := createPlayground(t, symbol, clock, feed, PlaygroundEnvironmentReconcile)
		require.NoError(t, err)

		reconcilePlayground, err := NewReconcilePlayground(playground)
		require.NoError(t, err)

		return reconcilePlayground
	}

	createLivePlayground := func(t *testing.T, broker IBroker, reconcilePlayground *ReconcilePlayground) *LivePlayground {
		account, err := NewLiveAccount(source, broker, reconcilePlayground)
		require.NoError(t, err)

		playgroundID, err := uuid.Parse("5ac6cb3a-5182-4330-96f6-297f0bb99ac1")
		require.NoError(t, err)

		clientId := "test"
		startingBalance := 1000.0
		orders := []*BacktesterOrder{}
		tags := []string{}

		repo, err := NewCandleRepository(symbol, time.Minute, feed, []string{}, nil, 0, eventmodels.CandleRepositorySource{})
		require.NoError(t, err)

		repositories := []*CandleRepository{repo}
		database := NewMockDatabase()
		livePlayground, err := NewLivePlayground(&playgroundID, database, &clientId, account, startingBalance, repositories, nil, nil, orders, now, tags)
		require.NoError(t, err)

		return livePlayground
	}

	t.Run("place order - new", func(t *testing.T) {
		broker := NewMockBroker()
		reconcilePlayground := createReconcilePlayground(t)
		livePlayground := createLivePlayground(t, broker, reconcilePlayground)

		// place buy order
		order := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 19, Market, Day, 0.01, nil, nil, BacktesterOrderStatusPending, "")

		placeOrderChanges, err := livePlayground.PlaceOrder(order)
		require.NoError(t, err)
		require.Len(t, placeOrderChanges, 2)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		require.Len(t, livePlayground.GetOrders(), 1)
		liveOrder := livePlayground.GetOrders()[0]
		require.Equal(t, order, liveOrder)

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetOrders()[0]

		require.NotEqual(t, order.ID, reconcileOrder.ID)
		require.Equal(t, order.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order.Side, reconcileOrder.Side)
		require.Equal(t, order.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order.Type, reconcileOrder.Type)
		require.Equal(t, order.Status, reconcileOrder.Status)
		require.Equal(t, order.Tag, reconcileOrder.Tag)
	})

	t.Run("place buy order - existing long", func(t *testing.T) {
		broker := NewMockBroker()
		reconcilePlayground := createReconcilePlayground(t)

		// place buy order
		livePlayground1 := createLivePlayground(t, broker, reconcilePlayground)
		order1 := NewBacktesterOrder(1, BacktesterOrderClassEquity, now, symbol, TradierOrderSideBuy, 19, Market, Day, 0.01, nil, nil, BacktesterOrderStatusPending, "")

		placeOrderChanges, err := livePlayground1.PlaceOrder(order1)
		require.NoError(t, err)
		require.Len(t, placeOrderChanges, 2)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		require.Len(t, livePlayground1.GetOrders(), 1)
		liveOrder := livePlayground1.GetOrders()[0]
		require.Equal(t, order1, liveOrder)

		// assert - reconciliation order is placed
		require.Len(t, reconcilePlayground.GetPlayground().GetOrders(), 1)
		reconcileOrder := reconcilePlayground.GetPlayground().GetOrders()[0]

		require.NotEqual(t, order1.ID, reconcileOrder.ID)
		require.Equal(t, order1.Symbol, reconcileOrder.Symbol)
		require.Equal(t, order1.Side, reconcileOrder.Side)
		require.Equal(t, order1.AbsoluteQuantity, reconcileOrder.AbsoluteQuantity)
		require.Equal(t, order1.Type, reconcileOrder.Type)

		// fill the orders
		executionFillMap := make(map[uint]ExecutionFillRequest)
		executionFillMap[reconcileOrder.ID] = ExecutionFillRequest{
			PlaygroundId: reconcilePlayground.GetId(),
			Price:        100.0,
			Quantity:     19,
		}

		_, _, err = reconcilePlayground.CommitPendingOrders(executionFillMap)
		require.NoError(t, err)

		// place sell order
		livePlayground2 := createLivePlayground(t, broker, reconcilePlayground)
		order2 := NewBacktesterOrder(2, BacktesterOrderClassEquity, now, symbol, TradierOrderSideSellShort, 20, Market, Day, 0.01, nil, nil, BacktesterOrderStatusPending, "")

		placeOrderChanges, err = livePlayground2.PlaceOrder(order2)
		require.NoError(t, err)

		for _, change := range placeOrderChanges {
			err := change.Commit()
			require.NoError(t, err)
		}

		// assert - live order is placed
		require.Len(t, livePlayground2.GetOrders(), 1)
		liveOrder = livePlayground2.GetOrders()[0]
		require.Equal(t, order2, liveOrder)

		// assert - reconciliation order is placed
		reconcileOrders := reconcilePlayground.GetPlayground().GetOrders()
		require.Len(t, reconcileOrders, 3)

		require.Equal(t, order1.Symbol, reconcileOrders[1].Symbol)
		require.Equal(t, TradierOrderSideSell, reconcileOrders[1].Side)
		require.Equal(t, 19.0, reconcileOrders[1].AbsoluteQuantity)
		require.Equal(t, order1.Type, reconcileOrders[1].Type)

		require.Equal(t, order2.Symbol, reconcileOrders[2].Symbol)
		require.Equal(t, TradierOrderSideSellShort, reconcileOrders[2].Side)
		require.Equal(t, 1.0, reconcileOrders[2].AbsoluteQuantity)
		require.Equal(t, order2.Type, reconcileOrders[2].Type)
	})
}
