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
