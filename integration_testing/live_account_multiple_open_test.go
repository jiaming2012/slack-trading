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

func TestLiveAccountMultipleOpen(t *testing.T) {
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

	lastIndex := reconcileAccountOrdersInitialCount
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[lastIndex].ExternalId,
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

	// Fill new buy order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	// Must fill 2nd order first
	require.NoError(t, err)
	require.Equal(t, 2, len(reconcileAccount.Orders)-reconcileAccountOrdersInitialCount)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[lastIndex+1].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	invalidOrdersCount := 0
	openTradeCount := 0
	bPlacedFillOrder := false
	for {
		uId := uuid.NewString()

		nextTickResponse, err := p.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: createLivePgResp.Id,
			RequestId:    uId,
		})
		require.NoError(t, err)

		invalidOrdersCount += len(nextTickResponse.InvalidOrders)

		if len(nextTickResponse.NewTrades) > 0 {
			openTradeCount += len(nextTickResponse.NewTrades)
		}

		if openTradeCount == 1 && !bPlacedFillOrder {
			fmt.Printf("waiting for order %d to be filled ...\n", placeOrderResp3.Id)

			err = waitUntilOrderStatus(p, placeOrderResp3.Id, "pending")
			require.NoError(t, err)

			reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
				PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
				FetchOrders:  true,
			})

			// 3rd order status, pre_pending -> pending
			require.NoError(t, err)
			require.Equal(t, 3, len(reconcileAccount.Orders)-reconcileAccountOrdersInitialCount)

			_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
				OrderId: *reconcileAccount.Orders[lastIndex+2].ExternalId,
				Price:   178.0,
				Status:  "filled",
				Broker:  "tradier",
			})

			require.NoError(t, err)

			bPlacedFillOrder = true
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

	require.Equal(t, placeOrderResp2.Id, liveAccount.Orders[1].Id)
	require.Equal(t, "filled", liveAccount.Orders[1].Status)

	require.Equal(t, placeOrderResp3.Id, liveAccount.Orders[2].Id)
	require.Equal(t, "filled", liveAccount.Orders[2].Status)

	// Re-fetch the reconcile account
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)

	require.Equal(t, 3, len(reconcileAccount.Orders)-reconcileAccountOrdersInitialCount)

	require.Len(t, reconcileAccount.Orders[lastIndex].Reconciles, 1)
	require.Equal(t, placeOrderResp1.Id, reconcileAccount.Orders[lastIndex].Reconciles[0].Id)
	require.Equal(t, "sell_short", reconcileAccount.Orders[lastIndex].Side)
	require.Equal(t, "filled", liveAccount.Orders[lastIndex].Status)

	require.Len(t, reconcileAccount.Orders[lastIndex+1].Reconciles, 1)
	require.Equal(t, placeOrderResp2.Id, reconcileAccount.Orders[lastIndex+1].Reconciles[0].Id)
	require.Equal(t, "buy_to_cover", reconcileAccount.Orders[lastIndex+1].Side)
	require.Equal(t, "filled", liveAccount.Orders[lastIndex+1].Status)

	require.Len(t, reconcileAccount.Orders[lastIndex+2].Reconciles, 1)
	require.Equal(t, placeOrderResp3.Id, reconcileAccount.Orders[lastIndex+2].Reconciles[0].Id)
	require.Equal(t, "buy", reconcileAccount.Orders[lastIndex+2].Side)
	require.Equal(t, "filled", liveAccount.Orders[lastIndex+2].Status)
}
