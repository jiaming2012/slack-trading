package integrationtesting

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccount(t *testing.T) {
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

	// Fetch the reconcile account
	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders: true,
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

	require.Len(t, reconcileAccount.Orders[0].Reconciles, 1)
	require.Equal(t, liveAccount.Orders[0].Id, reconcileAccount.Orders[0].Reconciles[0].Id)


	// // Fill the order
	// _, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
	// 	OrderId: liveAccount.Orders[0].Id,
	// 	Price: 178.0,
	// 	Status: "filled",
	// 	Broker: "tradier",
	// })

	// require.NoError(t, err)

	// // Check the order details
	// liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
	// 	PlaygroundId: createLivePgResp.Id,
	// 	FetchOrders:  true,
	// })
	// require.NoError(t, err)
	// require.NotNil(t, liveAccount)
	// require.Len(t, liveAccount.Orders, 1)
	
	// require.Equal(t, "filled", liveAccount.Orders[0].Status)
}
