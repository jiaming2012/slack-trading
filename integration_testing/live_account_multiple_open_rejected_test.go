package integrationtesting

import (
	"context"
	"fmt"
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

	fmt.Printf("Playground client: %v\n", p)

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

	// Place an order
	clientReqId := "test1"
	placeOrderResp1, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "sell_short",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp1)

	// Fetch the live account
	liveAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)

	// Fill the order
	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[0].ExternalId,
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

	// close order should succeed
	clientReqId = "test2"
	placeOrderResp2, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "buy_to_cover",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp2)

	// new buy order should succeed
	clientReqId = "test3"
	placeOrderResp3, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp3)

	// Reject new buy order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	// Must fill 2nd order first
	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, 2)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[1].ExternalId,
		Price:   178.0,
		Status:  "filled",
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

		if openTradeCount == 2 {
			break
		}

		time.Sleep(time.Second) // Wait for the order to be filled
	}

	require.NotNil(t, openTradeId)
	require.Equal(t, 0, invalidOrdersCount)

	// Re-fetch the live account
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)

	require.Len(t, liveAccount.Orders, 3)

	require.Equal(t, placeOrderResp1.Id, liveAccount.Orders[0].Id)
	require.Equal(t, "filled", liveAccount.Orders[0].Status)

	require.Equal(t, placeOrderResp2.Id, liveAccount.Orders[2].Id)
	require.Equal(t, "filled", liveAccount.Orders[2].Status)

	require.Equal(t, placeOrderResp3.Id, liveAccount.Orders[1].Id)
	require.Equal(t, "filled", liveAccount.Orders[1].Status)

	// Re-fetch the reconcile account
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)

	require.Len(t, reconcileAccount.Orders, 3)

	require.Len(t, reconcileAccount.Orders[0].Reconciles, 1)
	require.Equal(t, placeOrderResp1.Id, reconcileAccount.Orders[0].Reconciles[0].Id)
	require.Equal(t, "sell_short", reconcileAccount.Orders[0].Side)
	require.Equal(t, "filled", liveAccount.Orders[0].Status)

	require.Len(t, reconcileAccount.Orders[1].Reconciles, 1)
	require.Equal(t, placeOrderResp3.Id, reconcileAccount.Orders[1].Reconciles[0].Id)
	require.Equal(t, "buy_to_cover", reconcileAccount.Orders[1].Side)
	require.Equal(t, "filled", liveAccount.Orders[2].Status)

	require.Len(t, reconcileAccount.Orders[2].Reconciles, 1)
	require.Equal(t, placeOrderResp2.Id, reconcileAccount.Orders[2].Reconciles[0].Id)
	require.Equal(t, "buy", reconcileAccount.Orders[2].Side)
	require.Equal(t, "filled", liveAccount.Orders[2].Status)
}
