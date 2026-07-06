package stratopt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// runSyntheticFixture executes one full in-memory generation run over the
// built-in fixtures (gate exercised via its in-memory verdict store fake).
func runSyntheticFixture(t *testing.T) *RunResult {
	t.Helper()
	input, outcomes := SyntheticDataset()
	result, err := GenerateRun(input, outcomes, SyntheticRunConfig(), overfitting.NewFakeVerdictStore())
	require.NoError(t, err)
	return result
}

// findProposal returns the fixture proposal for one (strategy, regime).
func findProposal(t *testing.T, result *RunResult, strategyID, regime string) GeneratedProposal {
	t.Helper()
	for _, p := range result.Proposals {
		if p.Proposal.StrategyID == strategyID && p.Proposal.Regime == regime {
			return p
		}
	}
	t.Fatalf("no proposal for %s/%s", strategyID, regime)
	return GeneratedProposal{}
}

// TestBuildEvidence_FoldGeometryAndWindows: fold test windows never precede
// their train windows, folds are chronologically ordered, and the evidence
// derives only from the candidate's own group rows.
func TestBuildEvidence_FoldGeometryAndWindows(t *testing.T) {
	cfg := DefaultOptimizerConfig()
	rows := churnGroup("s1", "trend", 30, 10, 60)
	groups := ComputeGroupStats(rows, cfg)
	require.Len(t, groups, 1)

	candidates := GenerateCandidates(groups, cfg)
	require.Len(t, candidates, 1)

	evidence, err := BuildEvidence(groups[0], candidates[0], uuid.New(), cfg)
	require.NoError(t, err)

	assert.Equal(t, overfitting.ProposalKindStrategy, evidence.ProposalKind)
	assert.Equal(t, 100, evidence.SampleSize, "sample size is the group's decided count")
	assert.Equal(t, 1, evidence.TrialsCount)
	require.Len(t, evidence.Folds, cfg.FoldCount)

	for i, fold := range evidence.Folds {
		assert.Equal(t, i+1, fold.Index)
		assert.False(t, fold.TestStart.Before(fold.TrainEnd),
			"fold %d: test window must not start before its train window ends", fold.Index)
		assert.False(t, fold.TrainEnd.Before(fold.TrainStart))
		assert.False(t, fold.TestEnd.Before(fold.TestStart))
		if i > 0 {
			assert.False(t, fold.TestStart.Before(evidence.Folds[i-1].TestEnd),
				"fold test windows must be chronologically ordered")
		}
		require.Contains(t, fold.Params, "stop_churn_share",
			"each fold carries the rule's trigger statistic")
	}

	// The in-sample window precedes the out-of-sample window, and both sit
	// inside the group's own row timeline.
	assert.False(t, evidence.OutOfSample.WindowStart.Before(evidence.InSample.WindowEnd))
	assert.Equal(t, groups[0].Outcomes[0].SimulatedAt, evidence.InSample.WindowStart)
	last := groups[0].Outcomes[len(groups[0].Outcomes)-1]
	assert.Equal(t, last.SimulatedAt, evidence.OutOfSample.WindowEnd)
	assert.Equal(t, len(groups[0].Outcomes), evidence.InSample.SampleSize+evidence.OutOfSample.SampleSize)
}

// TestBuildEvidence_MultiplierEncodingWithinGateStepBound: all three default
// rule steps encode as baseline 1.0 / proposed 1.0+adjustment, inside the
// gate's default 25% relative-step bound.
func TestBuildEvidence_MultiplierEncodingWithinGateStepBound(t *testing.T) {
	cfg := DefaultOptimizerConfig()
	gateCfg := cfg.GateConfig
	group := ComputeGroupStats(churnGroup("s1", "trend", 30, 10, 60), cfg)[0]

	steps := []struct {
		parameter  string
		adjustment float64
		rule       string
	}{
		{ParamStopPct, cfg.StopStepPct, RuleStopChurn},
		{ParamMaxHoldDays, cfg.HoldStepPct, RuleTimeoutDrag},
		{ParamTargetPct, cfg.TargetStepPct, RuleFastTarget},
	}

	for _, s := range steps {
		candidate := Candidate{
			StrategyID:    "s1",
			Regime:        "trend",
			Parameter:     s.parameter,
			AdjustmentPct: s.adjustment,
			Rule:          s.rule,
		}
		evidence, err := BuildEvidence(group, candidate, uuid.New(), cfg)
		require.NoError(t, err)

		assert.InDelta(t, 1.0, evidence.BaselineParams[s.parameter], 1e-12)
		assert.InDelta(t, 1.0+s.adjustment, evidence.ProposedParams[s.parameter], 1e-12)

		verdict, err := overfitting.RunGate(evidence, gateCfg)
		require.NoError(t, err)
		for _, c := range verdict.Checks {
			if c.Name != overfitting.CheckNameParamStability {
				continue
			}
			observed := c.Observed["max_relative_step_observed"]
			assert.InDelta(t, absFloat(s.adjustment), observed, 1e-9,
				"the gate's step check must evaluate the adjustment magnitude directly")
			assert.LessOrEqual(t, observed, gateCfg.MaxRelativeStep,
				"every default step sits inside the gate's 25%% bound")
		}
	}
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// TestGenerateRun_SyntheticFixtures walks the full fixture set: the
// gate-passing stop-churn candidate lands pending_review with its verdict id
// set; the gate-failing group lands rejected_by_gate with the retention
// (walk-forward) check named; the below-threshold group generates nothing;
// insufficient-sample groups are reported, not omitted.
func TestGenerateRun_SyntheticFixtures(t *testing.T) {
	result := runSyntheticFixture(t)

	// Gating summary: every synthetic row survives the pipeline.
	assert.Equal(t, result.Gating.InputRows, result.Gating.CleanRows)
	assert.Equal(t, result.Gating.CleanRows, result.Gating.GatedOutcomes)
	assert.Zero(t, result.Gating.TimestampViolations)
	assert.Zero(t, result.Gating.DroppedByRegime)
	assert.Zero(t, result.Gating.DroppedByFidelity)

	// Five groups reported, including both insufficient-data groups.
	require.Len(t, result.Groups, 5)
	byKey := map[string]GroupReport{}
	for _, g := range result.Groups {
		byKey[g.StrategyID+"/"+g.Regime] = g
	}

	assert.True(t, byKey["strat-alpha/trend"].Eligible)
	assert.True(t, byKey["strat-beta/chop"].Eligible)
	assert.True(t, byKey["strat-gamma/trend"].Eligible)
	assert.Equal(t, 60, byKey["strat-gamma/trend"].DecidedCount, "breakevens excluded from decided count")

	assert.False(t, byKey["strat-alpha/range"].Eligible, "20 decided outcomes are insufficient")
	assert.Equal(t, 20, byKey["strat-alpha/range"].DecidedCount)
	assert.False(t, byKey["strat-delta/squeeze"].Eligible, "49 decided outcomes are insufficient despite extreme churn")
	assert.Equal(t, 49, byKey["strat-delta/squeeze"].DecidedCount)

	// Exactly two candidates: alpha/trend (pass) and beta/chop (fail).
	// gamma/trend is eligible but below every rule threshold; the two
	// insufficient groups generate nothing.
	require.Len(t, result.Proposals, 2)

	passing := findProposal(t, result, "strat-alpha", "trend")
	assert.True(t, passing.Verdict.Passed)
	assert.Equal(t, StatusPendingReview, passing.Proposal.Status)
	assert.Equal(t, ParamStopPct, passing.Proposal.Parameter)
	assert.Equal(t, RuleStopChurn, passing.Proposal.Rule)
	assert.Equal(t, passing.VerdictRow.ID, passing.Proposal.VerdictID,
		"the proposal must reference its persisted verdict")
	assert.NotEqual(t, uuid.Nil, passing.Proposal.VerdictID)

	failing := findProposal(t, result, "strat-beta", "chop")
	assert.False(t, failing.Verdict.Passed)
	assert.Equal(t, StatusRejectedByGate, failing.Proposal.Status)
	assert.Equal(t, ParamMaxHoldDays, failing.Proposal.Parameter)
	assert.Equal(t, RuleTimeoutDrag, failing.Proposal.Rule)
	assert.Equal(t, failing.VerdictRow.ID, failing.Proposal.VerdictID)

	walkForwardFailed := false
	for _, c := range failing.Verdict.Checks {
		if c.Name == overfitting.CheckNameWalkForward && !c.Passed {
			walkForwardFailed = true
			assert.Contains(t, c.Detail, "not strictly positive",
				"the out-of-sample collapse must be named")
		}
	}
	assert.True(t, walkForwardFailed, "the failing verdict must name the retention (walk-forward) check")

	// Both proposals share the run id and carry evidence JSON that decodes.
	for _, p := range result.Proposals {
		assert.Equal(t, result.RunID, p.Proposal.RunID)
		evidence, err := UnmarshalProposalEvidence(p.Proposal.EvidenceJSON)
		require.NoError(t, err)
		assert.Equal(t, result.Gating, evidence.GatingSummary)
		assert.Positive(t, evidence.DecidedSamples)
	}
}

// TestGenerateRun_NegativeEdgeGroupIsRejectedByGate: the documented D7
// consequence -- a group whose measured edge is non-positive produces a
// candidate that lands rejected_by_gate (the gate's retention check requires
// a strictly positive in-sample mean), visible in the audit trail rather
// than silently dropped.
func TestGenerateRun_NegativeEdgeGroupIsRejectedByGate(t *testing.T) {
	// Stationary negative-edge stop-churn group: 1 win : 2 quick stop losses.
	b := &syntheticBuilder{}
	for i := 0; i < 120; i++ {
		at := syntheticBase.Add(time.Duration(i) * time.Hour)
		if i%3 == 0 {
			b.add("strat-neg", "trend", "NEGG", at, 0.01, 3, "target")
			continue
		}
		b.add("strat-neg", "trend", "NEGG", at, -0.01, 1, "stop")
	}
	input := optvalidation.Input{Rows: b.rows}

	result, err := GenerateRun(input, b.outcomes, SyntheticRunConfig(), overfitting.NewFakeVerdictStore())
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)

	p := result.Proposals[0]
	assert.Equal(t, RuleStopChurn, p.Proposal.Rule)
	assert.False(t, p.Verdict.Passed)
	assert.Equal(t, StatusRejectedByGate, p.Proposal.Status)

	named := false
	for _, c := range p.Verdict.Checks {
		if c.Name == overfitting.CheckNameWalkForward && !c.Passed {
			named = true
			assert.Contains(t, c.Detail, "in-sample mean")
		}
	}
	assert.True(t, named, "the retention check must name the non-positive in-sample mean")
}

// TestGenerateRun_DeterministicEvidenceAndCandidates: two runs over the same
// fixture produce identical gating summaries, group reports, candidates, and
// gate evidence (modulo the generated proposal and run ids).
func TestGenerateRun_DeterministicEvidenceAndCandidates(t *testing.T) {
	input, outcomes := SyntheticDataset()

	first, err := GenerateRun(input, outcomes, SyntheticRunConfig(), overfitting.NewFakeVerdictStore())
	require.NoError(t, err)
	second, err := GenerateRun(input, outcomes, SyntheticRunConfig(), overfitting.NewFakeVerdictStore())
	require.NoError(t, err)

	assert.Equal(t, first.Gating, second.Gating)
	assert.Equal(t, first.Groups, second.Groups)
	require.Len(t, second.Proposals, len(first.Proposals))

	for i := range first.Proposals {
		a, b := first.Proposals[i], second.Proposals[i]
		assert.Equal(t, a.Candidate, b.Candidate)
		assert.Equal(t, a.Verdict.Passed, b.Verdict.Passed)
		assert.Equal(t, a.Verdict.Checks, b.Verdict.Checks)
		assert.Equal(t, a.Proposal.Status, b.Proposal.Status)
		assert.JSONEq(t, string(a.Proposal.EvidenceJSON), string(b.Proposal.EvidenceJSON))

		// Evidence equality modulo the generated proposal id.
		ea, eb := a.Evidence, b.Evidence
		ea.ProposalID = uuid.Nil
		eb.ProposalID = uuid.Nil
		assert.Equal(t, ea, eb)
	}
}
