package optvalidation

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// --- Row construction: NewTrainingRow ---------------------------------------

// TestNewTrainingRow_MapsEveryFieldFromDistinctSource gives every relevant
// ScanResult/SimOutcome field a distinct sentinel value and asserts each
// TrainingRow field maps from the correct source field. Any swap between two
// same-typed source fields (e.g. Price <-> RegimeConfidence, or Ticker <->
// RegimeTag <-> StrategyID) would make one of these assertions fail.
func TestNewTrainingRow_MapsEveryFieldFromDistinctSource(t *testing.T) {
	scanResultID := uuid.New()
	simOutcomeID := uuid.New()

	scannedAt := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	dataAsOf := time.Date(2024, 1, 9, 8, 0, 0, 0, time.UTC)

	ticker := "TICK-SENTINEL"
	regimeTag := "regime-sentinel"
	strategyID := "strategy-sentinel"

	regimeConfidence := 0.11
	price := 0.22
	volumeRatio := 0.33
	rsi14 := 0.44
	atrPct := 0.55
	scannerScore := 0.66

	sr := tradingstack.ScanResult{
		BaseModel:        tradingstack.BaseModel{ID: scanResultID},
		ScannedAt:        scannedAt,
		Ticker:           ticker,
		RegimeTag:        &regimeTag,
		RegimeConfidence: &regimeConfidence,
		Price:            &price,
		VolumeRatio:      &volumeRatio,
		Rsi14:            &rsi14,
		AtrPct:           &atrPct,
		ScannerScore:     &scannerScore,
		DataAsOf:         &dataAsOf,
	}
	so := tradingstack.SimOutcome{
		BaseModel:    tradingstack.BaseModel{ID: simOutcomeID},
		ScanResultID: scanResultID,
		StrategyID:   &strategyID,
	}

	row := NewTrainingRow(sr, so)

	assert.Equal(t, scanResultID, row.ScanResultID)
	assert.Equal(t, simOutcomeID, row.SimOutcomeID)
	assert.Equal(t, strategyID, row.StrategyID)
	assert.Equal(t, ticker, row.Ticker)
	assert.Equal(t, scannedAt, row.ScannedAt)
	assert.Equal(t, dataAsOf, row.DataAsOf)
	assert.Equal(t, regimeTag, row.RegimeTag)
	assert.Equal(t, regimeConfidence, row.RegimeConfidence)
	assert.Equal(t, price, row.Price)
	assert.Equal(t, volumeRatio, row.VolumeRatio)
	assert.Equal(t, rsi14, row.RSI14)
	assert.Equal(t, atrPct, row.ATRPct)
	assert.Equal(t, scannerScore, row.ScannerScore)
}

// TestNewTrainingRow_NilPointerFieldsDerefToZeroValue asserts the documented
// deref behavior: a nil DataAsOf, RegimeConfidence, or StrategyID becomes the
// Go zero value for its field rather than panicking.
func TestNewTrainingRow_NilPointerFieldsDerefToZeroValue(t *testing.T) {
	scanResultID := uuid.New()

	sr := tradingstack.ScanResult{
		BaseModel:        tradingstack.BaseModel{ID: scanResultID},
		ScannedAt:        time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC),
		Ticker:           "AAA",
		RegimeConfidence: nil,
		DataAsOf:         nil,
	}
	so := tradingstack.SimOutcome{
		ScanResultID: scanResultID,
		StrategyID:   nil,
	}

	row := NewTrainingRow(sr, so)

	assert.Equal(t, time.Time{}, row.DataAsOf)
	assert.Equal(t, 0.0, row.RegimeConfidence)
	assert.Equal(t, "", row.StrategyID)
}

// --- Stage 1: timestamp audit ---------------------------------------------

func TestTimestampAudit_ViolationExcludedAndReported(t *testing.T) {
	scannedAt := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)

	violatingRow := TrainingRow{
		ScanResultID: uuid.New(),
		StrategyID:   "S1",
		Ticker:       "AAA",
		ScannedAt:    scannedAt,
		DataAsOf:     scannedAt.Add(1 * time.Hour), // after scanned_at
	}
	cleanRow := TrainingRow{
		ScanResultID: uuid.New(),
		StrategyID:   "S2",
		Ticker:       "BBB",
		ScannedAt:    scannedAt,
		DataAsOf:     scannedAt.Add(-1 * time.Hour),
	}

	kept, violations := TimestampAudit([]TrainingRow{violatingRow, cleanRow})

	require.Len(t, kept, 1)
	assert.Equal(t, cleanRow, kept[0])

	require.Len(t, violations, 1)
	assert.Equal(t, violatingRow.ScanResultID, violations[0].ScanResultID)
	assert.Equal(t, violatingRow.StrategyID, violations[0].StrategyID)
}

func TestTimestampAudit_CleanBatchPassesThroughUnchanged(t *testing.T) {
	scannedAt := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	rows := []TrainingRow{
		{ScanResultID: uuid.New(), ScannedAt: scannedAt, DataAsOf: scannedAt},
		{ScanResultID: uuid.New(), ScannedAt: scannedAt, DataAsOf: scannedAt.Add(-24 * time.Hour)},
	}

	kept, violations := TimestampAudit(rows)

	assert.Equal(t, rows, kept)
	assert.Empty(t, violations)
}

// --- Stage 2: distribution check ------------------------------------------

func TestDistributionCheck_FeatureDriftingMoreThan2StdDevsIsFlagged(t *testing.T) {
	feature := "price"
	mean := 100.0
	stdDev := 5.0
	baseline := []tradingstack.FeatureDistribution{
		{FeatureName: &feature, Mean: &mean, StdDev: &stdDev},
	}

	// Batch mean = 115 = M + 3*S.
	rows := []TrainingRow{
		{Price: 115},
		{Price: 115},
	}

	report := DistributionCheck(rows, baseline, DefaultConfig())

	assert.Contains(t, report.Drifted, feature)
	assert.NotContains(t, report.NotDrifted, feature)
	// No row removed as a side effect.
	assert.Len(t, rows, 2)
}

func TestDistributionCheck_FeatureWithin2StdDevsIsNotFlagged(t *testing.T) {
	feature := "price"
	mean := 100.0
	stdDev := 5.0
	baseline := []tradingstack.FeatureDistribution{
		{FeatureName: &feature, Mean: &mean, StdDev: &stdDev},
	}

	// Batch mean = 105 = M + 1*S, within the 2*S threshold.
	rows := []TrainingRow{
		{Price: 105},
		{Price: 105},
	}

	report := DistributionCheck(rows, baseline, DefaultConfig())

	assert.NotContains(t, report.Drifted, feature)
	assert.Contains(t, report.NotDrifted, feature)
}

func TestDistributionCheck_EmptyBaselineYieldsEmptyReport(t *testing.T) {
	rows := []TrainingRow{{Price: 100}}

	report := DistributionCheck(rows, nil, DefaultConfig())

	assert.Empty(t, report.Drifted)
	assert.Empty(t, report.NotDrifted)
}

// --- Stage 3: regime confidence filter ------------------------------------

func TestRegimeConfidenceFilter_BelowAndAboveThreshold(t *testing.T) {
	below := TrainingRow{ScanResultID: uuid.New(), RegimeConfidence: 0.65}
	above := TrainingRow{ScanResultID: uuid.New(), RegimeConfidence: 0.9}

	kept, dropped := RegimeConfidenceFilter([]TrainingRow{below, above}, DefaultConfig())

	require.Len(t, kept, 1)
	assert.Equal(t, above, kept[0])

	require.Len(t, dropped, 1)
	assert.Equal(t, below, dropped[0])
}

func TestRegimeConfidenceFilter_ExactlyAtThresholdSurvives(t *testing.T) {
	atThreshold := TrainingRow{ScanResultID: uuid.New(), RegimeConfidence: 0.7}

	kept, dropped := RegimeConfidenceFilter([]TrainingRow{atThreshold}, DefaultConfig())

	require.Len(t, kept, 1)
	assert.Equal(t, atThreshold, kept[0])
	assert.Empty(t, dropped)
}

// --- Stage 4: fidelity gate ------------------------------------------------

func TestFidelityGate_RowInsideHighDriftPeriodIsDropped(t *testing.T) {
	strategyID := "S1"
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	fidelity := []tradingstack.SimulatorFidelity{
		{
			StrategyID:      &strategyID,
			PeriodStart:     &periodStart,
			PeriodEnd:       &periodEnd,
			WithinTolerance: false,
		},
	}

	insideRow := TrainingRow{StrategyID: "S1", DataAsOf: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}

	kept, dropped := FidelityGate([]TrainingRow{insideRow}, fidelity)

	assert.Empty(t, kept)
	require.Len(t, dropped, 1)
	assert.Equal(t, insideRow, dropped[0])
}

func TestFidelityGate_BoundaryInclusive(t *testing.T) {
	strategyID := "S1"
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	fidelity := []tradingstack.SimulatorFidelity{
		{
			StrategyID:      &strategyID,
			PeriodStart:     &periodStart,
			PeriodEnd:       &periodEnd,
			WithinTolerance: false,
		},
	}

	atStart := TrainingRow{StrategyID: "S1", DataAsOf: periodStart}
	atEnd := TrainingRow{StrategyID: "S1", DataAsOf: periodEnd}
	justBeforeStart := TrainingRow{StrategyID: "S1", DataAsOf: periodStart.Add(-time.Nanosecond)}
	justAfterEnd := TrainingRow{StrategyID: "S1", DataAsOf: periodEnd.Add(time.Nanosecond)}

	kept, dropped := FidelityGate([]TrainingRow{atStart, atEnd, justBeforeStart, justAfterEnd}, fidelity)

	assert.ElementsMatch(t, []TrainingRow{atStart, atEnd}, dropped)
	assert.ElementsMatch(t, []TrainingRow{justBeforeStart, justAfterEnd}, kept)
}

func TestFidelityGate_RowOutsideAllHighDriftPeriodsSurvives(t *testing.T) {
	strategyID := "S1"
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	fidelity := []tradingstack.SimulatorFidelity{
		{
			StrategyID:      &strategyID,
			PeriodStart:     &periodStart,
			PeriodEnd:       &periodEnd,
			WithinTolerance: false,
		},
	}

	outsideRow := TrainingRow{StrategyID: "S1", DataAsOf: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)}

	kept, dropped := FidelityGate([]TrainingRow{outsideRow}, fidelity)

	require.Len(t, kept, 1)
	assert.Equal(t, outsideRow, kept[0])
	assert.Empty(t, dropped)
}

func TestFidelityGate_EmptyFidelityInputDropsNothing(t *testing.T) {
	row := TrainingRow{StrategyID: "S1", DataAsOf: time.Now()}

	kept, dropped := FidelityGate([]TrainingRow{row}, nil)

	require.Len(t, kept, 1)
	assert.Equal(t, row, kept[0])
	assert.Empty(t, dropped)
}

func TestFidelityGate_WithinToleranceRecordDoesNotDrop(t *testing.T) {
	strategyID := "S1"
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	// within_tolerance = true -- not a high-drift period, should not drop.
	fidelity := []tradingstack.SimulatorFidelity{
		{
			StrategyID:      &strategyID,
			PeriodStart:     &periodStart,
			PeriodEnd:       &periodEnd,
			WithinTolerance: true,
		},
	}

	row := TrainingRow{StrategyID: "S1", DataAsOf: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}

	kept, dropped := FidelityGate([]TrainingRow{row}, fidelity)

	require.Len(t, kept, 1)
	assert.Empty(t, dropped)
}

// --- Stage 5: EV weight join -----------------------------------------------

func TestEVWeightJoin_MatchingRecordIsJoined(t *testing.T) {
	strategyID := "S1"
	regime := "trend"
	evWeight := 0.3
	weights := []tradingstack.StrategyEvWeight{
		{StrategyID: &strategyID, Regime: &regime, EvWeight: &evWeight},
	}

	row := TrainingRow{StrategyID: "S1", RegimeTag: "trend"}

	out := EVWeightJoin([]TrainingRow{row}, weights)

	require.Len(t, out, 1)
	assert.Equal(t, 0.3, out[0].EVWeight)
}

func TestEVWeightJoin_NoMatchGetsNeutralDefault(t *testing.T) {
	row := TrainingRow{StrategyID: "S1", RegimeTag: "trend"}

	out := EVWeightJoin([]TrainingRow{row}, nil)

	require.Len(t, out, 1)
	assert.Equal(t, 1.0, out[0].EVWeight)
}

func TestEVWeightJoin_DuplicateWeightsResolveToMostRecent(t *testing.T) {
	strategyID := "S1"
	regime := "trend"
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	oldWeight := 0.2
	newWeight := 0.8

	weights := []tradingstack.StrategyEvWeight{
		{StrategyID: &strategyID, Regime: &regime, EvWeight: &oldWeight, ComputedAt: &older},
		{StrategyID: &strategyID, Regime: &regime, EvWeight: &newWeight, ComputedAt: &newer},
	}

	row := TrainingRow{StrategyID: "S1", RegimeTag: "trend"}

	out := EVWeightJoin([]TrainingRow{row}, weights)

	require.Len(t, out, 1)
	assert.Equal(t, newWeight, out[0].EVWeight)
}

// --- Full pipeline integration ---------------------------------------------

func TestRun_FullPipelineIntegration(t *testing.T) {
	input := SyntheticInput()

	result, err := Run(input, DefaultConfig())
	require.NoError(t, err)

	require.Len(t, result.Violations, 1)
	assert.Equal(t, "strategy-lookahead", result.Violations[0].StrategyID)

	require.Len(t, result.DroppedByRegime, 1)
	assert.Equal(t, "strategy-low-confidence", result.DroppedByRegime[0].StrategyID)

	require.Len(t, result.DroppedByFidelity, 1)
	assert.Equal(t, "strategy-high-drift", result.DroppedByFidelity[0].StrategyID)

	require.Len(t, result.Clean, 1)
	assert.Equal(t, "strategy-clean", result.Clean[0].StrategyID)
	assert.Equal(t, 1.0, result.Clean[0].EVWeight)
}

func TestRun_GracefulDegradationWithEmptyReferenceTables(t *testing.T) {
	scannedAt := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	rows := []TrainingRow{
		{ScanResultID: uuid.New(), StrategyID: "S1", ScannedAt: scannedAt, DataAsOf: scannedAt, RegimeConfidence: 0.9},
		{ScanResultID: uuid.New(), StrategyID: "S2", ScannedAt: scannedAt, DataAsOf: scannedAt, RegimeConfidence: 0.8},
	}

	input := Input{Rows: rows} // Baseline, Fidelity, EVWeights all nil/empty.

	result, err := Run(input, DefaultConfig())
	require.NoError(t, err)

	assert.Empty(t, result.DroppedByFidelity)
	assert.Empty(t, result.DistributionReport.Drifted)
	require.Len(t, result.Clean, 2)
	for _, r := range result.Clean {
		assert.Equal(t, 1.0, r.EVWeight)
	}
}

func TestRun_EmptyBatchReturnsErrEmptyBatch(t *testing.T) {
	_, err := Run(Input{}, DefaultConfig())
	require.ErrorIs(t, err, ErrEmptyBatch)
}

func TestRun_Determinism(t *testing.T) {
	input := SyntheticInput()
	cfg := DefaultConfig()

	first, err := Run(input, cfg)
	require.NoError(t, err)

	second, err := Run(input, cfg)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}
