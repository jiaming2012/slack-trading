package stratopt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// churnGroup builds an eligible group whose losses are quickStops quick
// stop-outs and slowLosses slow signal exits, with wins topping the decided
// count up to total.
func churnGroup(strategyID, regime string, quickStops, slowLosses, wins int) []GatedOutcome {
	rows := make([]GatedOutcome, 0, quickStops+slowLosses+wins)
	seq := 0
	for i := 0; i < quickStops; i++ {
		rows = append(rows, gated(strategyID, regime, seq, -1.0, 1, "stop", 1.0))
		seq++
	}
	for i := 0; i < slowLosses; i++ {
		rows = append(rows, gated(strategyID, regime, seq, -1.0, 6, "signal_exit", 1.0))
		seq++
	}
	for i := 0; i < wins; i++ {
		rows = append(rows, gated(strategyID, regime, seq, 2.0, 4, "target", 1.0))
		seq++
	}
	return rows
}

// TestGenerateCandidates_StopChurnFixture: an eligible group whose losses are
// 70% (weighted) quick stop-outs yields exactly one stop-churn proposal:
// stop_pct +20% relative, with the 0.7 share, the group's weighted EV, and
// the decided-sample count carried as evidence.
func TestGenerateCandidates_StopChurnFixture(t *testing.T) {
	cfg := DefaultOptimizerConfig()
	// 14 quick stops + 6 slow losses = 20 losses (70% quick), 40 wins.
	rows := churnGroup("s1", "trend", 14, 6, 40)

	groups := ComputeGroupStats(rows, cfg)
	require.Len(t, groups, 1)
	require.Equal(t, 60, groups[0].DecidedCount)
	require.InDelta(t, 0.7, groups[0].QuickStopShareOfLosses, 1e-12)

	candidates := GenerateCandidates(groups, cfg)
	require.Len(t, candidates, 1, "exactly one stop-churn proposal for the group")

	c := candidates[0]
	assert.Equal(t, "s1", c.StrategyID)
	assert.Equal(t, "trend", c.Regime)
	assert.Equal(t, ParamStopPct, c.Parameter)
	assert.InDelta(t, 0.20, c.AdjustmentPct, 1e-12)
	assert.Equal(t, RuleStopChurn, c.Rule)
	assert.InDelta(t, 0.7, c.TriggerShare, 1e-12, "the evidence carries the 0.7 weighted share")
	assert.NotEmpty(t, c.Rationale)

	// The group's stats -- weighted EV and decided-sample count -- travel
	// with the candidate into the proposal evidence.
	evidence := buildProposalEvidence(groups[0], c, GatingSummary{})
	assert.InDelta(t, groups[0].WeightedEV, evidence.WeightedEV, 1e-12)
	assert.Equal(t, 60, evidence.DecidedSamples)
	assert.InDelta(t, 0.7, evidence.TriggerShare, 1e-12)
}

// TestGenerateCandidates_BelowThresholdGeneratesNothing: a quick-stop-out
// share of 0.4 (and no other rule condition met) yields zero proposals.
func TestGenerateCandidates_BelowThresholdGeneratesNothing(t *testing.T) {
	cfg := DefaultOptimizerConfig()
	// 8 quick stops + 12 slow losses = 20 losses (40% quick), 40 wins.
	rows := churnGroup("s1", "trend", 8, 12, 40)

	groups := ComputeGroupStats(rows, cfg)
	require.Len(t, groups, 1)
	require.InDelta(t, 0.4, groups[0].QuickStopShareOfLosses, 1e-12)

	candidates := GenerateCandidates(groups, cfg)
	assert.Empty(t, candidates)
}

// TestGenerateCandidates_MinimumSamplePreFilterBoundary: a group with 49
// decided outcomes exhibiting extreme stop-churn generates nothing; the same
// shape with exactly 50 decided outcomes is evaluated and generates the
// candidate.
func TestGenerateCandidates_MinimumSamplePreFilterBoundary(t *testing.T) {
	cfg := DefaultOptimizerConfig()

	// 49 decided: 30 quick stops (100% of losses), 19 wins.
	below := ComputeGroupStats(churnGroup("s1", "squeeze", 30, 0, 19), cfg)
	require.Len(t, below, 1)
	require.Equal(t, 49, below[0].DecidedCount)
	require.InDelta(t, 1.0, below[0].QuickStopShareOfLosses, 1e-12)
	assert.Empty(t, GenerateCandidates(below, cfg),
		"49 decided outcomes must generate no candidates despite extreme stop-churn")

	// 50 decided: same shape, one more win.
	at := ComputeGroupStats(churnGroup("s1", "squeeze", 30, 0, 20), cfg)
	require.Len(t, at, 1)
	require.Equal(t, 50, at[0].DecidedCount)
	candidates := GenerateCandidates(at, cfg)
	require.Len(t, candidates, 1, "a group at exactly the minimum is eligible")
	assert.Equal(t, RuleStopChurn, candidates[0].Rule)
}

// TestGenerateCandidates_RepeatedRunsAreIdentical: identical gated input and
// configuration produce identical candidates in identical order, including a
// multi-group, multi-rule set.
func TestGenerateCandidates_RepeatedRunsAreIdentical(t *testing.T) {
	cfg := DefaultOptimizerConfig()

	rows := churnGroup("s2", "trend", 40, 0, 20)
	// Second group triggering timeout-drag AND fast-target.
	for i := 0; i < 30; i++ {
		rows = append(rows, gated("s1", "chop", 100+i, -1.0, 9, "timeout", 1.0))
	}
	for i := 0; i < 30; i++ {
		rows = append(rows, gated("s1", "chop", 200+i, 2.0, 1, "target", 1.0))
	}

	first := GenerateCandidates(ComputeGroupStats(rows, cfg), cfg)
	second := GenerateCandidates(ComputeGroupStats(rows, cfg), cfg)

	require.Len(t, first, 3)
	assert.Equal(t, first, second, "repeated runs on identical input must be identical")

	// Sorted by (strategy_id, regime, parameter).
	assert.Equal(t, []string{"s1", "s1", "s2"},
		[]string{first[0].StrategyID, first[1].StrategyID, first[2].StrategyID})
	assert.Equal(t, ParamMaxHoldDays, first[0].Parameter)
	assert.Equal(t, ParamTargetPct, first[1].Parameter)
	assert.Equal(t, ParamStopPct, first[2].Parameter)
}
