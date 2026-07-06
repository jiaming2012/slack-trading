package stratopt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// RunConfig parameterizes one generation run. All clock values are
// caller-supplied so a run is a deterministic function of its inputs (modulo
// generated proposal ids).
type RunConfig struct {
	// Now stamps each verdict's computed_at and each proposal's created_at.
	Now time.Time

	// RunID groups the run's proposals; a zero value gets a fresh UUID.
	RunID uuid.UUID

	// Optimizer carries the pre-filter, rule, fold, and gate thresholds.
	Optimizer OptimizerConfig

	// PipelineConfig carries the optimizer-validation-pipeline thresholds.
	PipelineConfig optvalidation.Config
}

// GroupReport is one (strategy_id, regime) group's run outcome: its weighted
// statistics and whether it cleared the minimum-sample pre-filter. Groups
// below the pre-filter are reported here as insufficient-data -- never
// silently omitted.
type GroupReport struct {
	StrategyID   string
	Regime       string
	DecidedCount int
	Eligible     bool
	Stats        GroupStats
}

// GeneratedProposal is one candidate carried through the full pipeline: the
// rule output, the gate evidence built from the candidate's own gated rows,
// the gate's verdict, the persisted verdict row, and the proposal row whose
// status the verdict dictated.
type GeneratedProposal struct {
	Candidate  Candidate
	Evidence   overfitting.Evidence
	Verdict    overfitting.Verdict
	VerdictRow overfitting.OverfittingVerdict
	Proposal   StrategyProposal
}

// RunResult is everything one generation run produced. Zero proposals, and
// every proposal rejected by the gate, are both normal run outcomes.
type RunResult struct {
	RunID     uuid.UUID
	Gating    GatingSummary
	Groups    []GroupReport
	Proposals []GeneratedProposal
}

// ProposalEvidence is the JSON shape of a proposal row's evidence_json
// column: the group's weighted statistics, the rule's trigger share, the
// decided-sample count, and the pipeline gating summary.
type ProposalEvidence struct {
	WeightedEV             float64       `json:"weighted_ev"`
	WeightedWinRate        float64       `json:"weighted_win_rate"`
	WeightedLossRate       float64       `json:"weighted_loss_rate"`
	AvgWinPct              float64       `json:"avg_win_pct"`
	AvgLossPct             float64       `json:"avg_loss_pct"`
	QuickStopShareOfLosses float64       `json:"quick_stop_share_of_losses"`
	TimeoutShareOfLosses   float64       `json:"timeout_share_of_losses"`
	FastTargetShareOfWins  float64       `json:"fast_target_share_of_wins"`
	TriggerShare           float64       `json:"trigger_share"`
	DecidedSamples         int           `json:"decided_samples"`
	GatingSummary          GatingSummary `json:"gating_summary"`
}

// UnmarshalProposalEvidence decodes a proposal row's evidence_json (used by
// the operator inspect command).
func UnmarshalProposalEvidence(data []byte) (ProposalEvidence, error) {
	var e ProposalEvidence
	if err := json.Unmarshal(data, &e); err != nil {
		return ProposalEvidence{}, fmt.Errorf("stratopt: evidence unmarshal failed: %w", err)
	}
	return e, nil
}

// buildProposalEvidence assembles the evidence record for one candidate.
func buildProposalEvidence(group GroupStats, candidate Candidate, gating GatingSummary) ProposalEvidence {
	return ProposalEvidence{
		WeightedEV:             group.WeightedEV,
		WeightedWinRate:        group.WinRate,
		WeightedLossRate:       group.LossRate,
		AvgWinPct:              group.AvgWinPct,
		AvgLossPct:             group.AvgLossPct,
		QuickStopShareOfLosses: group.QuickStopShareOfLosses,
		TimeoutShareOfLosses:   group.TimeoutShareOfLosses,
		FastTargetShareOfWins:  group.FastTargetShareOfWins,
		TriggerShare:           candidate.TriggerShare,
		DecidedSamples:         group.DecidedCount,
		GatingSummary:          gating,
	}
}

// GenerateRun executes one full generation run over the candidate rows and
// their sim outcomes:
//
//  1. Thread the input through the optimizer-validation-pipeline; a pipeline
//     error aborts the run with zero proposals (wrapped ErrPipelineFailed).
//  2. Join the clean weighted rows back to their sim outcomes by
//     SimOutcomeID -- outcomes dropped by any stage influence nothing.
//  3. Compute ev-weight-weighted per-(strategy_id, regime) statistics and
//     apply the minimum-sample pre-filter (insufficient-data groups are
//     reported, not omitted).
//  4. Apply the three deterministic rules to eligible groups.
//  5. For every candidate: build gate evidence from the candidate's own
//     gated rows, run the shared overfitting gate with the strategy-kind
//     config, persist the verdict -- pass or fail -- through the supplied
//     VerdictStore, and record the proposal with the status the verdict
//     dictates (pending_review on pass, rejected_by_gate on fail).
//
// GenerateRun persists NO proposal rows itself -- the caller decides whether
// to call PersistRun (a dry run passes an in-memory FakeVerdictStore and
// never persists). Nothing here or anywhere else applies a proposal to any
// strategy configuration or trading behavior.
func GenerateRun(
	input optvalidation.Input,
	outcomes []tradingstack.SimOutcome,
	cfg RunConfig,
	verdicts overfitting.VerdictStore,
) (*RunResult, error) {
	pipelineResult, err := optvalidation.Run(input, cfg.PipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPipelineFailed, err)
	}

	gated := AssembleGatedOutcomes(pipelineResult, outcomes)
	gating := SummarizeGating(len(input.Rows), pipelineResult, len(gated))

	groups := ComputeGroupStats(gated, cfg.Optimizer)

	runID := cfg.RunID
	if runID == uuid.Nil {
		runID = uuid.New()
	}

	result := &RunResult{
		RunID:  runID,
		Gating: gating,
	}

	eligible := make([]GroupStats, 0, len(groups))
	for _, g := range groups {
		ok := g.DecidedCount >= cfg.Optimizer.MinGroupSamples
		result.Groups = append(result.Groups, GroupReport{
			StrategyID:   g.StrategyID,
			Regime:       g.Regime,
			DecidedCount: g.DecidedCount,
			Eligible:     ok,
			Stats:        g,
		})
		if ok {
			eligible = append(eligible, g)
		}
	}

	groupByKey := make(map[string]GroupStats, len(eligible))
	for _, g := range eligible {
		groupByKey[g.StrategyID+"\x00"+g.Regime] = g
	}

	for _, candidate := range GenerateCandidates(eligible, cfg.Optimizer) {
		group := groupByKey[candidate.StrategyID+"\x00"+candidate.Regime]
		proposalID := uuid.New()

		evidence, err := BuildEvidence(group, candidate, proposalID, cfg.Optimizer)
		if err != nil {
			return nil, err
		}

		verdict, err := overfitting.RunGate(evidence, cfg.Optimizer.GateConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGateFailed, err)
		}

		verdictRow := overfitting.NewOverfittingVerdict(verdict, cfg.Now)
		if err := verdicts.Persist(verdictRow); err != nil {
			return nil, fmt.Errorf("%w: verdict persistence: %v", ErrGateFailed, err)
		}

		status := StatusRejectedByGate
		if verdict.Passed {
			status = StatusPendingReview
		}

		evidenceJSON, err := json.Marshal(buildProposalEvidence(group, candidate, gating))
		if err != nil {
			return nil, fmt.Errorf("stratopt: evidence marshal failed: %w", err)
		}

		result.Proposals = append(result.Proposals, GeneratedProposal{
			Candidate:  candidate,
			Evidence:   evidence,
			Verdict:    verdict,
			VerdictRow: verdictRow,
			Proposal: StrategyProposal{
				ID:            proposalID,
				CreatedAt:     cfg.Now,
				RunID:         runID,
				StrategyID:    candidate.StrategyID,
				Regime:        candidate.Regime,
				Parameter:     candidate.Parameter,
				AdjustmentPct: candidate.AdjustmentPct,
				Rule:          candidate.Rule,
				Rationale:     candidate.Rationale,
				EvidenceJSON:  evidenceJSON,
				VerdictID:     verdictRow.ID,
				Status:        status,
			},
		})
	}

	return result, nil
}

// ProposalRows extracts the proposal rows of a run result, ready for
// PersistRun.
func (r *RunResult) ProposalRows() []StrategyProposal {
	rows := make([]StrategyProposal, 0, len(r.Proposals))
	for _, p := range r.Proposals {
		rows = append(rows, p.Proposal)
	}
	return rows
}
