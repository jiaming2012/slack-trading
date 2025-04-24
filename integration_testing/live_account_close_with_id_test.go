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

func TestLiveAccountCloseWithID(t *testing.T) {
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
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp1)

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

	// Close the trade
	clientReqId = "test2"
	placeOrderResp2, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "sell",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
		CloseOrderId:    &placeOrderResp1.Id,
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp2)

	// Fill first close order
	reconcileAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})
	require.NoError(t, err)

	require.Len(t, reconcileAccount.Orders, 2)

	delayInSeconds := int32(5)
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId:        *reconcileAccount.Orders[1].ExternalId,
		Price:          178.0,
		Status:         "filled",
		Broker:         "tradier",
		DelayInSeconds: &delayInSeconds,
	})

	require.NoError(t, err)

	// 2nd order should fail
	clientReqId = "test3"
	placeOrderResp3, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "sell",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
		CloseOrderId:    &placeOrderResp1.Id,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot sell when no position exists")
	require.Nil(t, placeOrderResp3)
}
