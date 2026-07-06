package stratopt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// pairedRow builds a matched (TrainingRow, SimOutcome) pair sharing an
// outcome id, with dataAsOf set so the fidelity gate can key on it.
func pairedRow(strategyID, regime string, scannedAt, dataAsOf time.Time, pnl float64, holdDays int, exitReason string) (optvalidation.TrainingRow, tradingstack.SimOutcome) {
	scanResultID := uuid.New()
	outcomeID := uuid.New()
	pnlCopy := pnl
	holdCopy := holdDays
	strategyCopy := strategyID

	row := optvalidation.TrainingRow{
		ScanResultID:     scanResultID,
		SimOutcomeID:     outcomeID,
		StrategyID:       strategyID,
		Ticker:           "TICK",
		ScannedAt:        scannedAt,
		DataAsOf:         dataAsOf,
		RegimeTag:        regime,
		RegimeConfidence: 0.9,
		PnlPct:           pnl,
	}
	outcome := tradingstack.SimOutcome{
		BaseModel:    tradingstack.BaseModel{ID: outcomeID},
		ScanResultID: scanResultID,
		SimulatedAt:  scannedAt.Add(time.Hour),
		StrategyID:   &strategyCopy,
		PnlPct:       &pnlCopy,
		HoldDays:     &holdCopy,
		ExitReason:   exitReason,
	}
	return row, outcome
}

// TestAssembleGatedOutcomes_FidelityDroppedOutcomeInfluencesNothing runs the
// real validation pipeline with a within_tolerance=false fidelity period
// covering one of two S1 outcomes: only the surviving outcome reaches the
// optimizer's statistics.
func TestAssembleGatedOutcomes_FidelityDroppedOutcomeInfluencesNothing(t *testing.T) {
	base := time.Date(2024, 4, 1, 9, 0, 0, 0, time.UTC)

	insideRow, insideOutcome := pairedRow("S1", "trend", base, base, -5.0, 1, "stop")
	outsideRow, outsideOutcome := pairedRow("S1", "trend", base.Add(48*time.Hour), base.Add(48*time.Hour), 1.5, 3, "target")

	strategyID := "S1"
	periodStart := base.Add(-time.Hour)
	periodEnd := base.Add(time.Hour)
	computedAt := base

	input := optvalidation.Input{
		Rows: []optvalidation.TrainingRow{insideRow, outsideRow},
		Fidelity: []tradingstack.SimulatorFidelity{{
			ComputedAt:      &computedAt,
			StrategyID:      &strategyID,
			PeriodStart:     &periodStart,
			PeriodEnd:       &periodEnd,
			WithinTolerance: false,
		}},
	}

	result, err := optvalidation.Run(input, optvalidation.DefaultConfig())
	require.NoError(t, err)
	require.Len(t, result.DroppedByFidelity, 1)
	require.Len(t, result.Clean, 1)

	gated := AssembleGatedOutcomes(result, []tradingstack.SimOutcome{insideOutcome, outsideOutcome})
	require.Len(t, gated, 1, "the fidelity-dropped outcome must be excluded")
	assert.Equal(t, outsideOutcome.ID, gated[0].SimOutcomeID)

	groups := ComputeGroupStats(gated, DefaultOptimizerConfig())
	require.Len(t, groups, 1)
	assert.Equal(t, 1, groups[0].DecidedCount)
	assert.InDelta(t, 1.5, groups[0].WeightedEV, 1e-12,
		"statistics must be computed from the surviving outcome only")
}

// TestAssembleGatedOutcomes_EVWeightAttached: a surviving row's pipeline
// ev_weight is carried onto its gated outcome and into the weighted
// statistics.
func TestAssembleGatedOutcomes_EVWeightAttached(t *testing.T) {
	base := time.Date(2024, 4, 1, 9, 0, 0, 0, time.UTC)
	row, outcome := pairedRow("S1", "trend", base, base, 2.0, 3, "target")

	strategyID := "S1"
	regime := "trend"
	weight := 0.3
	computedAt := base

	input := optvalidation.Input{
		Rows: []optvalidation.TrainingRow{row},
		EVWeights: []tradingstack.StrategyEvWeight{{
			ComputedAt: &computedAt,
			StrategyID: &strategyID,
			Regime:     &regime,
			EvWeight:   &weight,
		}},
	}

	result, err := optvalidation.Run(input, optvalidation.DefaultConfig())
	require.NoError(t, err)
	require.Len(t, result.Clean, 1)
	require.InDelta(t, 0.3, result.Clean[0].EVWeight, 1e-12)

	gated := AssembleGatedOutcomes(result, []tradingstack.SimOutcome{outcome})
	require.Len(t, gated, 1)
	assert.InDelta(t, 0.3, gated[0].EVWeight, 1e-12)
	assert.Equal(t, "trend", gated[0].Regime)
	assert.Equal(t, 3, gated[0].HoldDays)
	assert.Equal(t, "target", gated[0].ExitReason)
	assert.InDelta(t, 2.0, gated[0].PnlPct, 1e-12)
}

// TestGenerateRun_PipelineErrorAbortsWithZeroProposals: an empty batch makes
// the validation pipeline fail; the run aborts with a wrapped
// ErrPipelineFailed and no proposals.
func TestGenerateRun_PipelineErrorAbortsWithZeroProposals(t *testing.T) {
	result, err := GenerateRun(
		optvalidation.Input{},
		nil,
		SyntheticRunConfig(),
		overfitting.NewFakeVerdictStore(),
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPipelineFailed)
	assert.Nil(t, result, "an aborted run produces zero proposals")
}
