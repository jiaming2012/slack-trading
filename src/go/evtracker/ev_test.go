package evtracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asOfRef is a fixed reference timestamp used across the pure unit tests so no
// test reads the wall clock.
var asOfRef = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

// daysBefore returns a timestamp n days before the reference as-of.
func daysBefore(n int) time.Time {
	return asOfRef.Add(-time.Duration(n) * 24 * time.Hour)
}

// TestComputeEVKnownFixture pins the documented fixture: 6 winners at +2.0 and 4
// losers at -1.0 give EV 0.8.
func TestComputeEVKnownFixture(t *testing.T) {
	var trades []TradeOutcome
	for i := 0; i < 6; i++ {
		trades = append(trades, TradeOutcome{StrategyID: "s", Regime: "bull", ClosedAt: asOfRef, PnL: 2.0})
	}
	for i := 0; i < 4; i++ {
		trades = append(trades, TradeOutcome{StrategyID: "s", Regime: "bull", ClosedAt: asOfRef, PnL: -1.0})
	}

	stats := ComputeEV(trades)
	assert.Equal(t, 6, stats.Wins)
	assert.Equal(t, 4, stats.Losses)
	assert.Equal(t, 10, stats.Decided)
	assert.InDelta(t, 0.6, stats.WinRate, 1e-9)
	assert.InDelta(t, 2.0, stats.AvgWin, 1e-9)
	assert.InDelta(t, 0.4, stats.LossRate, 1e-9)
	assert.InDelta(t, 1.0, stats.AvgLoss, 1e-9)
	assert.InDelta(t, 0.8, stats.EV, 1e-9)
}

// TestComputeEVBreakevenExcluded verifies breakeven trades do not count toward
// decided, avg_win, or avg_loss, and that win_rate + loss_rate == 1.
func TestComputeEVBreakevenExcluded(t *testing.T) {
	trades := []TradeOutcome{
		{PnL: 3.0}, {PnL: 3.0}, {PnL: 3.0}, // 3 winners
		{PnL: -1.0}, {PnL: -1.0}, // 2 losers
		{PnL: 0.0}, // 1 breakeven
	}

	stats := ComputeEV(trades)
	assert.Equal(t, 5, stats.Decided)
	assert.Equal(t, 1, stats.Breakeven)
	assert.InDelta(t, 3.0, stats.AvgWin, 1e-9)
	assert.InDelta(t, 1.0, stats.AvgLoss, 1e-9)
	assert.InDelta(t, 1.0, stats.WinRate+stats.LossRate, 1e-9)
}

// TestComputeEVNoDecided verifies an all-breakeven (or empty) set yields zero EV
// and zero rates rather than a divide-by-zero.
func TestComputeEVNoDecided(t *testing.T) {
	stats := ComputeEV([]TradeOutcome{{PnL: 0.0}, {PnL: 0.0}})
	assert.Equal(t, 0, stats.Decided)
	assert.Equal(t, 0.0, stats.EV)
	assert.Equal(t, 0.0, stats.WinRate)
	assert.Equal(t, 0.0, stats.LossRate)

	empty := ComputeEV(nil)
	assert.Equal(t, 0, empty.Decided)
	assert.Equal(t, 0.0, empty.EV)
}

// TestComputeGroupsIndependent verifies that trades for one strategy split
// across two regimes produce two independent groups whose EVs do not bleed into
// each other.
func TestComputeGroupsIndependent(t *testing.T) {
	trades := []TradeOutcome{
		// bull regime: EV 2.0 (all winners)
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(1), PnL: 2.0},
		{StrategyID: "s", Regime: "bull", ClosedAt: daysBefore(2), PnL: 2.0},
		// bear regime: EV -1.0 (all losers)
		{StrategyID: "s", Regime: "bear", ClosedAt: daysBefore(1), PnL: -1.0},
		{StrategyID: "s", Regime: "bear", ClosedAt: daysBefore(2), PnL: -1.0},
	}

	results := Compute(trades, asOfRef, Options{})
	require.Len(t, results, 2)

	byRegime := map[string]StrategyEVResult{}
	for _, r := range results {
		byRegime[r.Regime] = r
	}
	assert.InDelta(t, 2.0, byRegime["bull"].EvAllTime, 1e-9)
	assert.InDelta(t, -1.0, byRegime["bear"].EvAllTime, 1e-9)
}
