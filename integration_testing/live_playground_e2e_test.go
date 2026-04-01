//go:build integration

package integrationtesting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/playground"
)

func TestLivePlaygroundEquityTradeAndDashboard(t *testing.T) {
	ctx := context.Background()

	p, collector := setupWithOtel(t, ctx, "test")

	clientId := "e2e-equity-test-" + uuid.NewString()[:8]

	// --- Step 1: Create live playground with mock account ---
	createResp, err := p.CreateLivePlayground(ctx, &playground.CreateLivePlaygroundRequest{
		ClientId:    &clientId,
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
	t.Logf("Created playground: %s (client_id: %s)", createResp.Id, clientId)

	// --- Step 2: Verify empty account ---
	account, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, account.Orders, 0)
	require.NotNil(t, account.Meta.ReconcilePlaygroundId)

	// --- Step 3: Place equity buy order ---
	reqId := "e2e-" + uuid.NewString()[:8]
	placeResp, err := p.PlaceOrder(ctx, &playground.PlaceOrderRequest{
		PlaygroundId:    createResp.Id,
		ClientRequestId: &reqId,
		Symbol:          "AAPL",
		AssetClass:      "equity",
		Quantity:        5,
		Side:            "buy",
		Type:            "market",
		RequestedPrice:  250.0,
		Duration:        "day",
	})

	require.NoError(t, err)
	require.NotNil(t, placeResp)
	t.Logf("Order placed: request_id=%s", reqId)

	// --- Step 4: Get reconcile account to find external order ID ---
	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *account.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, 1)
	require.NotNil(t, reconcileAccount.Orders[0].ExternalId)
	t.Logf("Reconcile order external_id: %d", *reconcileAccount.Orders[0].ExternalId)

	// --- Step 5: Mock fill the order ---
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[0].ExternalId,
		Price:   251.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)
	t.Log("Mock fill sent")

	// --- Step 6: Poll NextTick until trade appears ---
	var foundTrade bool
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		tickResp, err := p.NextTick(ctx, &playground.NextTickRequest{
			PlaygroundId: createResp.Id,
			RequestId:    uuid.NewString(),
		})
		require.NoError(t, err)

		if len(tickResp.NewTrades) > 0 {
			foundTrade = true
			t.Logf("Trade received: %d new trade(s)", len(tickResp.NewTrades))
			break
		}

		time.Sleep(time.Second)
	}

	require.True(t, foundTrade, "Expected trade to appear within 40 seconds")

	// --- Step 7: Verify order is filled and position exists ---
	account, err = p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, account.Orders, 1)
	require.Equal(t, "filled", account.Orders[0].Status)
	require.Equal(t, "AAPL", account.Orders[0].Symbol)
	require.Equal(t, "buy", account.Orders[0].Side)
	require.Equal(t, 5.0, account.Orders[0].Quantity)

	pos := account.Positions["AAPL"]
	require.NotNil(t, pos, "Expected AAPL position after fill")
	require.Equal(t, 5.0, pos.Quantity)
	t.Logf("Position verified: AAPL qty=%.0f", pos.Quantity)

	// --- Step 8: Verify OTel metrics were exported to the collector ---
	// The app's OTel runtime instrumentation exports metrics (e.g., runtime.uptime).
	// This verifies the full metric pipeline: app -> OTLP -> collector -> file export.
	metricData := waitForMetrics(t, ctx, collector, 30*time.Second)
	require.NotEmpty(t, metricData, "Expected metrics to be exported to collector")
	t.Log("Metrics verified: OTel collector received metric data from the app container")

	// Also verify trace spans were captured for the Twirp RPC calls
	spanData := waitForSpans(t, ctx, collector, 30*time.Second)
	assertSpanExists(t, spanData, "PlaceOrder")
	t.Log("Spans verified: OTel collector captured PlaceOrder trace from the app container")

	t.Log("E2E test passed: playground created, order filled, position verified, OTel telemetry confirmed")
}
