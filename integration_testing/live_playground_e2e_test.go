package integrationtesting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/playground"
)

// prometheusQueryResult represents the Prometheus instant query response.
type prometheusQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// getPlaygroundClient returns a Twirp client pointed at the configured host.
// Set TWIRP_HOST env var to override (default: http://159.89.226.131:5051).
func getPlaygroundClient() playground.PlaygroundService {
	host := os.Getenv("TWIRP_HOST")
	if host == "" {
		host = "http://159.89.226.131:5051"
	}

	client := http.Client{Timeout: 30 * time.Second}
	return playground.NewPlaygroundServiceProtobufClient(host, &client)
}

// queryPrometheusMetric queries a Prometheus metric via the Grafana datasource proxy.
// Returns the first result value as a string, or empty string if no data.
func queryPrometheusMetric(t *testing.T, query string) string {
	grafanaHost := os.Getenv("GRAFANA_HOST")
	if grafanaHost == "" {
		grafanaHost = "http://159.89.226.131:3000"
	}

	grafanaUser := os.Getenv("GRAFANA_USER")
	if grafanaUser == "" {
		grafanaUser = "admin"
	}

	grafanaPass := os.Getenv("GRAFANA_PASSWORD")
	if grafanaPass == "" {
		grafanaPass = "grodt2026"
	}

	url := fmt.Sprintf("%s/api/datasources/proxy/uid/prometheus/api/v1/query?query=%s", grafanaHost, query)

	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	req.SetBasicAuth(grafanaUser, grafanaPass)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Prometheus query via Grafana failed")

	var result prometheusQueryResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)

	if len(result.Data.Result) == 0 {
		return ""
	}

	// Value is [timestamp, "string_value"]
	return fmt.Sprintf("%v", result.Data.Result[0].Value[1])
}

func TestLivePlaygroundEquityTradeAndDashboard(t *testing.T) {
	if os.Getenv("RUN_E2E_PRODUCTION") == "" {
		t.Skip("Skipping production E2E test. Set RUN_E2E_PRODUCTION=1 to run.")
	}

	ctx := context.Background()
	p := getPlaygroundClient()

	clientId := "e2e-equity-test-" + uuid.NewString()[:8]

	// --- Step 1: Query baseline metric ---
	baselineStr := queryPrometheusMetric(t, "sum(grodt_orders_placed_total)")
	t.Logf("Baseline grodt_orders_placed_total: %s", baselineStr)

	// --- Step 2: Create live playground with mock account ---
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

	// --- Step 3: Verify empty account ---
	account, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: createResp.Id,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, account.Orders, 0)
	require.NotNil(t, account.Meta.ReconcilePlaygroundId)

	// --- Step 4: Place equity buy order ---
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

	// --- Step 5: Get reconcile account to find external order ID ---
	reconcileAccount, err := p.GetAccount(ctx, &playground.GetAccountRequest{
		PlaygroundId: *account.Meta.ReconcilePlaygroundId,
		FetchOrders:  true,
	})

	require.NoError(t, err)
	require.Len(t, reconcileAccount.Orders, 1)
	require.NotNil(t, reconcileAccount.Orders[0].ExternalId)
	t.Logf("Reconcile order external_id: %d", *reconcileAccount.Orders[0].ExternalId)

	// --- Step 6: Mock fill the order ---
	_, err = p.MockFillOrder(ctx, &playground.MockFillOrderRequest{
		OrderId: *reconcileAccount.Orders[0].ExternalId,
		Price:   251.0,
		Status:  "filled",
		Broker:  "tradier",
	})

	require.NoError(t, err)
	t.Log("Mock fill sent")

	// --- Step 7: Poll NextTick until trade appears ---
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

	// --- Step 8: Verify order is filled and position exists ---
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

	// --- Step 9: Verify Prometheus metric incremented ---
	// Wait for metric scrape interval (OTel exports every ~30s, Prometheus scrapes ~15s)
	t.Log("Waiting 45s for metric export + scrape...")
	time.Sleep(45 * time.Second)

	afterStr := queryPrometheusMetric(t, "sum(grodt_orders_placed_total)")
	t.Logf("After grodt_orders_placed_total: %s", afterStr)

	require.NotEmpty(t, afterStr, "Expected grodt_orders_placed_total to have data after placing order")

	// If baseline was empty, any value means it incremented
	if baselineStr == "" {
		t.Log("Metric appeared (was empty before) — dashboard metric verified")
	} else {
		// Both should be parseable as floats; after > baseline
		require.NotEqual(t, baselineStr, afterStr,
			"Expected grodt_orders_placed_total to increment after placing order")
		t.Logf("Metric incremented: %s -> %s — dashboard metric verified", baselineStr, afterStr)
	}

	t.Log("E2E test passed: playground created, order filled, position verified, dashboard metric confirmed")
}
