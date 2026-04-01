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

// TestLiveCandleMetricPipeline verifies that the OTel metric pipeline works
// end-to-end using TestContainers. Creates a playground, calls NextTick to
// generate activity, and asserts metrics arrived at the collector.
//
// Note: MockAddCandle RPC is not yet available in this branch. When it is,
// this test should be extended to inject a candle and assert
// grodt.candles.processed metric specifically.
func TestLiveCandleMetricPipeline(t *testing.T) {
	ctx := context.Background()

	p, collector := setupWithOtel(t, ctx, "test")

	clientId := "e2e-candle-test-" + uuid.NewString()[:8]

	// --- Create live mock playground ---
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

	// --- Call NextTick to generate activity ---
	tickResp, err := p.NextTick(ctx, &playground.NextTickRequest{
		PlaygroundId: createResp.Id,
		RequestId:    uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotNil(t, tickResp)
	t.Log("NextTick called successfully")

	// --- Verify OTel metrics were exported to the collector ---
	// The app's runtime instrumentation (go.opentelemetry.io/contrib/instrumentation/runtime)
	// exports metrics like process.runtime.go.mem.heap_alloc. This verifies the
	// full pipeline: app -> OTLP HTTP -> collector -> file exporter.
	metricData := waitForMetrics(t, ctx, collector, 30*time.Second)
	require.NotEmpty(t, metricData, "Expected metrics to be exported to collector")
	t.Log("Metrics verified: OTel collector received metric data from the app container")

	// --- Verify trace spans ---
	spanData := waitForSpans(t, ctx, collector, 30*time.Second)
	assertSpanExists(t, spanData, "NextTick")
	t.Log("Spans verified: OTel collector captured NextTick trace from the app container")

	t.Log("E2E test passed: metric pipeline verified end-to-end with self-contained OTel collector")
}
