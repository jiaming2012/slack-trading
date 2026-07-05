package crowding

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkCandidate(scannedAt time.Time, ticker string, strategyIDs ...string) ScanCandidate {
	return ScanCandidate{
		ScanResultID: uuid.New(),
		Ticker:       ticker,
		ScannedAt:    scannedAt,
		StrategyIDs:  strategyIDs,
	}
}

// TestDetectCrowding_KnownOverlapPercentage asserts a fixture of 10
// candidates where exactly 3 have 2+ distinct strategies yields
// TotalCandidates=10, OverlappingCandidates=3, OverlapPct=30.0.
func TestDetectCrowding_KnownOverlapPercentage(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "AAPL", "strat_a", "strat_b"), // overlapping
		mkCandidate(now, "MSFT", "strat_a", "strat_c"), // overlapping
		mkCandidate(now, "GOOG", "strat_b", "strat_c"), // overlapping
		mkCandidate(now, "TSLA", "strat_a"),
		mkCandidate(now, "AMZN", "strat_b"),
		mkCandidate(now, "META", "strat_c"),
		mkCandidate(now, "NFLX", "strat_a"),
		mkCandidate(now, "NVDA", "strat_b"),
		mkCandidate(now, "AMD"),
		mkCandidate(now, "INTC"),
	}

	result, err := DetectCrowding(candidates, DefaultOverlapThresholdPct)
	require.NoError(t, err)

	assert.Equal(t, 10, result.TotalCandidates)
	assert.Equal(t, 3, result.OverlappingCandidates)
	assert.InDelta(t, 30.0, result.OverlapPct, 1e-9)
	assert.Len(t, result.OverlappingCandidateDetails, 3)
}

// TestDetectCrowding_ZeroOverlap asserts OverlapPct is 0.0 when no candidate
// has more than one distinct strategy.
func TestDetectCrowding_ZeroOverlap(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "AAPL", "strat_a"),
		mkCandidate(now, "MSFT", "strat_b"),
		mkCandidate(now, "GOOG"),
	}

	result, err := DetectCrowding(candidates, DefaultOverlapThresholdPct)
	require.NoError(t, err)

	assert.Equal(t, 0, result.OverlappingCandidates)
	assert.InDelta(t, 0.0, result.OverlapPct, 1e-9)
	assert.False(t, result.Flagged)
}

// TestDetectCrowding_FullOverlap asserts OverlapPct is 100.0 when every
// candidate has 2+ distinct strategies.
func TestDetectCrowding_FullOverlap(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "AAPL", "strat_a", "strat_b"),
		mkCandidate(now, "MSFT", "strat_a", "strat_c", "strat_d"),
		mkCandidate(now, "GOOG", "strat_b", "strat_c"),
	}

	result, err := DetectCrowding(candidates, DefaultOverlapThresholdPct)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalCandidates)
	assert.Equal(t, 3, result.OverlappingCandidates)
	assert.InDelta(t, 100.0, result.OverlapPct, 1e-9)
	assert.True(t, result.Flagged)
}

// TestDetectCrowding_NoSimulatingStrategies asserts candidates with zero
// sim_outcomes rows (no strategy ids) count toward TotalCandidates but never
// OverlappingCandidates.
func TestDetectCrowding_NoSimulatingStrategies(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "AAPL"),
		mkCandidate(now, "MSFT"),
		mkCandidate(now, "GOOG", "strat_a", "strat_b"),
	}

	result, err := DetectCrowding(candidates, DefaultOverlapThresholdPct)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalCandidates)
	assert.Equal(t, 1, result.OverlappingCandidates)
}

// TestDetectCrowding_DuplicateStrategyIDsAreNotDoubleCounted asserts a
// candidate whose StrategyIDs contains the same strategy twice (e.g. two
// sim_outcomes rows from the same strategy) is NOT counted as overlapping,
// since only distinct strategies count.
func TestDetectCrowding_DuplicateStrategyIDsAreNotDoubleCounted(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "AAPL", "strat_a", "strat_a"),
		mkCandidate(now, "MSFT", "strat_a", "strat_b"),
	}

	result, err := DetectCrowding(candidates, DefaultOverlapThresholdPct)
	require.NoError(t, err)

	assert.Equal(t, 1, result.OverlappingCandidates)
}

// TestDetectCrowding_ThresholdBoundary_AboveIsFlagged asserts overlap above
// the threshold is flagged true.
func TestDetectCrowding_ThresholdBoundary_AboveIsFlagged(t *testing.T) {
	now := time.Now().UTC()

	// 3 of 10 overlapping -> 30.0% overlap, threshold 25.0 -> flagged.
	candidates := []ScanCandidate{
		mkCandidate(now, "A", "s1", "s2"),
		mkCandidate(now, "B", "s1", "s2"),
		mkCandidate(now, "C", "s1", "s2"),
		mkCandidate(now, "D", "s1"),
		mkCandidate(now, "E", "s1"),
		mkCandidate(now, "F", "s1"),
		mkCandidate(now, "G", "s1"),
		mkCandidate(now, "H", "s1"),
		mkCandidate(now, "I", "s1"),
		mkCandidate(now, "J", "s1"),
	}

	result, err := DetectCrowding(candidates, 25.0)
	require.NoError(t, err)

	assert.InDelta(t, 30.0, result.OverlapPct, 1e-9)
	assert.True(t, result.Flagged)
}

// TestDetectCrowding_ThresholdBoundary_ExactMatchIsNotFlagged asserts overlap
// exactly equal to the threshold is NOT flagged (strict greater-than only).
func TestDetectCrowding_ThresholdBoundary_ExactMatchIsNotFlagged(t *testing.T) {
	now := time.Now().UTC()

	// 3 of 10 overlapping -> exactly 30.0% overlap, threshold 30.0 -> not flagged.
	candidates := []ScanCandidate{
		mkCandidate(now, "A", "s1", "s2"),
		mkCandidate(now, "B", "s1", "s2"),
		mkCandidate(now, "C", "s1", "s2"),
		mkCandidate(now, "D", "s1"),
		mkCandidate(now, "E", "s1"),
		mkCandidate(now, "F", "s1"),
		mkCandidate(now, "G", "s1"),
		mkCandidate(now, "H", "s1"),
		mkCandidate(now, "I", "s1"),
		mkCandidate(now, "J", "s1"),
	}

	result, err := DetectCrowding(candidates, 30.0)
	require.NoError(t, err)

	assert.InDelta(t, 30.0, result.OverlapPct, 1e-9)
	assert.False(t, result.Flagged)
}

// TestDetectCrowding_ThresholdBoundary_BelowIsNotFlagged asserts overlap
// below the threshold is not flagged.
func TestDetectCrowding_ThresholdBoundary_BelowIsNotFlagged(t *testing.T) {
	now := time.Now().UTC()

	candidates := []ScanCandidate{
		mkCandidate(now, "A", "s1", "s2"),
		mkCandidate(now, "B", "s1"),
		mkCandidate(now, "C", "s1"),
		mkCandidate(now, "D", "s1"),
		mkCandidate(now, "E", "s1"),
		mkCandidate(now, "F", "s1"),
		mkCandidate(now, "G", "s1"),
		mkCandidate(now, "H", "s1"),
		mkCandidate(now, "I", "s1"),
		mkCandidate(now, "J", "s1"),
	}

	result, err := DetectCrowding(candidates, 50.0)
	require.NoError(t, err)

	assert.InDelta(t, 10.0, result.OverlapPct, 1e-9)
	assert.False(t, result.Flagged)
}

// TestDetectCrowding_EmptyCandidatesReturnsError asserts an empty candidate
// slice is rejected rather than producing an undefined percentage.
func TestDetectCrowding_EmptyCandidatesReturnsError(t *testing.T) {
	_, err := DetectCrowding(nil, DefaultOverlapThresholdPct)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyCandidates)
}

// TestFakeCrowdingStore_PersistRecordsInputVerbatim asserts the in-memory
// fake exposes the same metric values and flagged-candidate count that were
// passed to Persist.
func TestFakeCrowdingStore_PersistRecordsInputVerbatim(t *testing.T) {
	now := time.Now().UTC()

	metric := CrowdingMetric{
		ScannedAt:             now,
		ComputedAt:            now,
		TotalCandidates:       10,
		OverlappingCandidates: 2,
		OverlapPct:            20.0,
		ThresholdPct:          30.0,
		Flagged:               false,
	}

	flagged := []CrowdingFlaggedCandidate{
		{ScanResultID: uuid.New(), Ticker: "AAPL", StrategyIDs: JSONArray{"strat_a", "strat_b"}},
		{ScanResultID: uuid.New(), Ticker: "MSFT", StrategyIDs: JSONArray{"strat_a", "strat_c"}},
	}

	store := NewFakeCrowdingStore()
	err := store.Persist(metric, flagged)
	require.NoError(t, err)

	assert.Equal(t, 1, store.PersistCalls)
	assert.Equal(t, metric.TotalCandidates, store.LastMetric.TotalCandidates)
	assert.Equal(t, metric.OverlappingCandidates, store.LastMetric.OverlappingCandidates)
	assert.InDelta(t, metric.OverlapPct, store.LastMetric.OverlapPct, 1e-9)
	assert.InDelta(t, metric.ThresholdPct, store.LastMetric.ThresholdPct, 1e-9)
	assert.Equal(t, metric.Flagged, store.LastMetric.Flagged)
	require.Len(t, store.LastFlagged, 2)
	assert.Equal(t, "AAPL", store.LastFlagged[0].Ticker)
	assert.Equal(t, "MSFT", store.LastFlagged[1].Ticker)
}
