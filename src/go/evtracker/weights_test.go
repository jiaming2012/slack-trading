package evtracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyWeightBands exercises every weight band including the exact 0.1 and
// 0.0 boundaries (stable), a scale-up slope, a decaying slope, and a null slope.
func TestClassifyWeightBands(t *testing.T) {
	cases := []struct {
		name       string
		slope      *float64
		wantWeight float64
		wantStatus DecayStatus
	}{
		{"improving", f(0.2), WeightScaleUp, StatusImproving},
		{"boundary 0.1 is stable", f(0.1), WeightStable, StatusStable},
		{"boundary 0.0 is stable", f(0.0), WeightStable, StatusStable},
		{"positive within band", f(0.05), WeightStable, StatusStable},
		{"decaying", f(-0.05), WeightDecay, StatusDecaying},
		{"null slope is insufficient data", nil, WeightStable, StatusInsufficientData},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			weight, status := ClassifyWeight(tc.slope)
			assert.Equal(t, tc.wantWeight, weight)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

// TestRankByEV verifies descending-EV ranking assigns ranks 1/2/3 to EVs
// 0.8/0.1/-0.3.
func TestRankByEV(t *testing.T) {
	results := []StrategyEVResult{
		{StrategyID: "b", Regime: "r", EvAllTime: 0.1},
		{StrategyID: "c", Regime: "r", EvAllTime: -0.3},
		{StrategyID: "a", Regime: "r", EvAllTime: 0.8},
	}

	ranked := RankByEV(results)
	require.Len(t, ranked, 3)
	assert.InDelta(t, 0.8, ranked[0].EvAllTime, 1e-9)
	assert.Equal(t, 1, ranked[0].Rank)
	assert.InDelta(t, 0.1, ranked[1].EvAllTime, 1e-9)
	assert.Equal(t, 2, ranked[1].Rank)
	assert.InDelta(t, -0.3, ranked[2].EvAllTime, 1e-9)
	assert.Equal(t, 3, ranked[2].Rank)
}

// TestRankByEVTieBreak verifies equal-EV groups tie-break by strategy_id then
// regime deterministically.
func TestRankByEVTieBreak(t *testing.T) {
	results := []StrategyEVResult{
		{StrategyID: "s", Regime: "bull", EvAllTime: 0.5},
		{StrategyID: "s", Regime: "bear", EvAllTime: 0.5},
		{StrategyID: "a", Regime: "bull", EvAllTime: 0.5},
	}

	ranked := RankByEV(results)
	assert.Equal(t, "a", ranked[0].StrategyID)
	assert.Equal(t, "bear", ranked[1].Regime)
	assert.Equal(t, "bull", ranked[2].Regime)
}

// TestDeterminism verifies the same fixture and as-of computed twice yields
// identical results.
func TestDeterminism(t *testing.T) {
	trades := []TradeOutcome{
		{StrategyID: "s1", Regime: "bull", ClosedAt: daysBefore(5), PnL: 2.0},
		{StrategyID: "s1", Regime: "bull", ClosedAt: daysBefore(40), PnL: 1.0},
		{StrategyID: "s1", Regime: "bull", ClosedAt: daysBefore(70), PnL: 0.5},
		{StrategyID: "s2", Regime: "bear", ClosedAt: daysBefore(3), PnL: -1.0},
	}

	first := Compute(trades, asOfRef, Options{})
	second := Compute(trades, asOfRef, Options{})
	require.Equal(t, len(first), len(second))
	for i := range first {
		assert.Equal(t, first[i].StrategyID, second[i].StrategyID)
		assert.Equal(t, first[i].Regime, second[i].Regime)
		assert.Equal(t, first[i].Ev30d, second[i].Ev30d)
		assert.Equal(t, first[i].Ev90d, second[i].Ev90d)
		assert.Equal(t, first[i].EvAllTime, second[i].EvAllTime)
		assert.Equal(t, first[i].EvWeight, second[i].EvWeight)
		assert.Equal(t, first[i].Status, second[i].Status)
		assert.Equal(t, first[i].Rank, second[i].Rank)
		if first[i].EvSlope == nil {
			assert.Nil(t, second[i].EvSlope)
		} else {
			require.NotNil(t, second[i].EvSlope)
			assert.Equal(t, *first[i].EvSlope, *second[i].EvSlope)
		}
	}
}

func f(v float64) *float64 { return &v }
