package integrationtesting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLiveAccountClose(t *testing.T) {
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

	time.Sleep(time.Second * 10)

	// Place sell 1/2 order
	clientReqId = "test2"
	_, err = p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        5,
		Side:            "sell",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)

	// Fill the order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, 2)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[1].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	time.Sleep(time.Second * 10)

	// Place sell 1/2 order, closes remaining position
	clientReqId = "test3"
	_, err = p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        5,
		Side:            "sell",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)

	// Fill the order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, 3)

	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[2].ExternalId,
		Price:   178.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)

	// now := time.Now()
	// bNewTrade := false
	// for {
	// 	if time.Since(now) > time.Second*40 {
	// 		break
	// 	}

	// 	uId := uuid.NewString()
	// 	fmt.Printf("%s NextTick: %s\n", createLivePgResp.Id, uId)
	// 	nextTickResponse, err := p.NextTick(ctx, &playground.NextTickRequest{
	// 		PlaygroundId: createLivePgResp.Id,
	// 		RequestId:    uId,
	// 	})
	// 	require.NoError(t, err)

	// 	if len(nextTickResponse.NewTrades) > 0 {
	// 		bNewTrade = true
	// 		break
	// 	}

	// 	time.Sleep(time.Second) // Wait for the order to be filled
	// }

	// require.True(t, bNewTrade)

	time.Sleep(time.Second * 20)

	// Check the live account order details
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, liveAccount)
	require.Len(t, liveAccount.Orders, 3)

	require.Equal(t, "filled", liveAccount.Orders[0].Status)
	require.Equal(t, "filled", liveAccount.Orders[1].Status)
	require.Equal(t, "filled", liveAccount.Orders[2].Status)

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
	require.Len(t, reconcileAccount.Orders, 3)

	require.Equal(t, "filled", reconcileAccount.Orders[0].Status)
	require.Equal(t, "filled", reconcileAccount.Orders[1].Status)
	require.Equal(t, "filled", reconcileAccount.Orders[2].Status)

	// Check the live account positions
	pos = reconcileAccount.Positions["AAPL"]
	require.Nil(t, pos)
}
