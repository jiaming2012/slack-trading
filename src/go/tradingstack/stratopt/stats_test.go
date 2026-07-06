package stratopt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statsBase anchors the test timeline; each helper call advances one hour so
// chronological ordering is unambiguous.
var statsBase = time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)

// gated builds one GatedOutcome for stats/rules tests.
func gated(strategyID, regime string, seq int, pnl float64, holdDays int, exitReason string, weight float64) GatedOutcome {
	return GatedOutcome{
		SimOutcomeID: uuid.New(),
		StrategyID:   strategyID,
		Regime:       regime,
		SimulatedAt:  statsBase.Add(time.Duration(seq) * time.Hour),
		PnlPct:       pnl,
		HoldDays:     holdDays,
		ExitReason:   exitReason,
		EVWeight:     weight,
	}
}

// evFixture is the spec's hand-computed weighted-EV fixture: six winning
// outcomes averaging +2.0 and four losing outcomes averaging -1.0.
func evFixture(lossWeight float64) []GatedOutcome {
	rows := make([]GatedOutcome, 0, 10)
	for i := 0; i < 6; i++ {
		rows = append(rows, gated("s1", "trend", i, 2.0, 3, "target", 1.0))
	}
	for i := 0; i < 4; i++ {
		rows = append(rows, gated("s1", "trend", 6+i, -1.0, 1, "stop", lossWeight))
	}
	return rows
}

// TestWeightedEV_HandComputedFixture: uniform weight 1.0, six wins averaging
// +2.0, four losses averaging -1.0 -> EV = 0.6*2.0 - 0.4*1.0 = 0.8.
func TestWeightedEV_HandComputedFixture(t *testing.T) {
	groups := ComputeGroupStats(evFixture(1.0), DefaultOptimizerConfig())
	require.Len(t, groups, 1)

	g := groups[0]
	assert.Equal(t, "s1", g.StrategyID)
	assert.Equal(t, "trend", g.Regime)
	assert.Equal(t, 10, g.DecidedCount)
	assert.InDelta(t, 0.6, g.WinRate, 1e-12)
	assert.InDelta(t, 0.4, g.LossRate, 1e-12)
	assert.InDelta(t, 2.0, g.AvgWinPct, 1e-12)
	assert.InDelta(t, 1.0, g.AvgLossPct, 1e-12, "avg loss must be a positive magnitude")
	assert.InDelta(t, 0.8, g.WeightedEV, 1e-12)
}

// TestWeightedEV_DownweightedLossesMoveInHandComputedDirection: the same
// fixture with every loss weighted 0.5 -> win weight 6, loss weight 2 ->
// loss rate 0.25 (down from 0.4) and EV = 0.75*2.0 - 0.25*1.0 = 1.25 (up
// from 0.8): losses count for less.
func TestWeightedEV_DownweightedLossesMoveInHandComputedDirection(t *testing.T) {
	groups := ComputeGroupStats(evFixture(0.5), DefaultOptimizerConfig())
	require.Len(t, groups, 1)

	g := groups[0]
	assert.InDelta(t, 0.25, g.LossRate, 1e-12)
	assert.InDelta(t, 0.75, g.WinRate, 1e-12)
	assert.InDelta(t, 1.25, g.WeightedEV, 1e-12)
	assert.Greater(t, g.WeightedEV, 0.8, "downweighting losses must raise the weighted EV")
	assert.Less(t, g.LossRate, 0.4, "downweighting losses must lower the weighted loss rate")
}

// TestGroupStats_GroupsAreIndependent: outcomes for one strategy spanning two
// regimes produce two independent groups -- no outcome from one regime
// affects the other's statistics.
func TestGroupStats_GroupsAreIndependent(t *testing.T) {
	rows := evFixture(1.0) // s1/trend: EV 0.8
	// s1/chop: pure losses. If these leaked into s1/trend its EV would move.
	for i := 0; i < 5; i++ {
		rows = append(rows, gated("s1", "chop", 20+i, -3.0, 9, "timeout", 1.0))
	}

	groups := ComputeGroupStats(rows, DefaultOptimizerConfig())
	require.Len(t, groups, 2)

	// Sorted by (strategy_id, regime): chop before trend.
	assert.Equal(t, "chop", groups[0].Regime)
	assert.Equal(t, 5, groups[0].DecidedCount)
	assert.InDelta(t, -3.0, groups[0].WeightedEV, 1e-12)
	assert.InDelta(t, 1.0, groups[0].TimeoutShareOfLosses, 1e-12)

	assert.Equal(t, "trend", groups[1].Regime)
	assert.Equal(t, 10, groups[1].DecidedCount)
	assert.InDelta(t, 0.8, groups[1].WeightedEV, 1e-12, "the trend group's EV must be untouched by the chop group")
}

// TestGroupStats_BreakevensExcluded: PnlPct == 0 outcomes count toward
// neither wins nor losses nor the decided-sample count, and leave the
// weighted EV unchanged.
func TestGroupStats_BreakevensExcluded(t *testing.T) {
	rows := evFixture(1.0)
	for i := 0; i < 7; i++ {
		rows = append(rows, gated("s1", "trend", 30+i, 0, 2, "signal_exit", 1.0))
	}

	groups := ComputeGroupStats(rows, DefaultOptimizerConfig())
	require.Len(t, groups, 1)

	g := groups[0]
	assert.Equal(t, 10, g.DecidedCount, "breakevens must not count as decided")
	assert.Len(t, g.Outcomes, 17, "breakevens remain part of the group's chronological row set")
	assert.InDelta(t, 0.8, g.WeightedEV, 1e-12)
	assert.InDelta(t, 0.6, g.WinRate, 1e-12)
	assert.InDelta(t, 0.4, g.LossRate, 1e-12)
}

// TestGroupStats_ExitReasonShares: the three rule trigger shares apply their
// hold-day cutoffs and ev-weights.
func TestGroupStats_ExitReasonShares(t *testing.T) {
	cfg := DefaultOptimizerConfig() // stop cutoff 2 days, target cutoff 1 day
	rows := []GatedOutcome{
		// Losses: quick stop (counts), slow stop (does not), timeout.
		gated("s1", "trend", 0, -1.0, 1, "stop", 1.0),
		gated("s1", "trend", 1, -1.0, 5, "stop", 1.0),
		gated("s1", "trend", 2, -1.0, 9, "timeout", 2.0),
		// Wins: fast target (counts), slow target (does not).
		gated("s1", "trend", 3, 2.0, 1, "target", 3.0),
		gated("s1", "trend", 4, 2.0, 4, "target", 1.0),
	}

	groups := ComputeGroupStats(rows, cfg)
	require.Len(t, groups, 1)
	g := groups[0]

	// Loss weight total 4.0: quick stop 1.0 -> 0.25; timeout 2.0 -> 0.5.
	assert.InDelta(t, 0.25, g.QuickStopShareOfLosses, 1e-12)
	assert.InDelta(t, 0.5, g.TimeoutShareOfLosses, 1e-12)
	// Win weight total 4.0: fast target 3.0 -> 0.75.
	assert.InDelta(t, 0.75, g.FastTargetShareOfWins, 1e-12)
}
