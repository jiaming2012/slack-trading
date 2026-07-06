package stratopt

import (
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// OptimizerConfig carries every tunable of the strategy optimizer: the
// candidate-generation minimum-sample pre-filter, the three rule thresholds
// with their hold-day cutoffs and step magnitudes, the walk-forward fold
// count for gate evidence, and the strategy-kind overfitting-gate thresholds.
// All defaults are the values signed off in the strategy-optimizer change
// (design.md D1/D6/D7).
type OptimizerConfig struct {
	// MinGroupSamples is the candidate-generation pre-filter: a
	// (strategy_id, regime) group with fewer decided (non-breakeven) gated
	// outcomes generates no candidates and is reported as insufficient-data.
	// This is a cheap economy measure only -- the overfitting gate remains
	// the final honesty bar for every candidate that is generated.
	MinGroupSamples int

	// StopChurnThreshold is the minimum ev-weighted share of losses exiting
	// via "stop" within StopChurnMaxHoldDays for the stop-churn rule to fire.
	StopChurnThreshold float64

	// StopChurnMaxHoldDays is the hold-day cutoff defining a "quick" stop-out.
	StopChurnMaxHoldDays int

	// TimeoutThreshold is the minimum ev-weighted share of losses exiting via
	// "timeout" for the timeout-drag rule to fire.
	TimeoutThreshold float64

	// FastTargetThreshold is the minimum ev-weighted share of wins exiting
	// via "target" within FastTargetMaxHoldDays for the fast-target rule to
	// fire.
	FastTargetThreshold float64

	// FastTargetMaxHoldDays is the hold-day cutoff defining a "fast" target
	// hit.
	FastTargetMaxHoldDays int

	// StopStepPct is the stop-churn rule's signed relative adjustment to
	// stop_pct (default +0.20 = widen the stop by 20%).
	StopStepPct float64

	// HoldStepPct is the timeout-drag rule's signed relative adjustment to
	// max_hold_days (default -0.20 = shorten the max hold by 20%).
	HoldStepPct float64

	// TargetStepPct is the fast-target rule's signed relative adjustment to
	// target_pct (default +0.10 = raise the target by 10%).
	TargetStepPct float64

	// FoldCount is the number of walk-forward folds built for gate evidence.
	// The group's gated outcomes are partitioned chronologically into
	// FoldCount+1 contiguous windows; fold i trains on window i and tests on
	// window i+1, so every fold's test window starts no earlier than its
	// train window ends, by construction.
	FoldCount int

	// GateConfig carries the strategy-kind overfitting-gate thresholds
	// submitted with every candidate's evidence.
	GateConfig overfitting.Config
}

// DefaultOptimizerConfig returns the signed-off defaults: a 50-decided-sample
// per-group pre-filter; rule thresholds of 0.5 with hold cutoffs of 2 days
// (stop-churn) and 1 day (fast-target); steps of +20% stop, -20% hold, +10%
// target; 4 walk-forward folds; and the strategy-kind gate config.
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		MinGroupSamples:       50,
		StopChurnThreshold:    0.5,
		StopChurnMaxHoldDays:  2,
		TimeoutThreshold:      0.5,
		FastTargetThreshold:   0.5,
		FastTargetMaxHoldDays: 1,
		StopStepPct:           0.20,
		HoldStepPct:           -0.20,
		TargetStepPct:         0.10,
		FoldCount:             4,
		GateConfig:            StrategyGateConfig(),
	}
}

// StrategyGateConfig returns the strategy-kind overfitting-gate thresholds:
// MinSamples 50 (aligned with the candidate-generation pre-filter, so the
// gate re-verifies the same bar on the submitted evidence itself) and
// MinOOSSamples 15 -- strategy groups are far thinner than scanner training
// data, so the gate library's scanner-scale defaults (500/50) would reject
// every early strategy group mechanically. All other thresholds inherit the
// gate library's DefaultConfig.
func StrategyGateConfig() overfitting.Config {
	cfg := overfitting.DefaultConfig()
	cfg.MinSamples = 50
	cfg.MinOOSSamples = 15
	return cfg
}
