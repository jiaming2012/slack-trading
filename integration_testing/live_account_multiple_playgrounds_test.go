package integrationtesting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccountMultiplePlaygrounds(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)

	// Start main app container
	p := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	fmt.Printf("Playground client: %v\n", p)

	createLivePgResp1, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
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

	createLivePgResp2, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
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

	// Place a short order in the first playground
	clientReqId := "test1"
	placeOrderResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp1.Id,
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
	require.NotNil(t, placeOrderResp)

	// Fill the order
	liveAccount1, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp1.Id,
		FetchOrders:  false,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount1)
	require.NotNil(t, liveAccount1.Meta.ReconcilePlaygroundId)

	reconcileAccount1, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount1.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount1.Orders[0].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	time.Sleep(time.Second * 10)

	// Place a long order in the second playground
	clientReqId = "test2"
	placeOrderResp, err = p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp2.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        2,
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp)

	// Fill the order
	liveAccount2, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp2.Id,
		FetchOrders:  false,
	})

	require.NoError(t, err)
	require.NotNil(t, liveAccount2)
	require.NotNil(t, liveAccount2.Meta.ReconcilePlaygroundId)

	reconcileAccount2, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount2.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount2)
	require.Greater(t, len(reconcileAccount2.Meta.PlaygroundId), 0)
	require.Equal(t, reconcileAccount2.Meta.PlaygroundId, reconcileAccount1.Meta.PlaygroundId)
	require.Len(t, reconcileAccount2.Orders, 2)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount2.Orders[1].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	time.Sleep(time.Second * 20)

	// Check the live account #1 order details
	liveAccount1, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp1.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	require.Len(t, liveAccount1.Orders, 1)
	require.Equal(t, "filled", liveAccount1.Orders[0].Status)

	// Check the live account positions
	pos := liveAccount1.Positions["AAPL"]
	require.Equal(t, -10.0, pos.Quantity)

	// Check the live account #2 order details
	liveAccount2, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp2.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)

	require.Len(t, liveAccount2.Orders, 1)
	require.Equal(t, "filled", liveAccount2.Orders[0].Status)

	// Check the live account positions
	pos = liveAccount2.Positions["AAPL"]
	require.Equal(t, 2.0, pos.Quantity)

	// Fetch the reconcile account
	reconcileAccount2, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount2.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, reconcileAccount2)
	require.Len(t, reconcileAccount2.Orders, 2)

	// Check the reconcile order details
	require.Equal(t, "filled", reconcileAccount2.Orders[0].Status)
	require.Equal(t, "filled", reconcileAccount2.Orders[1].Status)

	// Check the reconcile account positions
	pos = reconcileAccount2.Positions["AAPL"]
	require.NotNil(t, pos)
	require.Equal(t, -8.0, pos.Quantity)
}
