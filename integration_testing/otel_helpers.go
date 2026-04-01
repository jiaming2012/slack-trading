//go:build integration

package integrationtesting

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// waitForSpans polls the collector container's trace export file until non-empty or timeout.
// Returns raw JSON bytes (newline-delimited JSON, each line is a ResourceSpans batch).
func waitForSpans(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		reader, err := collector.CopyFileFromContainer(ctx, "/tmp/otel-traces.json")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reader)
		reader.Close()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		data := buf.Bytes()
		if len(bytes.TrimSpace(data)) > 0 {
			return data
		}

		time.Sleep(2 * time.Second)
	}

	require.Fail(t, "Timed out waiting for spans to appear in collector export file")
	return nil
}

// assertSpanExists parses the OTLP JSON export (newline-delimited JSON) and asserts
// at least one span name contains the given substring.
func assertSpanExists(t *testing.T, spanData []byte, spanNameSubstring string) {
	t.Helper()

	scanner := bufio.NewScanner(bytes.NewReader(spanData))
	// Increase scanner buffer for potentially large JSON lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var batch map[string]interface{}
		if err := json.Unmarshal([]byte(line), &batch); err != nil {
			continue
		}

		if containsSpanName(batch, spanNameSubstring) {
			return
		}
	}

	require.Failf(t, "Span not found", "Expected span with name containing %q in collector export", spanNameSubstring)
}

// containsSpanName recursively searches a JSON structure for a span name containing the substring.
func containsSpanName(data interface{}, substring string) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this is a span with a matching name
		if name, ok := v["name"]; ok {
			if nameStr, ok := name.(string); ok {
				if strings.Contains(nameStr, substring) {
					return true
				}
			}
		}
		// Recurse into all values
		for _, val := range v {
			if containsSpanName(val, substring) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if containsSpanName(item, substring) {
				return true
			}
		}
	}
	return false
}

// waitForMetrics polls the collector container's metric export file until non-empty or timeout.
// Returns raw JSON bytes (newline-delimited JSON, each line is a ResourceMetrics batch).
func waitForMetrics(t *testing.T, ctx context.Context, collector testcontainers.Container, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		reader, err := collector.CopyFileFromContainer(ctx, "/tmp/otel-metrics.json")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(reader)
		reader.Close()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		data := buf.Bytes()
		if len(bytes.TrimSpace(data)) > 0 {
			return data
		}

		time.Sleep(2 * time.Second)
	}

	require.Fail(t, "Timed out waiting for metrics to appear in collector export file")
	return nil
}

// assertMetricExists parses the OTLP JSON metric export (newline-delimited JSON)
// and asserts at least one metric name contains the given substring.
func assertMetricExists(t *testing.T, metricData []byte, metricNameSubstring string) {
	t.Helper()

	scanner := bufio.NewScanner(bytes.NewReader(metricData))
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var batch map[string]interface{}
		if err := json.Unmarshal([]byte(line), &batch); err != nil {
			continue
		}

		if containsMetricName(batch, metricNameSubstring) {
			return
		}
	}

	require.Failf(t, "Metric not found", "Expected metric with name containing %q in collector export", metricNameSubstring)
}

// containsMetricName recursively searches a JSON structure for a metric name containing the substring.
func containsMetricName(data interface{}, substring string) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this is a metric with a matching name
		if name, ok := v["name"]; ok {
			if nameStr, ok := name.(string); ok {
				if strings.Contains(nameStr, substring) {
					return true
				}
			}
		}
		// Recurse into all values
		for _, val := range v {
			if containsMetricName(val, substring) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if containsMetricName(item, substring) {
				return true
			}
		}
	}
	return false
}
