package models

import (
	"testing"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface satisfaction check.
var _ ISignalRepository = &ESDBSignalRepository{}

func TestESDBSignalRepository_ImplementsInterface(t *testing.T) {
	// Verify at runtime that the type assertion holds.
	var repo ISignalRepository = &ESDBSignalRepository{}
	assert.NotNil(t, repo)
}

func TestESDBSignalRepository_WriteNilSignal(t *testing.T) {
	// A nil esdbProducer is acceptable here because Write should
	// guard against nil signal before attempting to use the producer.
	repo := NewESDBSignalRepository(nil)
	err := repo.Write(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil signal")
}

func TestTradeSignal_GlobalStreamName(t *testing.T) {
	// Per D-01: all symbols must use the single global "trade-signals" stream.
	symbols := []eventmodels.StockSymbol{"AAPL", "MSFT", "GOOG", "TSLA"}
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	for _, sym := range symbols {
		signal := eventmodels.NewTradeSignal(
			eventmodels.SignalMeanReversion,
			sym,
			base,
			nil,
		)

		params := signal.GetSavedEventParameters()
		assert.Equal(t, eventmodels.TradeSignalStream, params.StreamName,
			"expected global stream 'trade-signals' for symbol %s, got %s", sym, params.StreamName)
	}
}
