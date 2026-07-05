package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradeSignal_NewTradeSignal(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	attrs := map[string]interface{}{
		"fast_period": float64(10),
		"slow_period": float64(20),
	}

	signal := models.NewTradeSignal(
		models.SignalMACrossover,
		models.StockSymbol("AAPL"),
		ts,
		attrs,
	)

	assert.NotEqual(t, uuid.Nil, signal.ID, "ID should be non-zero UUID")
	assert.Equal(t, models.SignalMACrossover, signal.Name)
	assert.Equal(t, models.StockSymbol("AAPL"), signal.Symbol)
	assert.Equal(t, ts, signal.Timestamp)
	assert.Equal(t, float64(10), signal.Attributes["fast_period"])
	assert.Equal(t, float64(20), signal.Attributes["slow_period"])
}

func TestTradeSignal_GetSavedEventParameters_DefaultGlobalStream(t *testing.T) {
	ts := time.Now()
	signal := models.NewTradeSignal(
		models.SignalMACrossover,
		models.StockSymbol("AAPL"),
		ts,
		nil,
	)

	params := signal.GetSavedEventParameters()

	// Default: global trade-signals stream (not per-symbol)
	assert.Equal(t, models.StreamName("trade-signals"), params.StreamName)
	assert.Equal(t, models.TradeSignalEventName, params.EventName)
	assert.Equal(t, 1, params.SchemaVersion)
}

func TestTradeSignal_SetStreamName_OverridesStream(t *testing.T) {
	ts := time.Now()
	signal := models.NewTradeSignal(
		models.SignalMACrossover,
		models.StockSymbol("AAPL"),
		ts,
		nil,
	)

	// Override to a sim stream
	simStream := models.NewSimSignalStreamName("abc-123")
	signal.SetStreamName(simStream)

	params := signal.GetSavedEventParameters()
	assert.Equal(t, models.StreamName("trade-signals-sim-abc-123"), params.StreamName)
}

func TestNewSimSignalStreamName(t *testing.T) {
	stream := models.NewSimSignalStreamName("playground-uuid-here")
	assert.Equal(t, models.StreamName("trade-signals-sim-playground-uuid-here"), stream)
}

func TestSignalName_Validate_Known(t *testing.T) {
	knownSignals := []models.SignalName{
		models.SignalMACrossover,
		models.SignalStartOfWeek,
		models.SignalCoveredCall,
		models.SignalMeanReversion,
	}

	for _, name := range knownSignals {
		err := name.Validate()
		assert.NoError(t, err, "Validate should return nil for known signal %q", name)
	}
}

func TestSignalName_Validate_Unknown(t *testing.T) {
	unknown := models.SignalName("invalid_signal")
	err := unknown.Validate()
	assert.Error(t, err, "Validate should return error for unknown signal")
	assert.Contains(t, err.Error(), "invalid_signal")
}

func TestTradeSignal_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	attrs := map[string]interface{}{
		"strategy":   "momentum",
		"confidence": float64(0.85),
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	original := models.NewTradeSignal(
		models.SignalCoveredCall,
		models.StockSymbol("MSFT"),
		ts,
		attrs,
	)

	data, err := json.Marshal(original)
	require.NoError(t, err, "Marshal should not error")

	var restored models.TradeSignal
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err, "Unmarshal should not error")

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Symbol, restored.Symbol)
	assert.True(t, original.Timestamp.Equal(restored.Timestamp))
	assert.Equal(t, "momentum", restored.Attributes["strategy"])
	assert.Equal(t, float64(0.85), restored.Attributes["confidence"])

	nested, ok := restored.Attributes["nested"].(map[string]interface{})
	require.True(t, ok, "nested should be map[string]interface{}")
	assert.Equal(t, "value", nested["key"])
}
