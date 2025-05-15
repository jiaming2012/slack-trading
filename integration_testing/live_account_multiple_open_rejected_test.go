package integrationtesting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccountMultipleOpenRejected(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// Start main app container
	p := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	createLivePgResp, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
		Balance:     10000,
		Broker:      "tradier",
		AccountType: "mock",
		Repositories: []*playground.Repository{
			{
				Symbol:             "AAPL",
				TimespanMultiplier: 1,
				TimespanUnit:       "minute",
				Indicators:         []string{},
				HistoryInDays:      0,
			},
		},
		Environment: "live",
	})

	require.NoError(t, err)

	// Fetch reconcile account initial order count
	liveAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  false,
	})
	require.NoError(t, err)

	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})
	require.NoError(t, err)

	reconcileAccountOrdersInitialCount := len(reconcileAccount.Orders)

	// Place an order
	placeOrderResp1, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   createLivePgResp.Id,
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "sell_short",
		Type:           "market",
		RequestedPrice: 177.0,
		Duration:       "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp1)

	// Fetch the live account
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)

	// Fill the order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	require.Len(t, reconcileAccount.Orders, reconcileAccountOrdersInitialCount+1)
	require.Len(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Reconciles, 1)
	require.Equal(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Reconciles[0].Id, placeOrderResp1.Id)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[reconcileAccountOrdersInitialCount].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	now := time.Now()
	var openTradeId *uint64
	for {
		if time.Since(now) > time.Second*40 {
			break
		}

		uId := uuid.NewString()

		nextTickResponse, err := p.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: createLivePgResp.Id,
			RequestId:    uId,
		})
		require.NoError(t, err)

		if len(nextTickResponse.NewTrades) > 0 {
			openTradeId = &nextTickResponse.NewTrades[0].Id
			break
		}

		time.Sleep(time.Second) // Wait for the order to be filled
	}

	require.NotNil(t, openTradeId)

	// order 2: buy_to_cover
	placeOrderResp2, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   createLivePgResp.Id,
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "buy_to_cover",
		Type:           "market",
		RequestedPrice: 177.0,
		Duration:       "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp2)

	// order 3: buy
	placeOrderResp3, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   createLivePgResp.Id,
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "buy",
		Type:           "market",
		RequestedPrice: 177.0,
		Duration:       "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp3)

	// Reject the second order: buy_to_cover
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, reconcileAccountOrdersInitialCount+2)
	require.Len(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Reconciles, 1)
	require.Equal(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Reconciles[0].Id, placeOrderResp2.Id)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].ExternalId,
		Price:   178.0,
		Status:  "rejected",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	invalidOrdersCount := 0
	openTradeCount := 0
	now = time.Now()
	for {
		if time.Since(now) > time.Second*40 {
			break
		}

		uId := uuid.NewString()

		nextTickResponse, err := p.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: createLivePgResp.Id,
			RequestId:    uId,
		})
		require.NoError(t, err)

		invalidOrdersCount += len(nextTickResponse.InvalidOrders)

		if len(nextTickResponse.NewTrades) > 0 {
			openTradeCount += len(nextTickResponse.NewTrades)
			break
		}

		if invalidOrdersCount > 0 {
			break
		}

		time.Sleep(time.Second) // Wait for the order to be filled
	}

	require.NotNil(t, openTradeId)
	require.Equal(t, 0, openTradeCount)
	require.Equal(t, 1, invalidOrdersCount)

	// Re-fetch the live account
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	// Wait until order 3 is rejected

	require.NoError(t, err)
	require.NotNil(t, liveAccount)

	require.Len(t, liveAccount.Orders, 3)

	require.Equal(t, placeOrderResp1.Id, liveAccount.Orders[0].Id)
	require.Equal(t, "filled", liveAccount.Orders[0].Status)

	require.Equal(t, placeOrderResp2.Id, liveAccount.Orders[1].Id)
	require.Equal(t, "rejected", liveAccount.Orders[1].Status)

	require.Equal(t, placeOrderResp3.Id, liveAccount.Orders[2].Id)
	require.Equal(t, "rejected", liveAccount.Orders[2].Status)

	// Re-fetch the reconcile account
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)

	require.Len(t, reconcileAccount.Orders, reconcileAccountOrdersInitialCount+2)

	require.Len(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Reconciles, 1)
	require.Equal(t, placeOrderResp1.Id, reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Reconciles[0].Id)
	require.Equal(t, "sell_short", reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Side)
	require.Equal(t, "filled", reconcileAccount.Orders[reconcileAccountOrdersInitialCount].Status)

	require.Len(t, reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Reconciles, 1)
	require.Equal(t, placeOrderResp2.Id, reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Reconciles[0].Id)
	require.Equal(t, "buy_to_cover", reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Side)
	require.Equal(t, "rejected", reconcileAccount.Orders[reconcileAccountOrdersInitialCount+1].Status)
}
