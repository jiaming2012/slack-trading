//go:build integration

package integrationtesting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/playground"
)

func TestTraceId_E2E_SuccessfulTrade(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	p, collector := setupWithOtel(t, ctx, goEnv)

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

	// Call NextTick
	tickReqId := uuid.NewString()
	nextTickResp, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    tickReqId,
	})
	require.NoError(t, err)
	require.NotNil(t, nextTickResp)

	// Place an order
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

	// Verify spans arrived at the OTel collector
	spanData := waitForSpans(t, ctx, collector, 30*time.Second)
	assertSpanExists(t, spanData, "PlaceOrder")
	assertSpanExists(t, spanData, "NextTick")
}

func TestTraceId_E2E_ErrorCase(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	p, collector := setupWithOtel(t, ctx, goEnv)

	// Call PlaceOrder with an invalid playground_id
	_, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:   "00000000-0000-0000-0000-000000000000",
		Symbol:         "AAPL",
		AssetClass:     "equity",
		Quantity:       10,
		Side:           "buy",
		Type:           "market",
		RequestedPrice: 177.0,
		Duration:       "day",
	})

	// Expect an error because the playground doesn't exist
	require.Error(t, err)

	// Even error cases should produce spans
	spanData := waitForSpans(t, ctx, collector, 30*time.Second)
	assertSpanExists(t, spanData, "PlaceOrder")
}

func TestTraceId_E2E_TraceIdCorrelatesTickAndOrder(t *testing.T) {
	ctx := context.Background()
	goEnv := "test"

	p, collector := setupWithOtel(t, ctx, goEnv)

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

	// Call NextTick
	tickResp1, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotNil(t, tickResp1)

	// Place an order
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
	})
	require.NoError(t, err)
	require.NotNil(t, orderResp)

	// Call NextTick again
	tickResp2, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotNil(t, tickResp2)

	// Verify calls succeeded
	account, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})
	require.NoError(t, err)
	require.Len(t, account.Orders, 1)

	// Verify spans exist in the collector
	spanData := waitForSpans(t, ctx, collector, 30*time.Second)
	assertSpanExists(t, spanData, "PlaceOrder")
	assertSpanExists(t, spanData, "NextTick")
}
