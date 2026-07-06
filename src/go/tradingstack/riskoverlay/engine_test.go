package riskoverlay

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// permissiveLimits returns limits that no order can breach, so a single limit
// under test can be tightened in isolation.
func permissiveLimits() RiskLimits {
	return RiskLimits{
		MaxGrossExposure:          1e15,
		MaxNetExposure:            1e15,
		MaxSectorConcentrationPct: 100,
		MaxDrawdownPct:            100,
		DeployableCapital:         1e15,
		RejectCrowdedEntries:      false,
	}
}

func emptyView() CrowdingView { return NewCrowdingView(false, nil) }

// 6.1 — a clean order under every limit is allowed.
func TestEvaluate_CleanOrderAllowed(t *testing.T) {
	state := PortfolioState{
		Positions: []LogicalPosition{
			{Ticker: "AAPL", Sector: "technology", SignedNotional: 20_000},
		},
	}
	order := ProposedOrder{Ticker: "KO", Sector: "staples", StrategyID: "s1", SignedNotional: 5_000}

	dec, err := Evaluate(state, order, permissiveLimits(), emptyView())
	require.NoError(t, err)
	require.True(t, dec.Allowed)
	require.Empty(t, dec.Breaches)
}

// 6.2 — gross exposure over cap rejected; exactly at cap allowed.
func TestEvaluate_GrossExposure(t *testing.T) {
	state := PortfolioState{Positions: []LogicalPosition{{Ticker: "AAPL", SignedNotional: 90_000}}}
	limits := permissiveLimits()
	limits.MaxGrossExposure = 100_000

	t.Run("over cap rejected", func(t *testing.T) {
		order := ProposedOrder{Ticker: "MSFT", SignedNotional: 15_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitGrossExposure))
	})

	t.Run("exactly at cap allowed", func(t *testing.T) {
		order := ProposedOrder{Ticker: "MSFT", SignedNotional: 10_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitGrossExposure))
		require.True(t, dec.Allowed)
	})
}

// 6.3 — net exposure over cap rejected while gross within cap.
func TestEvaluate_NetExposureOverCap(t *testing.T) {
	state := PortfolioState{Positions: []LogicalPosition{{Ticker: "AAPL", SignedNotional: 80_000}}}
	limits := permissiveLimits()
	limits.MaxNetExposure = 90_000
	limits.MaxGrossExposure = 1_000_000

	order := ProposedOrder{Ticker: "MSFT", SignedNotional: 20_000} // long, pushes net to +100k
	dec, err := Evaluate(state, order, limits, emptyView())
	require.NoError(t, err)
	require.False(t, dec.Allowed)
	require.True(t, dec.HasBreach(LimitNetExposure))
	require.False(t, dec.HasBreach(LimitGrossExposure))
}

// Net exposure counts a short entry too: shorts drive |net| up in the negative
// direction.
func TestEvaluate_NetExposureShortSide(t *testing.T) {
	state := PortfolioState{Positions: []LogicalPosition{{Ticker: "AAPL", SignedNotional: -80_000}}}
	limits := permissiveLimits()
	limits.MaxNetExposure = 90_000
	limits.MaxGrossExposure = 1_000_000

	order := ProposedOrder{Ticker: "MSFT", SignedNotional: -20_000} // short, net to -100k
	dec, err := Evaluate(state, order, limits, emptyView())
	require.NoError(t, err)
	require.True(t, dec.HasBreach(LimitNetExposure))
}

// 6.4 — sector concentration over limit rejected (names sector); under allowed.
func TestEvaluate_SectorConcentration(t *testing.T) {
	// technology already 40k of 100k gross; add 30k technology => 70k of 130k ≈ 53.8%.
	state := PortfolioState{Positions: []LogicalPosition{
		{Ticker: "AAPL", Sector: "technology", SignedNotional: 40_000},
		{Ticker: "KO", Sector: "staples", SignedNotional: 60_000},
	}}
	order := ProposedOrder{Ticker: "MSFT", Sector: "technology", SignedNotional: 30_000}

	t.Run("over limit rejected and names sector", func(t *testing.T) {
		limits := permissiveLimits()
		limits.MaxSectorConcentrationPct = 50.0
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitSectorConcentration))
		found := false
		for _, b := range dec.Breaches {
			if b.Type == LimitSectorConcentration {
				require.Contains(t, b.Reason, "technology")
				found = true
			}
		}
		require.True(t, found)
	})

	t.Run("under limit allowed", func(t *testing.T) {
		limits := permissiveLimits()
		limits.MaxSectorConcentrationPct = 60.0
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitSectorConcentration))
	})
}

// Sector concentration exactly at the limit is allowed (strict > boundary).
func TestEvaluate_SectorConcentrationExactlyAtLimit(t *testing.T) {
	// technology 50k of 100k total after order => exactly 50%.
	state := PortfolioState{Positions: []LogicalPosition{
		{Ticker: "AAPL", Sector: "technology", SignedNotional: 30_000},
		{Ticker: "KO", Sector: "staples", SignedNotional: 50_000},
	}}
	order := ProposedOrder{Ticker: "MSFT", Sector: "technology", SignedNotional: 20_000}
	limits := permissiveLimits()
	limits.MaxSectorConcentrationPct = 50.0

	dec, err := Evaluate(state, order, limits, emptyView())
	require.NoError(t, err)
	require.False(t, dec.HasBreach(LimitSectorConcentration))
}

// 6.5 — drawdown breaker: 8% drawdown entry rejected; exactly at limit not tripped.
func TestEvaluate_DrawdownBreaker(t *testing.T) {
	limits := permissiveLimits()
	limits.MaxDrawdownPct = 5.0

	t.Run("8pct drawdown halts entry", func(t *testing.T) {
		state := PortfolioState{EquitySeries: []float64{100_000, 98_000, 95_000, 93_000, 92_000}} // peak 100k, current 92k => 8%
		order := ProposedOrder{Ticker: "AAPL", SignedNotional: 1_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitDrawdownBreaker))
	})

	t.Run("exactly at limit does not trip", func(t *testing.T) {
		state := PortfolioState{EquitySeries: []float64{100_000, 99_000, 97_000, 96_000, 95_000}} // 5% exactly
		order := ProposedOrder{Ticker: "AAPL", SignedNotional: 1_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitDrawdownBreaker))
	})
}

// wire-risk-overlay-state nit d — the drawdown breaker fails SAFE on a
// wiped-out book: a non-empty series whose 5-session peak is <= 0 trips the
// breaker categorically instead of silently disabling it; an empty series
// stays inactive; negative values under a positive peak read as ordinary deep
// drawdown, not a categorical trip.
func TestEvaluate_DrawdownFailSafe(t *testing.T) {
	limits := permissiveLimits()
	limits.MaxDrawdownPct = 5.0
	entry := ProposedOrder{Ticker: "AAPL", SignedNotional: 1_000}

	t.Run("all-non-positive equity trips the breaker", func(t *testing.T) {
		state := PortfolioState{EquitySeries: []float64{0, -5_000, -12_000}}
		dec, err := Evaluate(state, entry, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitDrawdownBreaker))
		var reason string
		for _, b := range dec.Breaches {
			if b.Type == LimitDrawdownBreaker {
				reason = b.Reason
			}
		}
		require.Contains(t, reason, "non-positive", "breach reason must name the non-positive peak")
		require.Contains(t, reason, "0.00", "breach reason must carry the peak value")
	})

	t.Run("a stale positive peak outside the window does not mask a wiped-out trailing window", func(t *testing.T) {
		// 6 values: the positive peak is OUTSIDE the trailing-5 window; every
		// value inside the window is <= 0, so the fail-safe trips.
		state := PortfolioState{EquitySeries: []float64{100_000, 0, -1, -2, -3, -4}}
		dec, err := Evaluate(state, entry, limits, emptyView())
		require.NoError(t, err)
		require.True(t, dec.HasBreach(LimitDrawdownBreaker))
	})

	t.Run("negative values with a positive peak read as ordinary deep drawdown", func(t *testing.T) {
		state := PortfolioState{EquitySeries: []float64{100_000, 50_000, -10_000}} // dd = 110%
		dec, err := Evaluate(state, entry, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitDrawdownBreaker))
		var reason string
		for _, b := range dec.Breaches {
			if b.Type == LimitDrawdownBreaker {
				reason = b.Reason
			}
		}
		require.Contains(t, reason, "exceeds max", "an ordinary drawdown breach, not the categorical fail-safe")
		require.NotContains(t, reason, "non-positive")
	})

	t.Run("empty series leaves the breaker inactive", func(t *testing.T) {
		state := PortfolioState{EquitySeries: nil}
		dec, err := Evaluate(state, entry, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitDrawdownBreaker))
		require.True(t, dec.Allowed)
	})

	t.Run("reduction passes even with a wiped-out book", func(t *testing.T) {
		state := PortfolioState{EquitySeries: []float64{-1, -2, -3}}
		reduction := ProposedOrder{Ticker: "AAPL", SignedNotional: -1_000, IsReduction: true}
		dec, err := Evaluate(state, reduction, limits, emptyView())
		require.NoError(t, err)
		require.True(t, dec.Allowed)
		require.Empty(t, dec.Breaches)
	})
}

// wire-risk-overlay-state nit a — an EMPTY EV-weight set pins the
// strategy-allocation family INACTIVE: no strategy_allocation breach for any
// entry (contrast: a strategy absent from a NON-empty set is capped at zero).
func TestEvaluate_EmptyEvSetPinsAllocationFamilyInactive(t *testing.T) {
	limits := permissiveLimits()
	limits.DeployableCapital = 100_000

	for name, ev := range map[string]map[string]float64{"nil map": nil, "empty map": {}} {
		t.Run(name, func(t *testing.T) {
			state := PortfolioState{EvWeights: ev, StrategyDeployed: map[string]float64{"anything": 1e9}}
			order := ProposedOrder{Ticker: "AAPL", StrategyID: "anything", SignedNotional: 1e9}
			dec, err := Evaluate(state, order, limits, emptyView())
			require.NoError(t, err)
			require.False(t, dec.HasBreach(LimitStrategyAllocation))
			require.True(t, dec.Allowed)
		})
	}
}

// N2 — the drawdown window defensively truncates to the most recent 5 sessions
// rather than trusting the supplier. A stale, deeper peak outside the trailing
// window must not trip the breaker.
func TestEvaluate_DrawdownTruncatesToFiveSessions(t *testing.T) {
	limits := permissiveLimits()
	limits.MaxDrawdownPct = 5.0

	// 6 sessions: a 200k peak sits OUTSIDE the trailing-5 window; within the last
	// 5 the series is flat at 100k => 0% drawdown. Untruncated it would be 50%.
	state := PortfolioState{EquitySeries: []float64{200_000, 100_000, 100_000, 100_000, 100_000, 100_000}}
	order := ProposedOrder{Ticker: "AAPL", SignedNotional: 1_000}
	dec, err := Evaluate(state, order, limits, emptyView())
	require.NoError(t, err)
	require.False(t, dec.HasBreach(LimitDrawdownBreaker), "stale peak outside the 5-session window must not trip the breaker")
	require.True(t, dec.Allowed)
}

// 6.6 — reduction orders always pass, even while the breaker is tripped.
func TestEvaluate_ReductionAlwaysPasses(t *testing.T) {
	limits := permissiveLimits()
	limits.MaxDrawdownPct = 5.0
	limits.MaxGrossExposure = 1.0 // impossibly tight
	limits.RejectCrowdedEntries = true

	state := PortfolioState{
		Positions:    []LogicalPosition{{Ticker: "AAPL", Sector: "technology", SignedNotional: 500_000}},
		EquitySeries: []float64{100_000, 80_000, 70_000, 60_000, 50_000}, // 50% drawdown, breaker tripped
	}
	// Reduction of the existing long, into a crowded ticker, huge notional — all
	// of which would breach if this were an entry.
	order := ProposedOrder{Ticker: "AAPL", Sector: "technology", SignedNotional: -400_000, IsReduction: true}

	view := NewCrowdingView(true, []string{"AAPL"})
	dec, err := Evaluate(state, order, limits, view)
	require.NoError(t, err)
	require.True(t, dec.Allowed)
	require.Empty(t, dec.Breaches)
}

// 6.7 — per-strategy allocation: higher-EV passes, lower-EV capped, absent rejected.
func TestEvaluate_StrategyAllocation(t *testing.T) {
	limits := permissiveLimits()
	limits.DeployableCapital = 100_000

	evWeights := map[string]float64{"A": 0.75, "B": 0.25}

	t.Run("higher-EV strategy passes", func(t *testing.T) {
		state := PortfolioState{EvWeights: evWeights, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 40_000} // cap 75k
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitStrategyAllocation))
	})

	t.Run("lower-EV strategy capped", func(t *testing.T) {
		state := PortfolioState{EvWeights: evWeights, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "B", SignedNotional: 40_000} // cap 25k
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitStrategyAllocation))
	})

	t.Run("strategy absent from EV set is capped at zero", func(t *testing.T) {
		state := PortfolioState{EvWeights: evWeights, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "C", SignedNotional: 1_000} // cap 0
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitStrategyAllocation))
	})

	t.Run("exactly at cap allowed", func(t *testing.T) {
		state := PortfolioState{EvWeights: evWeights, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 75_000} // cap exactly 75k
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitStrategyAllocation))
	})

	t.Run("no EV data participates skips the check", func(t *testing.T) {
		state := PortfolioState{EvWeights: map[string]float64{}, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "anything", SignedNotional: 1e9}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitStrategyAllocation))
	})

	// B2 — a non-empty EV-weight set that sums to <= 0 gives every listed
	// strategy a zero cap; a listed strategy's entry is rejected (the reviewer's
	// $50M probe). Contrast with the empty-map case above, which stays inactive.
	t.Run("all-zero weights reject a listed strategy entry", func(t *testing.T) {
		state := PortfolioState{EvWeights: map[string]float64{"A": 0, "B": 0}, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 50_000_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitStrategyAllocation))
	})

	// B2 — a strategy with a negative individual weight is treated as zero cap
	// and rejected, even alongside a positive strategy.
	t.Run("negative-weight strategy treated as zero cap", func(t *testing.T) {
		state := PortfolioState{EvWeights: map[string]float64{"A": -1.0, "B": 2.0}, StrategyDeployed: map[string]float64{}}
		order := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 1_000}
		dec, err := Evaluate(state, order, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitStrategyAllocation))
	})

	// B2 — a mixed {2.0, -1.0} set no longer inflates the positive strategy's
	// cap. With DeployableCapital=100k the cap for A is 100k*2/2 = 100k (clamped
	// sum), not the old 100k*2/(2-1) = 200k. A 150k entry that the old math would
	// have allowed is now rejected.
	t.Run("mixed positive/negative weights do not inflate the positive cap", func(t *testing.T) {
		state := PortfolioState{EvWeights: map[string]float64{"A": 2.0, "B": -1.0}, StrategyDeployed: map[string]float64{}}

		// 150k exceeds the correct 100k cap -> rejected (old inflated 200k cap would allow).
		over := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 150_000}
		dec, err := Evaluate(state, over, limits, emptyView())
		require.NoError(t, err)
		require.True(t, dec.HasBreach(LimitStrategyAllocation))

		// Exactly at the correct 100k cap is allowed (proves cap == 100k, not 200k or 0).
		atCap := ProposedOrder{Ticker: "AAPL", StrategyID: "A", SignedNotional: 100_000}
		dec2, err := Evaluate(state, atCap, limits, emptyView())
		require.NoError(t, err)
		require.False(t, dec2.HasBreach(LimitStrategyAllocation))
	})
}

// 6.8 — crowding: flagged ticker rejected; non-flagged allowed; disabled off.
func TestEvaluate_Crowding(t *testing.T) {
	view := NewCrowdingView(true, []string{"NVDA"})

	t.Run("entry into flagged ticker rejected", func(t *testing.T) {
		limits := permissiveLimits()
		limits.RejectCrowdedEntries = true
		order := ProposedOrder{Ticker: "NVDA", SignedNotional: 1_000}
		dec, err := Evaluate(PortfolioState{}, order, limits, view)
		require.NoError(t, err)
		require.False(t, dec.Allowed)
		require.True(t, dec.HasBreach(LimitCrowding))
	})

	t.Run("non-flagged ticker allowed under flagged cycle", func(t *testing.T) {
		limits := permissiveLimits()
		limits.RejectCrowdedEntries = true
		order := ProposedOrder{Ticker: "KO", SignedNotional: 1_000}
		dec, err := Evaluate(PortfolioState{}, order, limits, view)
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitCrowding))
	})

	t.Run("RejectCrowdedEntries false disables rejection", func(t *testing.T) {
		limits := permissiveLimits()
		limits.RejectCrowdedEntries = false
		order := ProposedOrder{Ticker: "NVDA", SignedNotional: 1_000}
		dec, err := Evaluate(PortfolioState{}, order, limits, view)
		require.NoError(t, err)
		require.False(t, dec.HasBreach(LimitCrowding))
	})
}

// 6.9 — multi-breach: order breaching gross exposure and strategy allocation reports both.
func TestEvaluate_MultiBreach(t *testing.T) {
	state := PortfolioState{
		Positions:        []LogicalPosition{{Ticker: "AAPL", SignedNotional: 95_000}},
		EvWeights:        map[string]float64{"A": 0.5, "B": 0.5},
		StrategyDeployed: map[string]float64{},
	}
	limits := permissiveLimits()
	limits.MaxGrossExposure = 100_000
	limits.DeployableCapital = 100_000 // cap for B = 50k

	order := ProposedOrder{Ticker: "MSFT", StrategyID: "B", SignedNotional: 60_000} // gross->155k, alloc->60k>50k
	dec, err := Evaluate(state, order, limits, emptyView())
	require.NoError(t, err)
	require.False(t, dec.Allowed)
	require.True(t, dec.HasBreach(LimitGrossExposure))
	require.True(t, dec.HasBreach(LimitStrategyAllocation))
	require.Len(t, dec.Breaches, 2)
}
