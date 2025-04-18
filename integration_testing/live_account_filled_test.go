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

func TestLiveAccountFilled(t *testing.T) {
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

	// Fetch the account
	// This should return an empty list of orders since we haven't placed any orders yet
	liveAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)
	require.Len(t, liveAccount.Orders, 0)

	// Place an order
	clientReqId := "test"
	placeOrderResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
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
	require.NotNil(t, placeOrderResp)

	// Fetch the account
	// This should now return a list of orders with the order we just placed
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)
	require.Len(t, liveAccount.Orders, 1)

	// Check the order details
	require.NotNil(t, liveAccount.Orders[0].ClientRequestId)
	require.Equal(t, clientReqId, *liveAccount.Orders[0].ClientRequestId)
	require.Equal(t, "AAPL", liveAccount.Orders[0].Symbol)
	require.Equal(t, "equity", liveAccount.Orders[0].Class)
	require.Equal(t, "buy", liveAccount.Orders[0].Side)
	require.Equal(t, 10.0, liveAccount.Orders[0].Quantity)
	require.Equal(t, "market", liveAccount.Orders[0].Type)
	require.Equal(t, 177.0, liveAccount.Orders[0].RequestedPrice)
	require.Equal(t, "day", liveAccount.Orders[0].Duration)
	require.Equal(t, "pending", liveAccount.Orders[0].Status)
	require.NotNil(t, liveAccount.Meta.ReconcilePlaygroundId)

	// Check the live account positions
	pos := liveAccount.Positions["AAPL"]
	require.Nil(t, pos)

	// Fetch the reconcile account
	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)
	require.Len(t, reconcileAccount.Orders, 1)

	// Check the reconcile order details
	require.NotEqual(t, liveAccount.Orders[0].Id, reconcileAccount.Orders[0].Id)
	require.Equal(t, "AAPL", reconcileAccount.Orders[0].Symbol)
	require.Equal(t, "equity", reconcileAccount.Orders[0].Class)
	require.Equal(t, "buy", reconcileAccount.Orders[0].Side)
	require.Equal(t, "market", reconcileAccount.Orders[0].Type)
	require.Equal(t, "day", reconcileAccount.Orders[0].Duration)
	require.Equal(t, "pending", reconcileAccount.Orders[0].Status)

	// Check the reconcile account positions
	pos = reconcileAccount.Positions["AAPL"]
	require.Nil(t, pos)

	require.Len(t, reconcileAccount.Orders[0].Reconciles, 1)
	require.Equal(t, liveAccount.Orders[0].Id, reconcileAccount.Orders[0].Reconciles[0].Id)
	require.NotNil(t, reconcileAccount.Orders[0].ExternalId)

	// Fill the order
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[0].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	now := time.Now()
	bNewTrade := false
	for {
		if time.Since(now) > time.Second*40 {
			break
		}

		uId := uuid.NewString()
		fmt.Printf("%s NextTick: %s\n", createLivePgResp.Id, uId)
		nextTickResponse, err := p.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: createLivePgResp.Id,
			RequestId:    uId,
		})
		require.NoError(t, err)

		if len(nextTickResponse.NewTrades) > 0 {
			bNewTrade = true
			break
		}

		time.Sleep(time.Second) // Wait for the order to be filled
	}

	require.True(t, bNewTrade)

	// Check the live account order details
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, liveAccount)
	require.Len(t, liveAccount.Orders, 1)

	require.Equal(t, "filled", liveAccount.Orders[0].Status)

	// Check the live account positions
	pos = liveAccount.Positions["AAPL"]
	require.NotNil(t, pos)
	require.Equal(t, 10.0, pos.Quantity)

	// Check the reconcile account order details
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)
	require.Len(t, reconcileAccount.Orders, 1)

	require.Equal(t, "filled", reconcileAccount.Orders[0].Status)

	// Check the live account positions
	pos = reconcileAccount.Positions["AAPL"]
	require.NotNil(t, pos)
	require.Equal(t, 10.0, pos.Quantity)
}
