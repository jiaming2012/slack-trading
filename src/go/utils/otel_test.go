package utils

import (
	"bytes"
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSetupOTelSDK(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	ctx := context.Background()
	shutdown, err := SetupOTelSDK(ctx, "test-service", "0.1.0")
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Verify TracerProvider is a real SDK provider, not the no-op default
	tp := otel.GetTracerProvider()
	_, ok := tp.(*sdktrace.TracerProvider)
	assert.True(t, ok, "expected *sdktrace.TracerProvider, got %T", tp)

	// Verify MeterProvider is a real SDK provider, not the no-op default
	mp := otel.GetMeterProvider()
	_, ok = mp.(*sdkmetric.MeterProvider)
	assert.True(t, ok, "expected *sdkmetric.MeterProvider, got %T", mp)

	// Verify W3C TraceContext propagator is set (composite with TraceContext + Baggage)
	prop := otel.GetTextMapPropagator()
	assert.NotNil(t, prop, "expected text map propagator to be set")
	// Verify it handles both traceparent and baggage fields (composite propagator behavior)
	fields := prop.Fields()
	assert.Contains(t, fields, "traceparent", "expected propagator to handle traceparent field")
	assert.Contains(t, fields, "baggage", "expected propagator to handle baggage field")

	// Clean up -- shutdown may return connection errors when no collector is running,
	// which is expected in unit tests. The important assertion is that providers were set.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = shutdown(shutdownCtx)
}

func TestOTelShutdown(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	ctx := context.Background()
	shutdown, err := SetupOTelSDK(ctx, "test-service", "0.1.0")
	require.NoError(t, err, "SetupOTelSDK should not error")
	require.NotNil(t, shutdown, "shutdown function should not be nil")

	// Calling shutdown should not panic. Connection errors are expected
	// when no OTel Collector is running (unit test environment).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NotPanics(t, func() {
		_ = shutdown(shutdownCtx)
	}, "shutdown should not panic")
}

func TestLogfmtFormat(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&log.TextFormatter{
		DisableColors:  true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		FieldMap: log.FieldMap{
			log.FieldKeyTime:  "ts",
			log.FieldKeyLevel: "level",
			log.FieldKeyMsg:   "msg",
		},
	})

	logger.WithField("playground_id", "test-123").Info("test message")

	output := buf.String()
	assert.Contains(t, output, "playground_id=test-123", "expected logfmt key=value pair for playground_id")
	assert.Contains(t, output, "msg=\"test message\"", "expected logfmt msg field")
	assert.NotContains(t, output, "{", "output should not be JSON format")
}
