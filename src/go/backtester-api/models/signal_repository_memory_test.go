package models

import (
	"sync"
	"testing"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInMemorySignalRepository verifies clock-gated delivery, sorted insertion,
// cursor advancement, and thread safety of the in-memory signal repository.

func TestInMemorySignalRepository_ClockGatedDelivery(t *testing.T) {
	// Write 3 signals at T+0, T+5min, T+10min.
	// ReadPending(T+0) returns 1 signal.
	// ReadPending(T+5min) returns 1 signal (not re-delivering first).
	// ReadPending(T+10min) returns 1 signal.
	// ReadPending(T+11min) returns 0.
	repo := NewInMemorySignalRepository()
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	s0 := eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, eventmodels.StockSymbol("AAPL"), base, nil)
	s5 := eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, eventmodels.StockSymbol("AAPL"), base.Add(5*time.Minute), nil)
	s10 := eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, eventmodels.StockSymbol("AAPL"), base.Add(10*time.Minute), nil)

	require.NoError(t, repo.Write(s0))
	require.NoError(t, repo.Write(s5))
	require.NoError(t, repo.Write(s10))

	// ReadPending at T+0 should return only the first signal
	pending := repo.ReadPending(base)
	require.Len(t, pending, 1)
	assert.Equal(t, s0.ID, pending[0].ID)

	// ReadPending at T+5min should return only the second signal (first already consumed)
	pending = repo.ReadPending(base.Add(5 * time.Minute))
	require.Len(t, pending, 1)
	assert.Equal(t, s5.ID, pending[0].ID)

	// ReadPending at T+10min should return only the third signal
	pending = repo.ReadPending(base.Add(10 * time.Minute))
	require.Len(t, pending, 1)
	assert.Equal(t, s10.ID, pending[0].ID)

	// ReadPending at T+11min should return nothing (all consumed)
	pending = repo.ReadPending(base.Add(11 * time.Minute))
	assert.Len(t, pending, 0)
}

func TestInMemorySignalRepository_OutOfOrderWrites(t *testing.T) {
	// Write signals out of order (T+10, T+0, T+5).
	// ReadPending(T+10min) returns all 3 in timestamp order.
	repo := NewInMemorySignalRepository()
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	s10 := eventmodels.NewTradeSignal(eventmodels.SignalMACrossover, eventmodels.StockSymbol("AAPL"), base.Add(10*time.Minute), nil)
	s0 := eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, eventmodels.StockSymbol("AAPL"), base, nil)
	s5 := eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, eventmodels.StockSymbol("AAPL"), base.Add(5*time.Minute), nil)

	require.NoError(t, repo.Write(s10))
	require.NoError(t, repo.Write(s0))
	require.NoError(t, repo.Write(s5))

	pending := repo.ReadPending(base.Add(10 * time.Minute))
	require.Len(t, pending, 3)
	assert.Equal(t, s0.ID, pending[0].ID)
	assert.Equal(t, s5.ID, pending[1].ID)
	assert.Equal(t, s10.ID, pending[2].ID)
}

func TestInMemorySignalRepository_GetAllAfterPartialConsumption(t *testing.T) {
	// GetAll returns all signals regardless of cursor position
	// (even after ReadPending consumed some).
	repo := NewInMemorySignalRepository()
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	s0 := eventmodels.NewTradeSignal(eventmodels.SignalMeanReversion, eventmodels.StockSymbol("AAPL"), base, nil)
	s5 := eventmodels.NewTradeSignal(eventmodels.SignalCoveredCall, eventmodels.StockSymbol("AAPL"), base.Add(5*time.Minute), nil)

	require.NoError(t, repo.Write(s0))
	require.NoError(t, repo.Write(s5))

	// Consume the first signal
	pending := repo.ReadPending(base)
	require.Len(t, pending, 1)

	// GetAll should still return both
	all := repo.GetAll()
	require.Len(t, all, 2)
	assert.Equal(t, s0.ID, all[0].ID)
	assert.Equal(t, s5.ID, all[1].ID)
}

func TestInMemorySignalRepository_WriteNilSignal(t *testing.T) {
	repo := NewInMemorySignalRepository()
	err := repo.Write(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil signal")
}

func TestInMemorySignalRepository_ReadPendingEmpty(t *testing.T) {
	// ReadPending with no signals returns empty slice (not nil).
	repo := NewInMemorySignalRepository()
	pending := repo.ReadPending(time.Now())
	require.NotNil(t, pending)
	assert.Len(t, pending, 0)
}

func TestInMemorySignalRepository_ConcurrentWrites(t *testing.T) {
	// Concurrent writes are safe (no race detector failures).
	repo := NewInMemorySignalRepository()
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			s := eventmodels.NewTradeSignal(
				eventmodels.SignalMeanReversion,
				eventmodels.StockSymbol("AAPL"),
				base.Add(time.Duration(offset)*time.Second),
				nil,
			)
			_ = repo.Write(s)
		}(i)
	}
	wg.Wait()

	all := repo.GetAll()
	assert.Len(t, all, 100)

	// Verify sorted order
	for i := 1; i < len(all); i++ {
		assert.False(t, all[i].Timestamp.Before(all[i-1].Timestamp),
			"signals should be in timestamp order, but index %d (%v) is before index %d (%v)",
			i, all[i].Timestamp, i-1, all[i-1].Timestamp)
	}
}
