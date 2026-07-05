package evtracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWindowBoundaries verifies a 45-day-old trade is excluded from ev_30d but
// included in ev_90d/all-time, and a post-as-of trade is excluded everywhere.
func TestWindowBoundaries(t *testing.T) {
	trades := []TradeOutcome{
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(10), PnL: 2.0},                    // in 30d + 90d
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(45), PnL: -4.0},                   // in 90d only
		{StrategyID: "s", Regime: "bull", ClosedAt: asOfRef.Add(5 * 24 * time.Hour), PnL: 100.0}, // after as-of
	}

	results := Compute(trades, asOfRef, Options{})
	require.Len(t, results, 1)
	r := results[0]

	assert.InDelta(t, 2.0, r.Ev30d, 1e-9, "45-day trade must be excluded from ev_30d")
	assert.InDelta(t, -1.0, r.Ev90d, 1e-9, "45-day trade included in ev_90d: mean(2.0,-4.0)")
	assert.InDelta(t, -1.0, r.EvAllTime, 1e-9, "post-as-of trade excluded from all-time")
}

// TestBucketEVsLinearSeriesSlope builds four consecutive non-empty buckets whose
// per-bucket EVs are 0.0, 0.2, 0.4, 0.6 and verifies both BucketEVs and the
// resulting OLS slope (0.2).
func TestBucketEVsLinearSeriesSlope(t *testing.T) {
	base := asOfRef.Add(-200 * 24 * time.Hour)
	day := func(n int) time.Time { return base.Add(time.Duration(n) * 24 * time.Hour) }

	trades := []TradeOutcome{
		// bucket 0 -> EV 0.0 (one +1 winner, one -1 loser => mean 0)
		{ClosedAt: day(0), PnL: 1.0},
		{ClosedAt: day(1), PnL: -1.0},
		// bucket 1 -> EV 0.2
		{ClosedAt: day(35), PnL: 0.2},
		// bucket 2 -> EV 0.4
		{ClosedAt: day(65), PnL: 0.4},
		// bucket 3 -> EV 0.6
		{ClosedAt: day(95), PnL: 0.6},
	}

	evs := BucketEVs(trades, DefaultBucketDays)
	require.Len(t, evs, 4)
	assert.InDelta(t, 0.0, evs[0], 1e-9)
	assert.InDelta(t, 0.2, evs[1], 1e-9)
	assert.InDelta(t, 0.4, evs[2], 1e-9)
	assert.InDelta(t, 0.6, evs[3], 1e-9)

	slope, ok := OLSSlope(evs)
	require.True(t, ok)
	assert.InDelta(t, 0.2, slope, 1e-9)
}

// TestOLSSlopeLiteralSeries pins the OLS slope of the exact documented series.
func TestOLSSlopeLiteralSeries(t *testing.T) {
	slope, ok := OLSSlope([]float64{0.0, 0.2, 0.4, 0.6})
	require.True(t, ok)
	assert.InDelta(t, 0.2, slope, 1e-9)
}

// TestSingleBucketNullSlope verifies that trades falling in a single bucket yield
// a null (undefined) slope, surfaced as a nil EvSlope on the result.
func TestSingleBucketNullSlope(t *testing.T) {
	trades := []TradeOutcome{
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(3), PnL: 1.0},
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(5), PnL: -1.0},
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(7), PnL: 2.0},
	}

	evs := BucketEVs(trades, DefaultBucketDays)
	require.Len(t, evs, 1, "all trades fall in one 30-day bucket")
	_, ok := OLSSlope(evs)
	assert.False(t, ok, "fewer than two buckets => undefined slope")

	results := Compute(trades, asOfRef, Options{})
	require.Len(t, results, 1)
	assert.Nil(t, results[0].EvSlope, "single-bucket group must have null slope")
	assert.Equal(t, StatusInsufficientData, results[0].Status)
}
