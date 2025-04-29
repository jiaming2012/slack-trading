package integrationtesting

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccountCancelOrder(t *testing.T) {
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

	// Fetch reconcile playground initial order count
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

	reconcileAccountInitialOrderCount := len(reconcileAccount.Orders)
	initialReconcileAccountPosition := 0.0
	if pos, found := reconcileAccount.Positions["AAPL"]; found {
		initialReconcileAccountPosition = pos.Quantity
	}

	// Place an order
	clientReqId := "test1"
	placeOrderResponse1, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
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
	require.NotNil(t, placeOrderResponse1)

	// Fetch the account
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount)

	// Cancel the order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	require.Equal(t, reconcileAccountInitialOrderCount+1, len(reconcileAccount.Orders))

	lastIndex := len(reconcileAccount.Orders) - 1
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[lastIndex].ExternalId,
		Price:   178.0,
		Status:  "canceled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	fmt.Printf("Cancel order %d for playground %s\n", placeOrderResponse1.Id, createLivePgResp.Id)

	err = waitUntilOrderStatus(p, placeOrderResponse1.Id, "canceled")
	require.NoError(t, err)

	// Check the live account order details
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, liveAccount)
	require.Len(t, liveAccount.Orders, 1)

	fmt.Printf("Fetch live orders for playground %s\n", createLivePgResp.Id)

	require.Equal(t, "canceled", liveAccount.Orders[0].Status)

	// Check the live account positions
	pos := liveAccount.Positions["AAPL"]
	require.Nil(t, pos)

	// Check the reconcile account order details
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount)
	require.Equal(t, 1, len(reconcileAccount.Orders)-reconcileAccountInitialOrderCount)

	require.Equal(t, "canceled", reconcileAccount.Orders[reconcileAccountInitialOrderCount].Status)

	// Check the reconcile account positions
	pos = reconcileAccount.Positions["AAPL"]
	if pos != nil {
		require.Equal(t, initialReconcileAccountPosition, pos.Quantity)
	}
}
