package optvalidation

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// SyntheticInput builds an in-memory Input containing one TrainingRow per
// excluded category the pipeline is documented to produce -- a
// timestamp-audit violation, a low-regime-confidence row, and a
// high-drift-period row -- plus one fully-clean row that survives every
// filter stage and, having no matching StrategyEvWeight record, ends up
// demonstrating the neutral default ev_weight of 1.0. It also supplies a
// feature_distributions baseline chosen so the batch's aggregate "price"
// feature is flagged as drifted by the (advisory, non-dropping)
// distribution-check stage. It requires no database and no network, and is
// shared by the package's full-pipeline integration test and the
// optimizer-validate command's --synthetic self-check mode.
func SyntheticInput() Input {
	scannedAt := time.Date(2024, 1, 10, 15, 0, 0, 0, time.UTC)
	regimeTrend := "trend"

	// Row 1: excluded by the timestamp-audit stage -- data_as_of is after
	// scanned_at (lookahead bias). Excluded before the distribution check
	// runs, so it never contributes to the batch's feature means.
	lookaheadRow := TrainingRow{
		ScanResultID:     uuid.New(),
		StrategyID:       "strategy-lookahead",
		Ticker:           "AAA",
		ScannedAt:        scannedAt,
		DataAsOf:         scannedAt.Add(24 * time.Hour),
		RegimeTag:        regimeTrend,
		RegimeConfidence: 0.9,
		Price:            115,
	}

	// Row 2: survives the timestamp audit but is dropped by the regime
	// confidence filter (0.5 is strictly less than the default threshold
	// 0.7).
	lowConfidenceRow := TrainingRow{
		ScanResultID:     uuid.New(),
		StrategyID:       "strategy-low-confidence",
		Ticker:           "BBB",
		ScannedAt:        scannedAt,
		DataAsOf:         scannedAt.Add(-1 * time.Hour),
		RegimeTag:        regimeTrend,
		RegimeConfidence: 0.5,
		Price:            115,
	}

	// Row 3: survives the timestamp audit and regime filter but is dropped
	// by the fidelity gate -- its data_as_of falls inside a recorded
	// high-drift period for its strategy.
	strategyHighDrift := "strategy-high-drift"
	driftPeriodStart := scannedAt.Add(-48 * time.Hour)
	driftPeriodEnd := scannedAt.Add(-2 * time.Hour)
	highDriftRow := TrainingRow{
		ScanResultID:     uuid.New(),
		StrategyID:       strategyHighDrift,
		Ticker:           "CCC",
		ScannedAt:        scannedAt,
		DataAsOf:         scannedAt.Add(-24 * time.Hour),
		RegimeTag:        regimeTrend,
		RegimeConfidence: 0.9,
		Price:            115,
	}

	// Row 4: fully clean -- survives every filter stage. No
	// StrategyEvWeight record matches its (strategy, regime), so it
	// receives the neutral default ev_weight of 1.0 rather than being
	// dropped or zero-weighted.
	cleanRow := TrainingRow{
		ScanResultID:     uuid.New(),
		StrategyID:       "strategy-clean",
		Ticker:           "DDD",
		ScannedAt:        scannedAt,
		DataAsOf:         scannedAt.Add(-1 * time.Hour),
		RegimeTag:        regimeTrend,
		RegimeConfidence: 0.9,
		Price:            115,
	}

	rows := []TrainingRow{lookaheadRow, lowConfidenceRow, highDriftRow, cleanRow}

	// Baseline: the rows surviving the timestamp audit (rows 2-4) all carry
	// Price=115, so the current batch mean (115) sits 3 standard deviations
	// above this baseline's mean of 100 (std_dev 5, drift threshold 2*5=10)
	// -- flagged as drifted, without dropping any row.
	baselineFeature := "price"
	baselineMean := 100.0
	baselineStdDev := 5.0
	baseline := []tradingstack.FeatureDistribution{
		{FeatureName: &baselineFeature, Mean: &baselineMean, StdDev: &baselineStdDev},
	}

	// simulator_fidelity: one high-drift (within_tolerance=false) period for
	// strategyHighDrift covering highDriftRow's data_as_of.
	fidelityStrategy := strategyHighDrift
	fidelity := []tradingstack.SimulatorFidelity{
		{
			StrategyID:      &fidelityStrategy,
			PeriodStart:     &driftPeriodStart,
			PeriodEnd:       &driftPeriodEnd,
			WithinTolerance: false,
		},
	}

	// strategy_ev_weights: deliberately left empty so the clean row falls
	// through to the neutral default (1.0), demonstrating graceful
	// degradation when no EV history has accrued.
	var evWeights []tradingstack.StrategyEvWeight

	return Input{
		Rows:      rows,
		Baseline:  baseline,
		Fidelity:  fidelity,
		EVWeights: evWeights,
	}
}
