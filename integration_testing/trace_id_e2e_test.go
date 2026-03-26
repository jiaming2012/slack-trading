//go:build integration

package integrationtesting

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/playground"
)

func TestTraceId_E2E_SuccessfulTrade(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)
	p := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	// Create a live playground
	createResp, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
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
	require.NotEmpty(t, createResp.Id)

	// Call NextTick with trace_id
	tickReqId := uuid.NewString()
	nextTickResp, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    tickReqId,
		TraceId:      "e2e-success-trace-001",
	})
	require.NoError(t, err)
	require.NotNil(t, nextTickResp)

	// Place an order with trace_id
	clientReqId := "e2e-trace-order-001"
	placeOrderResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        10,
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
		TraceId:         "e2e-success-trace-001",
	})
	require.NoError(t, err)
	require.NotNil(t, placeOrderResp)
	require.Equal(t, "AAPL", placeOrderResp.Symbol)
	require.Equal(t, "buy", placeOrderResp.Side)

	// Verify the order was created
	account, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.Len(t, account.Orders, 1)
}

func TestTraceId_E2E_ErrorCase(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)
	p := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	// Call PlaceOrder with an invalid playground_id and trace_id
	_, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   "00000000-0000-0000-0000-000000000000",
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "buy",
		Type:           "market",
		RequestedPrice: 177.0,
		Duration:       "day",
		TraceId:        "e2e-error-trace-001",
	})

	// Expect an error because the playground doesn't exist
	require.Error(t, err)
}

func TestTraceId_E2E_TraceIdCorrelatesTickAndOrder(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	projectsDir, networkName := setupDatabases(t, ctx, goEnv)
	p := createPlaygroundServerAndClient(ctx, t, projectsDir, networkName)

	// Create a live playground
	createResp, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
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

	// Call NextTick with trace_id "correlated-001"
	tickResp1, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
		TraceId:      "correlated-001",
	})
	require.NoError(t, err)
	require.NotNil(t, tickResp1)

	// Place an order with trace_id "correlated-001"
	clientReqId := "correlated-order-001"
	orderResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createResp.Id,
		ClientRequestId: &clientReqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        5,
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  177.0,
		Duration:        "day",
		TraceId:         "correlated-001",
	})
	require.NoError(t, err)
	require.NotNil(t, orderResp)

	// Call NextTick with a different trace_id "correlated-002"
	tickResp2, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
		TraceId:      "correlated-002",
	})
	require.NoError(t, err)
	require.NotNil(t, tickResp2)

	// Verify both calls succeeded -- trace_id propagation is verified
	// by the server-side span attributes (tested in unit tests).
	// This test confirms trace_id flows through real Twirp RPC calls
	// without breaking any functionality.
	account, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.Len(t, account.Orders, 1)
}
