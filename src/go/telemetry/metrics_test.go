package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	Init()

	assert.NotNil(t, Default, "Default registry should be non-nil after Init")
	assert.NotNil(t, OrdersPlaced, "OrdersPlaced should be non-nil after Init")
	assert.NotNil(t, OrdersFilled, "OrdersFilled should be non-nil after Init")
	assert.NotNil(t, OrdersRejected, "OrdersRejected should be non-nil after Init")
	assert.NotNil(t, CandlesProcessed, "CandlesProcessed should be non-nil after Init")
	assert.NotNil(t, SignalsGenerated, "SignalsGenerated should be non-nil after Init")
	assert.NotNil(t, SignalsConsumed, "SignalsConsumed should be non-nil after Init")
	assert.NotNil(t, ErrorsTotal, "ErrorsTotal should be non-nil after Init")
	assert.NotNil(t, ActivePlaygrounds, "ActivePlaygrounds should be non-nil after Init")
	assert.NotNil(t, OpenOrders, "OpenOrders should be non-nil after Init")
	assert.NotNil(t, UptimeSeconds, "UptimeSeconds should be non-nil after Init")
}

func TestRecordingBeforeWriterStarts(t *testing.T) {
	Init()

	OrdersPlaced.Add(1, PlaygroundAttrs("simulator", "simulator", "test-client")...)

	got := snapshotValue(t, Default.Snapshot(), "grodt.orders.placed", map[string]string{
		"mode":         "simulator",
		"account_type": "simulator",
		"client_id":    "test-client",
	})
	assert.Equal(t, float64(1), got, "recording after Init but before persistence starts is captured")
}

func TestPlaygroundAttrs(t *testing.T) {
	t.Run("carries mode, account_type and client_id", func(t *testing.T) {
		labels := PlaygroundAttrs("live", "margin", "my-strategy")
		assert.Equal(t, []Label{
			{Key: "mode", Value: "live"},
			{Key: "account_type", Value: "margin"},
			{Key: "client_id", Value: "my-strategy"},
		}, labels)
	})

	t.Run("empty client_id is preserved", func(t *testing.T) {
		labels := PlaygroundAttrs("live", "margin", "")
		assert.Equal(t, "", labels[2].Value)
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
