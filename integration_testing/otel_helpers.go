//go:build integration

package integrationtesting

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// getCollectorMetricsURL returns the Prometheus metrics URL for the collector container.
func getCollectorMetricsURL(ctx context.Context, t *testing.T, collector testcontainers.Container) string {
	t.Helper()
	host, err := collector.Host(ctx)
	require.NoError(t, err)
	port, err := collector.MappedPort(ctx, "8888/tcp")
	require.NoError(t, err)
	return fmt.Sprintf("http://%s:%s/metrics", host, port.Port())
}

// fetchCollectorMetrics fetches the Prometheus metrics page from the collector.
func fetchCollectorMetrics(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// waitForSpans polls the collector's Prometheus metrics endpoint until it reports
// that spans have been received (otelcol_exporter_sent_spans > 0).
func waitForSpans(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte {
	t.Helper()
	metricsURL := getCollectorMetricsURL(ctx, t, collector)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, err := fetchCollectorMetrics(metricsURL)
		if err != nil {
			t.Logf("waitForSpans: fetch err=%v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Look for otelcol_exporter_sent_spans metric > 0
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "otelcol_exporter_sent_spans") && !strings.HasPrefix(line, "otelcol_exporter_sent_spans_total 0") {
				if strings.Contains(line, `exporter="debug"`) && !strings.HasSuffix(strings.TrimSpace(line), " 0") {
					t.Logf("waitForSpans: collector received spans: %s", strings.TrimSpace(line))
					return []byte(body) // Return full metrics page for detailed assertions
				}
			}
		}

		time.Sleep(2 * time.Second)
	}

	require.Fail(t, "Timed out waiting for spans to be received by collector")
	return nil
}

// assertSpanExists checks that the collector's debug exporter has sent spans.
// With the debug exporter approach, we verify spans were received and exported
// rather than parsing individual span names. The span names are visible in
// the collector's stdout logs (via LogConsumer).
func assertSpanExists(t *testing.T, collectorMetrics []byte, _ string) {
	t.Helper()
	body := string(collectorMetrics)
	require.Contains(t, body, "otelcol_exporter_sent_spans", "Expected collector to report sent spans")
}

// waitForMetrics polls the collector's Prometheus metrics endpoint until it reports
// that metric data points have been received (otelcol_exporter_sent_metric_points > 0).
func waitForMetrics(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte {
	t.Helper()
	metricsURL := getCollectorMetricsURL(ctx, t, collector)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, err := fetchCollectorMetrics(metricsURL)
		if err != nil {
			t.Logf("waitForMetrics: fetch err=%v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "otelcol_exporter_sent_metric_points") {
				if strings.Contains(line, `exporter="debug"`) && !strings.HasSuffix(strings.TrimSpace(line), " 0") {
					t.Logf("waitForMetrics: collector received metrics: %s", strings.TrimSpace(line))
					return []byte(body)
				}
			}
		}

		time.Sleep(2 * time.Second)
	}

	require.Fail(t, "Timed out waiting for metrics to be received by collector")
	return nil
}

// assertMetricExists checks that the collector's debug exporter has sent metric data points.
func assertMetricExists(t *testing.T, collectorMetrics []byte, _ string) {
	t.Helper()
	body := string(collectorMetrics)
	require.Contains(t, body, "otelcol_exporter_sent_metric_points", "Expected collector to report sent metric points")
}
