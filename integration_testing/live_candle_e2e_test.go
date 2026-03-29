package integrationtesting

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/playground"
)

func TestLiveCandleProcessedAndDashboard(t *testing.T) {
	if os.Getenv("RUN_E2E_PRODUCTION") == "" {
		t.Skip("Skipping production E2E test. Set RUN_E2E_PRODUCTION=1 to run.")
	}

	ctx := context.Background()
	p := getPlaygroundClient()

	clientId := "e2e-candle-test-" + uuid.NewString()[:8]

	// --- Baseline metric ---
	baselineStr := queryPrometheusMetric(t, "sum(grodt_candles_processed_total)")
	t.Logf("Baseline grodt_candles_processed_total: %s", baselineStr)

	// --- Create live mock playground with 1-minute candle repo and 1 day history ---
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
				HistoryInDays:      1,
			},
		},
		Environment: "live",
	})

	require.NoError(t, err)
	require.NotEmpty(t, createResp.Id)
	t.Logf("Created playground: %s (client_id: %s)", createResp.Id, clientId)

	// --- Drain any existing candles from the queue ---
	tickResp, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
	})
	require.NoError(t, err)
	t.Logf("Initial tick: drained %d existing candles", len(tickResp.NewCandles))

	// --- Fetch candles from repo to find the last timestamp ---
	// Fetch from 7 days ago to cover history_in_days=1
	fromTime := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
	candlesResp, err := p.GetCandlesFromRepo(ctx, &playground.GetCandlesRequest{
		PlaygroundId:    createResp.Id,
		Symbol:          "AAPL",
		PeriodInSeconds: 60,
		FromRTF3339:     fromTime,
	})
	require.NoError(t, err)
	require.NotEmpty(t, candlesResp.Bars, "Expected candles from history_in_days=1")

	lastBar := candlesResp.Bars[len(candlesResp.Bars)-1]
	lastTs, err := time.Parse(time.RFC3339, lastBar.Datetime)
	if err != nil {
		// Try alternate format
		lastTs, err = time.Parse("2006-01-02 15:04:05-07:00", lastBar.Datetime)
		require.NoError(t, err)
	}
	t.Logf("Last candle: %v close=%.2f", lastTs, lastBar.Close)

	// --- Inject a mock candle via MockAddCandle ---
	newCandleTs := lastTs.Add(1 * time.Minute)
	_, err = p.MockAddCandle(ctx, &playground.MockAddCandleRequest{
		PlaygroundId:    createResp.Id,
		Symbol:          "AAPL",
		PeriodInSeconds: 60,
		Open:            250.0,
		High:            251.5,
		Low:             249.5,
		Close:           251.0,
		Volume:          10000,
		Timestamp:       newCandleTs.Format(time.RFC3339),
	})

	require.NoError(t, err)
	t.Logf("MockAddCandle sent: AAPL 1m at %v", newCandleTs)

	// --- Tick to pick up the new candle from the queue ---
	tickResp, err = p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, tickResp.NewCandles, "Expected new candle in tick response after MockAddCandle")
	t.Logf("NextTick returned %d new candle(s)", len(tickResp.NewCandles))

	// Verify the candle data
	foundCandle := false
	for _, c := range tickResp.NewCandles {
		if c.Symbol == "AAPL" {
			require.Equal(t, 251.0, c.Bar.Close)
			require.Equal(t, 251.5, c.Bar.High)
			foundCandle = true
		}
	}
	require.True(t, foundCandle, "Expected AAPL candle in tick response")
	t.Log("Candle data verified in tick response")

	// --- Wait for metric scrape and verify dashboard ---
	t.Log("Waiting 45s for metric export + scrape...")
	time.Sleep(45 * time.Second)

	afterStr := queryPrometheusMetric(t, "sum(grodt_candles_processed_total)")
	t.Logf("After grodt_candles_processed_total: %s", afterStr)

	require.NotEmpty(t, afterStr, "Expected grodt_candles_processed_total to have data")

	afterVal, _ := strconv.ParseFloat(afterStr, 64)
	require.GreaterOrEqual(t, afterVal, 1.0, "Expected at least 1 candle processed")
	t.Logf("Candles metric verified: %s (baseline was %s)", afterStr, baselineStr)

	t.Log("E2E test passed: candle injected via MockAddCandle, received in tick, dashboard metric confirmed")
}
