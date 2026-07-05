package optvalidation

import (
	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// Input is the full set of in-memory rows and reference data the pipeline
// operates over. All fields are plain slices -- no database or network I/O
// happens inside Run or any stage it calls.
type Input struct {
	Rows      []TrainingRow
	Baseline  []tradingstack.FeatureDistribution
	Fidelity  []tradingstack.SimulatorFidelity
	EVWeights []tradingstack.StrategyEvWeight
}

// Result is the pipeline's full output: the clean weighted training dataset
// plus every stage's reporting/exclusion output, for audit purposes.
type Result struct {
	Clean              []WeightedTrainingRow
	Violations         []Violation
	DistributionReport DistributionReport
	DroppedByRegime    []TrainingRow
	DroppedByFidelity  []TrainingRow
}

// Run threads the five pipeline stages, in the fixed documented order,
// over input: timestamp audit -> distribution check -> regime confidence
// filter -> fidelity gate -> EV weight join. Running Run twice on the same
// Input and Config produces identical Result values. Returns ErrEmptyBatch
// when input.Rows is empty -- there is nothing to validate.
func Run(input Input, cfg Config) (Result, error) {
	if len(input.Rows) == 0 {
		return Result{}, ErrEmptyBatch
	}

	afterTimestampAudit, violations := TimestampAudit(input.Rows)
	distributionReport := DistributionCheck(afterTimestampAudit, input.Baseline, cfg)
	afterRegimeFilter, droppedByRegime := RegimeConfidenceFilter(afterTimestampAudit, cfg)
	afterFidelityGate, droppedByFidelity := FidelityGate(afterRegimeFilter, input.Fidelity)
	clean := EVWeightJoin(afterFidelityGate, input.EVWeights)

	return Result{
		Clean:              clean,
		Violations:         violations,
		DistributionReport: distributionReport,
		DroppedByRegime:    droppedByRegime,
		DroppedByFidelity:  droppedByFidelity,
	}, nil
}
