package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestShouldEmitOrderTelemetry(t *testing.T) {
	t.Run("returns true for live environment", func(t *testing.T) {
		assert.True(t, ShouldEmitOrderTelemetry("live"))
	})

	t.Run("returns true for reconcile environment", func(t *testing.T) {
		assert.True(t, ShouldEmitOrderTelemetry("reconcile"))
	})

	t.Run("returns false for simulator environment", func(t *testing.T) {
		assert.False(t, ShouldEmitOrderTelemetry("simulator"))
	})
}

func TestInit(t *testing.T) {
	// Set up an in-memory meter provider so Init() gets a real provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer provider.Shutdown(t.Context())

	err := Init()
	require.NoError(t, err)

	assert.NotNil(t, OrdersPlaced, "OrdersPlaced should be non-nil after Init")
	assert.NotNil(t, OrdersFilled, "OrdersFilled should be non-nil after Init")
	assert.NotNil(t, OrdersRejected, "OrdersRejected should be non-nil after Init")
	assert.NotNil(t, CandlesProcessed, "CandlesProcessed should be non-nil after Init")
	assert.NotNil(t, SignalsGenerated, "SignalsGenerated should be non-nil after Init")
	assert.NotNil(t, ActivePlaygrounds, "ActivePlaygrounds should be non-nil after Init")
	assert.NotNil(t, OpenOrders, "OpenOrders should be non-nil after Init")
	assert.NotNil(t, UptimeSeconds, "UptimeSeconds should be non-nil after Init")
}

func TestPlaygroundAttrs(t *testing.T) {
	t.Run("returns non-nil option with client_id", func(t *testing.T) {
		opt := PlaygroundAttrs("live", "margin", "my-strategy")
		assert.NotNil(t, opt, "PlaygroundAttrs should return a non-nil option")
	})

	t.Run("returns non-nil option with empty client_id", func(t *testing.T) {
		opt := PlaygroundAttrs("live", "margin", "")
		assert.NotNil(t, opt, "PlaygroundAttrs should return a non-nil option for empty client_id")
	})
}

func TestClientIDOrEmpty(t *testing.T) {
	t.Run("returns value when non-nil", func(t *testing.T) {
		s := "my-strategy"
		assert.Equal(t, "my-strategy", ClientIDOrEmpty(&s))
	})

	t.Run("returns empty string when nil", func(t *testing.T) {
		assert.Equal(t, "", ClientIDOrEmpty(nil))
	})
}
