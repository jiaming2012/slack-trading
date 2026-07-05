package riskoverlay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 6.11 — seeded flagged cycle returns the expected flagged ticker set.
func TestFakeCrowdingLookup_SeededView(t *testing.T) {
	scannedAt := time.Date(2026, 7, 4, 14, 30, 0, 0, time.UTC)
	fake := NewFakeCrowdingLookup()
	fake.Seed(scannedAt, true, []string{"NVDA", "SMCI"})

	view, err := fake.ViewForScanCycle(scannedAt)
	require.NoError(t, err)
	require.True(t, view.Flagged)
	require.True(t, view.IsFlagged("NVDA"))
	require.True(t, view.IsFlagged("SMCI"))
	require.False(t, view.IsFlagged("KO"))
	require.ElementsMatch(t, []string{"NVDA", "SMCI"}, view.Tickers())
}

// An unseeded scan cycle returns an unflagged, empty view without error.
func TestFakeCrowdingLookup_UnseededIsEmpty(t *testing.T) {
	fake := NewFakeCrowdingLookup()
	view, err := fake.ViewForScanCycle(time.Now())
	require.NoError(t, err)
	require.False(t, view.Flagged)
	require.Empty(t, view.Tickers())
	require.False(t, view.IsFlagged("NVDA"))
}

// A seeded-but-not-flagged cycle carries no flagged tickers.
func TestNewCrowdingView_EmptyTickers(t *testing.T) {
	view := NewCrowdingView(false, nil)
	require.False(t, view.IsFlagged("ANY"))
	require.Empty(t, view.Tickers())
}
