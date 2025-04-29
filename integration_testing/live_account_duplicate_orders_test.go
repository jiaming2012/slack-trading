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

func TestLiveAccountDuplicateOrdersTest(t *testing.T) {
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
	clientReqId := "test"
	placeOrderResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createLivePgResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        2,
		Side:            "sell_short",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeOrderResp)

	// Fill the order
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
	require.NotNil(t, reconcileAccount)
	require.Len(t, reconcileAccount.Orders, 1)

	externalOpenOrderId := *reconcileAccount.Orders[0].ExternalId
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: externalOpenOrderId,
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

	// Check the live account order details
	liveAccount, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createLivePgResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)

	require.Equal(t, -2.0, liveAccount.Positions["AAPL"].Quantity)

	// Place 2 orders: 1 original + 1 duplicate
	errorCount := 0
	openOrderId := uint64(placeOrderResp.Id)
	for i := 0; i < 2; i++ {
		clientReqId = fmt.Sprintf("test-%d", i)
		placeOrderResp, err = p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
			PlaygroundId:    createLivePgResp.Id,
			ClientRequestId: &clientReqId,
			Symbol:          "AAPL",
			AssetClass:      "equity",
			Quantity:        2,
			Side:            "buy_to_cover",
			Type:            "market",
			RequestedPrice:  177.0,
			Duration:        "day",
			CloseOrderId:    &openOrderId,
		})

		if err != nil {
			fmt.Printf("Error placing order %d: %v\n", i, err)
			errorCount++
			continue
		}

		externalOrderId, err := getExternalOrderId(p, createLivePgResp.Id, placeOrderResp.Id)

		require.NoError(t, err, fmt.Sprintf("i == %d", i))

		// Fill the order
		_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
			OrderId: externalOrderId,
			Price:   178.0,
			Status:  "filled",
			Broker:  "tradier",
		})

		require.NoError(t, err, fmt.Sprintf("i == %d", i))

		waitUntilOrderStatus(p, placeOrderResp.Id, "filled")
	}

	require.Equal(t, 1, errorCount, "Duplicate order should have failed")

	filledOrders := 0
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

		if len(nextTickResponse.NewTrades) > 0 {
			filledOrders += len(nextTickResponse.NewTrades)
		}

		if filledOrders == 1 {
			break
		}

		time.Sleep(time.Second) // Wait for the order to be filled
	}

	require.Equal(t, 1, filledOrders)
}

func getExternalOrderId(p playground.PlaygroundService, livePlaygroundId string, orderId uint64) (uint64, error) {
	ctx := context.Background()

	liveAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: livePlaygroundId,
		FetchOrders:  false,
	})

	if err != nil {
		return 0, fmt.Errorf("failed to get live account: %w", err)
	}

	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *liveAccount.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get reconcile account: %w", err)
	}

	for _, order := range reconcileAccount.Orders {
		for _, reconcileOrder := range order.Reconciles {
			if reconcileOrder.Id == orderId {
				if order.ExternalId == nil {
					return 0, fmt.Errorf("order %d has no external ID", orderId)
				}

				return *order.ExternalId, nil
			}
		}
	}

	return 0, fmt.Errorf("order %d not found in reconcile account", orderId)
}
