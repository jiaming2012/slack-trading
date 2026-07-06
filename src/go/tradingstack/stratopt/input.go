package stratopt

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
)

// GatedOutcome is one sim outcome that survived every stage of the
// optimizer-validation-pipeline, viewed through the fields the strategy
// optimizer's rules consume, with the pipeline's ev_weight attached. The
// regime tag comes from the pipeline row (it originates on the scan result);
// the outcome fields come from the joined tradingstack.SimOutcome.
type GatedOutcome struct {
	SimOutcomeID uuid.UUID
	StrategyID   string
	Regime       string
	SimulatedAt  time.Time
	PnlPct       float64
	HoldDays     int
	ExitReason   string
	EVWeight     float64
}

// GatingSummary reports what the validation pipeline kept and dropped, per
// stage, for one generation run -- carried into every proposal's evidence so
// a recommendation is auditable back to its data trust gate.
type GatingSummary struct {
	InputRows           int `json:"input_rows"`
	TimestampViolations int `json:"timestamp_violations"`
	DroppedByRegime     int `json:"dropped_by_regime"`
	DroppedByFidelity   int `json:"dropped_by_fidelity"`
	CleanRows           int `json:"clean_rows"`
	GatedOutcomes       int `json:"gated_outcomes"`
}

// SummarizeGating builds the per-stage kept/dropped summary from the pipeline
// result. inputRows is the pre-pipeline row count; gatedOutcomes is the count
// of clean rows successfully joined back to their sim outcomes.
func SummarizeGating(inputRows int, result optvalidation.Result, gatedOutcomes int) GatingSummary {
	return GatingSummary{
		InputRows:           inputRows,
		TimestampViolations: len(result.Violations),
		DroppedByRegime:     len(result.DroppedByRegime),
		DroppedByFidelity:   len(result.DroppedByFidelity),
		CleanRows:           len(result.Clean),
		GatedOutcomes:       gatedOutcomes,
	}
}

// AssembleGatedOutcomes joins the validation pipeline's clean weighted output
// back to the full sim-outcome batch by SimOutcomeID, keeping exactly the
// outcomes whose ids survive in pipelineResult.Clean and attaching each
// surviving row's ev_weight and regime tag. An outcome excluded by any
// pipeline stage never appears in the result and so can influence no
// statistic or proposal.
//
// The result is sorted by SimulatedAt (ties broken by SimOutcomeID) so
// identical inputs always assemble identically.
func AssembleGatedOutcomes(pipelineResult optvalidation.Result, outcomes []tradingstack.SimOutcome) []GatedOutcome {
	cleanByOutcomeID := make(map[uuid.UUID]optvalidation.WeightedTrainingRow, len(pipelineResult.Clean))
	for _, row := range pipelineResult.Clean {
		cleanByOutcomeID[row.SimOutcomeID] = row
	}

	gated := make([]GatedOutcome, 0, len(pipelineResult.Clean))
	for _, o := range outcomes {
		row, ok := cleanByOutcomeID[o.ID]
		if !ok {
			continue
		}

		pnl := 0.0
		if o.PnlPct != nil {
			pnl = *o.PnlPct
		}
		holdDays := 0
		if o.HoldDays != nil {
			holdDays = *o.HoldDays
		}
		strategyID := row.StrategyID
		if strategyID == "" && o.StrategyID != nil {
			strategyID = *o.StrategyID
		}

		gated = append(gated, GatedOutcome{
			SimOutcomeID: o.ID,
			StrategyID:   strategyID,
			Regime:       row.RegimeTag,
			SimulatedAt:  o.SimulatedAt,
			PnlPct:       pnl,
			HoldDays:     holdDays,
			ExitReason:   o.ExitReason,
			EVWeight:     row.EVWeight,
		})
	}

	sort.SliceStable(gated, func(i, j int) bool {
		if !gated[i].SimulatedAt.Equal(gated[j].SimulatedAt) {
			return gated[i].SimulatedAt.Before(gated[j].SimulatedAt)
		}
		return gated[i].SimOutcomeID.String() < gated[j].SimOutcomeID.String()
	})

	return gated
}
