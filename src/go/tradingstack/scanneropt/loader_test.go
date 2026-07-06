package scanneropt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/optvalidation"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scannercfg"
)

// seedScanResult inserts one scan_results row with fully populated features.
func seedScanResult(t *testing.T, db *gorm.DB, ticker, regime string, confidence float64, scannedAt, dataAsOf time.Time) tradingstack.ScanResult {
	t.Helper()
	price := 100.0
	vr := 1.5
	rsi := 60.0
	atr := 2.0
	sr := tradingstack.ScanResult{
		BaseModel:        tradingstack.BaseModel{ID: uuid.New()},
		ScannedAt:        scannedAt,
		Ticker:           ticker,
		RegimeTag:        &regime,
		RegimeConfidence: &confidence,
		Price:            &price,
		VolumeRatio:      &vr,
		Rsi14:            &rsi,
		AtrPct:           &atr,
		DataAsOf:         &dataAsOf,
	}
	require.NoError(t, db.Create(&sr).Error)
	return sr
}

// seedSimOutcome links one sim_outcomes row to a scan result.
func seedSimOutcome(t *testing.T, db *gorm.DB, scanResultID uuid.UUID, strategyID string, pnlPct float64, simulatedAt time.Time) tradingstack.SimOutcome {
	t.Helper()
	label := "win"
	if pnlPct < 0 {
		label = "loss"
	}
	so := tradingstack.SimOutcome{
		BaseModel:    tradingstack.BaseModel{ID: uuid.New()},
		ScanResultID: scanResultID,
		SimulatedAt:  simulatedAt,
		StrategyID:   &strategyID,
		PnlPct:       &pnlPct,
		ExitReason:   "target",
		OutcomeLabel: &label,
	}
	require.NoError(t, db.Create(&so).Error)
	return so
}

// seedTimestampViolation inserts a scan_results row whose data_as_of is AFTER
// scanned_at -- legacy lookahead-biased data. Both the Go BeforeSave hook and
// the chk_scan_results_data_as_of CHECK constraint forbid writing such a row
// today, so the test drops the constraint and inserts raw, simulating data
// that predates the constraint. The validation pipeline's timestamp audit
// exists precisely as defense-in-depth against rows like this.
func seedTimestampViolation(t *testing.T, db *gorm.DB, ticker, regime string, scannedAt time.Time) uuid.UUID {
	t.Helper()
	require.NoError(t, db.Exec(
		`ALTER TABLE scan_results DROP CONSTRAINT IF EXISTS chk_scan_results_data_as_of`,
	).Error)

	id := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO scan_results
			(id, scanned_at, ticker, regime_tag, regime_confidence, price, volume_ratio, rsi_14, atr_pct, data_as_of)
		 VALUES (?, ?, ?, ?, 0.9, 100, 1.5, 60, 2.0, ?)`,
		id, scannedAt, ticker, regime, scannedAt.Add(24*time.Hour),
	).Error)
	return id
}

// TestLoadInput_JoinsOnlyRowsWithOutcomesInsideTheWindow pins the loader spec
// scenario: an outcome-less scan result and an out-of-window joined pair are
// excluded; exactly the inside-window joined pair loads as a TrainingRow.
func TestLoadInput_JoinsOnlyRowsWithOutcomesInsideTheWindow(t *testing.T) {
	db := newTestDB(t)

	from := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	inWindow := time.Date(2024, 5, 10, 15, 0, 0, 0, time.UTC)

	joined := seedScanResult(t, db, "IN-WINDOW", "trend", 0.9, inWindow, inWindow.Add(-time.Hour))
	outcome := seedSimOutcome(t, db, joined.ID, "strategy-a", 0.02, inWindow.Add(time.Hour))

	// Outcome-less scan result inside the window: excluded by the inner join.
	seedScanResult(t, db, "NO-OUTCOME", "trend", 0.9, inWindow.Add(time.Minute), inWindow.Add(-time.Hour))

	// Joined pair outside the window: excluded by the half-open window.
	outOfWindow := time.Date(2024, 6, 5, 15, 0, 0, 0, time.UTC)
	late := seedScanResult(t, db, "LATE", "trend", 0.9, outOfWindow, outOfWindow.Add(-time.Hour))
	seedSimOutcome(t, db, late.ID, "strategy-a", 0.03, outOfWindow.Add(time.Hour))

	// A pair exactly at the exclusive upper bound: excluded (half-open).
	atBound := seedScanResult(t, db, "AT-BOUND", "trend", 0.9, to, to.Add(-time.Hour))
	seedSimOutcome(t, db, atBound.ID, "strategy-a", 0.01, to.Add(time.Hour))

	input, err := LoadInput(db, from, to)
	require.NoError(t, err)

	require.Len(t, input.Rows, 1, "exactly the one inside-window joined pair must load")
	assert.Equal(t, "IN-WINDOW", input.Rows[0].Ticker)
	assert.Equal(t, joined.ID, input.Rows[0].ScanResultID)
	assert.Equal(t, outcome.ID, input.Rows[0].SimOutcomeID)
	assert.Equal(t, 0.02, input.Rows[0].PnlPct)
}

// TestLoadInput_DBBackedRunMatchesPipelineFiltering pins the DB-backed spec
// scenario: a seeded timestamp violation and a low-regime-confidence row load
// into the pipeline input but are excluded from the tuner's input (the
// pipeline's Clean output), matching the pipeline's documented filtering. The
// reference tables load wholesale -- a matching strategy_ev_weights row
// weights the surviving rows.
func TestLoadInput_DBBackedRunMatchesPipelineFiltering(t *testing.T) {
	db := newTestDB(t)

	from := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	base := time.Date(2024, 5, 10, 15, 0, 0, 0, time.UTC)

	clean := seedScanResult(t, db, "CLEAN", "trend", 0.9, base, base.Add(-time.Hour))
	seedSimOutcome(t, db, clean.ID, "strategy-clean", 0.02, base.Add(time.Hour))

	lowConf := seedScanResult(t, db, "LOW-CONF", "trend", 0.5, base.Add(time.Minute), base.Add(-time.Hour))
	seedSimOutcome(t, db, lowConf.ID, "strategy-low-conf", 0.01, base.Add(time.Hour))

	violationID := seedTimestampViolation(t, db, "LOOKAHEAD", "trend", base.Add(2*time.Minute))
	seedSimOutcome(t, db, violationID, "strategy-lookahead", 0.05, base.Add(time.Hour))

	// Reference tables: one EV weight matching the clean row's
	// (strategy, regime).
	computedAt := base.Add(-24 * time.Hour)
	strategyClean := "strategy-clean"
	regimeTrend := "trend"
	evWeight := 2.0
	require.NoError(t, db.Create(&tradingstack.StrategyEvWeight{
		ComputedAt: &computedAt,
		StrategyID: &strategyClean,
		Regime:     &regimeTrend,
		EvWeight:   &evWeight,
	}).Error)

	input, err := LoadInput(db, from, to)
	require.NoError(t, err)
	require.Len(t, input.Rows, 3, "all three joined pairs load; exclusion is the pipeline's job")
	require.Len(t, input.EVWeights, 1, "reference tables load wholesale")

	result, err := optvalidation.Run(input, optvalidation.DefaultConfig())
	require.NoError(t, err)

	require.Len(t, result.Clean, 1, "the tuner's input excludes the violation and low-confidence rows")
	assert.Equal(t, "CLEAN", result.Clean[0].Ticker)
	assert.Equal(t, 2.0, result.Clean[0].EVWeight, "the seeded EV weight joins through the pipeline")

	require.Len(t, result.Violations, 1)
	assert.Equal(t, violationID, result.Violations[0].ScanResultID)
	require.Len(t, result.DroppedByRegime, 1)
	assert.Equal(t, lowConf.ID, result.DroppedByRegime[0].ScanResultID)
}

// TestLoadActiveBaseline_EmptyTableFallsBackToDefault and with rows resolves
// the latest created_at row's payload.
func TestLoadActiveBaseline_ResolvesLatestOrDefault(t *testing.T) {
	db := newTestDB(t)

	payload, err := LoadActiveBaseline(db)
	require.NoError(t, err)
	assert.Equal(t, scannercfg.DefaultPayload(), payload,
		"an empty scanner_configs table falls back to the built-in default payload")

	olderAt := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	newerAt := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	older := `{"version": "older", "regime_models": {}, "global": {"top_n_candidates": 10, "min_labeled_samples": 100}}`
	newer := `{"version": "newer", "regime_models": {}, "global": {"top_n_candidates": 30, "min_labeled_samples": 200}}`
	require.NoError(t, db.Create(&tradingstack.ScannerConfig{CreatedAt: &olderAt, ConfigJSON: []byte(older)}).Error)
	require.NoError(t, db.Create(&tradingstack.ScannerConfig{CreatedAt: &newerAt, ConfigJSON: []byte(newer)}).Error)

	payload, err = LoadActiveBaseline(db)
	require.NoError(t, err)
	assert.Equal(t, "newer", payload.Version, "the latest created_at row is the active baseline")
	assert.Equal(t, 30, payload.Global.TopNCandidates)
}
