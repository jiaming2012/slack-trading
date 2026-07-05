package tradingstack

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTradingStackDB spins up an ephemeral PostgreSQL container, opens a GORM
// connection, and runs MigrateTradingStack. The container is auto-terminated on
// test cleanup.
func setupTradingStackDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "tradingstack_test"
	)

	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Tmpfs:        map[string]string{"/var/lib/postgresql/data": "rw"},
		Env: map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": pass,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, pass, dbName, port.Port())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, MigrateTradingStack(db))

	return db
}

func strp(s string) *string         { return &s }
func f64p(f float64) *float64       { return &f }
func intp(i int) *int               { return &i }
func timep(tm time.Time) *time.Time { return &tm }

func nowUTC() time.Time {
	// timestamptz resolution is microseconds; truncate so round-trips compare equal.
	return time.Now().UTC().Truncate(time.Microsecond)
}

// TestMigrateCreatesExactlySixTables asserts the migration creates the six
// trading-stack tables and no playground table.
func TestMigrateCreatesExactlySixTables(t *testing.T) {
	db := setupTradingStackDB(t)

	var tables []string
	err := db.Raw(
		`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`,
	).Scan(&tables).Error
	require.NoError(t, err)

	present := make(map[string]bool, len(tables))
	for _, tbl := range tables {
		present[tbl] = true
	}

	wantTradingStack := []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
	}
	for _, tbl := range wantTradingStack {
		assert.True(t, present[tbl], "expected trading-stack table %q to exist", tbl)
	}

	playgroundTables := []string{
		"playgrounds", "order_records", "trade_records",
		"equity_plot_records", "live_accounts", "live_account_plots",
	}
	for _, tbl := range playgroundTables {
		assert.False(t, present[tbl], "playground table %q must NOT be created by MigrateTradingStack", tbl)
	}
}

// TestMigrateIsIdempotent asserts a second migration run is a no-op returning nil.
func TestMigrateIsIdempotent(t *testing.T) {
	db := setupTradingStackDB(t)
	require.NoError(t, MigrateTradingStack(db))
}

// TestScanResultRoundTrip verifies a valid ScanResult writes and reads back
// field-for-field with a generated id.
func TestScanResultRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	sr := &ScanResult{
		ScannedAt:        now,
		Ticker:           "AAPL",
		RegimeTag:        strp("bull"),
		RegimeConfidence: f64p(0.87),
		Price:            f64p(212.34),
		VolumeRatio:      f64p(1.42),
		Rsi14:            f64p(61.5),
		AtrPct:           f64p(0.021),
		ShortInterest:    f64p(0.033),
		Sector:           strp("technology"),
		ScannerScore:     f64p(88.0),
		ScannerVersion:   strp("v1.2.3"),
		DataAsOf:         timep(now.Add(-time.Hour)),
	}
	require.NoError(t, db.Create(sr).Error)
	require.NotEqual(t, uuid.Nil, sr.ID, "id should be generated")

	var got ScanResult
	require.NoError(t, db.First(&got, "id = ?", sr.ID).Error)

	assert.Equal(t, sr.ID, got.ID)
	assert.True(t, sr.ScannedAt.Equal(got.ScannedAt))
	assert.Equal(t, "AAPL", got.Ticker)
	assert.Equal(t, "bull", *got.RegimeTag)
	assert.InDelta(t, 0.87, *got.RegimeConfidence, 1e-9)
	assert.InDelta(t, 212.34, *got.Price, 1e-9)
	assert.InDelta(t, 1.42, *got.VolumeRatio, 1e-9)
	assert.InDelta(t, 61.5, *got.Rsi14, 1e-9)
	assert.InDelta(t, 0.021, *got.AtrPct, 1e-9)
	assert.InDelta(t, 0.033, *got.ShortInterest, 1e-9)
	assert.Equal(t, "technology", *got.Sector)
	assert.InDelta(t, 88.0, *got.ScannerScore, 1e-9)
	assert.Equal(t, "v1.2.3", *got.ScannerVersion)
	require.NotNil(t, got.DataAsOf)
	assert.True(t, sr.DataAsOf.Equal(*got.DataAsOf))
}

// TestScanResultNullablesRoundTripNull verifies NULL columns round-trip as nil.
func TestScanResultNullablesRoundTripNull(t *testing.T) {
	db := setupTradingStackDB(t)

	sr := &ScanResult{ScannedAt: nowUTC(), Ticker: "MSFT"}
	require.NoError(t, db.Create(sr).Error)

	var got ScanResult
	require.NoError(t, db.First(&got, "id = ?", sr.ID).Error)

	assert.Nil(t, got.RegimeTag)
	assert.Nil(t, got.Price)
	assert.Nil(t, got.DataAsOf)
	assert.Equal(t, "MSFT", got.Ticker)
}

// TestScanResultDataAsOfInvariantRejected verifies data_as_of > scanned_at is
// rejected and no row is written.
func TestScanResultDataAsOfInvariantRejected(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	sr := &ScanResult{
		ScannedAt: now,
		Ticker:    "AAPL",
		DataAsOf:  timep(now.Add(time.Hour)), // strictly after scanned_at
	}
	err := db.Create(sr).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrScanDataAsOfAfterScannedAt)

	var count int64
	require.NoError(t, db.Model(&ScanResult{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no scan_results row should be written")
}

// TestSimOutcomeRoundTrip verifies a SimOutcome round-trips against an existing
// parent ScanResult.
func TestSimOutcomeRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	parent := &ScanResult{ScannedAt: now, Ticker: "AAPL"}
	require.NoError(t, db.Create(parent).Error)

	out := &SimOutcome{
		ScanResultID: parent.ID,
		SimulatedAt:  now,
		StrategyID:   strp("covered_call_v7"),
		EntryPrice:   f64p(210.0),
		ExitPrice:    f64p(215.5),
		StopPrice:    f64p(205.0),
		TargetPrice:  f64p(220.0),
		PnlPct:       f64p(2.62),
		HoldDays:     intp(4),
		ExitReason:   "target",
		MaxDrawdown:  f64p(-1.1),
		OutcomeLabel: strp("win"),
	}
	require.NoError(t, db.Create(out).Error)

	var got SimOutcome
	require.NoError(t, db.First(&got, "id = ?", out.ID).Error)
	assert.Equal(t, parent.ID, got.ScanResultID)
	assert.Equal(t, "target", got.ExitReason)
	assert.Equal(t, "covered_call_v7", *got.StrategyID)
	assert.Equal(t, 4, *got.HoldDays)
	assert.InDelta(t, 2.62, *got.PnlPct, 1e-9)
	assert.InDelta(t, -1.1, *got.MaxDrawdown, 1e-9)
}

// TestSimOutcomeInvalidExitReasonRejected verifies an unknown exit_reason is
// rejected and no row is written.
func TestSimOutcomeInvalidExitReasonRejected(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	parent := &ScanResult{ScannedAt: now, Ticker: "AAPL"}
	require.NoError(t, db.Create(parent).Error)

	out := &SimOutcome{
		ScanResultID: parent.ID,
		SimulatedAt:  now,
		ExitReason:   "moon", // not in the allowed set
	}
	err := db.Create(out).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidExitReason)

	var count int64
	require.NoError(t, db.Model(&SimOutcome{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no sim_outcomes row should be written")
}

// TestSimOutcomeOrphanForeignKeyRejected verifies a scan_result_id with no
// matching parent is rejected by the FK constraint and no row is written.
func TestSimOutcomeOrphanForeignKeyRejected(t *testing.T) {
	db := setupTradingStackDB(t)

	out := &SimOutcome{
		ScanResultID: uuid.New(), // no such scan_results row
		SimulatedAt:  nowUTC(),
		ExitReason:   "stop", // valid, so Go validation passes and DB FK fires
	}
	err := db.Create(out).Error
	require.Error(t, err, "orphan scan_result_id should violate the foreign key")

	var count int64
	require.NoError(t, db.Model(&SimOutcome{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no sim_outcomes row should be written")
}

// TestSimulatorFidelityRoundTrip verifies drift_score and within_tolerance
// round-trip.
func TestSimulatorFidelityRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	fid := &SimulatorFidelity{
		ComputedAt:      timep(now),
		StrategyID:      strp("covered_call_v7"),
		PeriodStart:     timep(now.Add(-30 * 24 * time.Hour)),
		PeriodEnd:       timep(now),
		DriftPnl:        f64p(120.5),
		DriftFill:       f64p(0.02),
		DriftScore:      f64p(0.42),
		WithinTolerance: true,
	}
	require.NoError(t, db.Create(fid).Error)

	var got SimulatorFidelity
	require.NoError(t, db.First(&got, "id = ?", fid.ID).Error)
	require.NotNil(t, got.DriftScore)
	assert.InDelta(t, 0.42, *got.DriftScore, 1e-9)
	assert.True(t, got.WithinTolerance)
}

// TestStrategyEvWeightRoundTrip verifies EV fields round-trip, preserving a
// negative ev_slope.
func TestStrategyEvWeightRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	w := &StrategyEvWeight{
		ComputedAt: timep(now),
		StrategyID: strp("covered_call_v7"),
		Regime:     strp("bull"),
		Ev30d:      f64p(0.031),
		Ev90d:      f64p(0.028),
		EvSlope:    f64p(-0.0004), // decaying
		EvWeight:   f64p(0.65),
	}
	require.NoError(t, db.Create(w).Error)

	var got StrategyEvWeight
	require.NoError(t, db.First(&got, "id = ?", w.ID).Error)
	assert.InDelta(t, 0.031, *got.Ev30d, 1e-9)
	assert.InDelta(t, 0.028, *got.Ev90d, 1e-9)
	assert.InDelta(t, -0.0004, *got.EvSlope, 1e-9)
	assert.Less(t, *got.EvSlope, 0.0, "negative slope must be preserved")
	assert.InDelta(t, 0.65, *got.EvWeight, 1e-9)
}

// TestScannerConfigJSONBRoundTrip verifies config_json round-trips to the same
// keys and values.
func TestScannerConfigJSONBRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)

	raw := []byte(`{"min_volume_ratio":1.5,"filters":["rsi","atr"]}`)
	cfg := &ScannerConfig{
		Regime:         strp("bull"),
		ConfigJSON:     raw,
		OptimizerRunID: strp("run-001"),
	}
	require.NoError(t, db.Create(cfg).Error)

	var got ScannerConfig
	require.NoError(t, db.First(&got, "id = ?", cfg.ID).Error)

	var wantMap, gotMap map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &wantMap))
	require.NoError(t, json.Unmarshal(got.ConfigJSON, &gotMap))
	assert.Equal(t, wantMap, gotMap, "config_json should deserialize to the same keys and values")
}

// TestScannerConfigRollbackByID verifies fetching a prior config by id returns
// that version, unaffected by a later row.
func TestScannerConfigRollbackByID(t *testing.T) {
	db := setupTradingStackDB(t)

	first := &ScannerConfig{
		Regime:     strp("bull"),
		ConfigJSON: []byte(`{"min_volume_ratio":1.5}`),
	}
	require.NoError(t, db.Create(first).Error)

	second := &ScannerConfig{
		Regime:     strp("bull"),
		ConfigJSON: []byte(`{"min_volume_ratio":2.0}`),
	}
	require.NoError(t, db.Create(second).Error)

	rolledBack, err := FetchScannerConfigByID(db, first.ID)
	require.NoError(t, err)

	var got, want map[string]interface{}
	require.NoError(t, json.Unmarshal(rolledBack.ConfigJSON, &got))
	require.NoError(t, json.Unmarshal(first.ConfigJSON, &want))
	assert.Equal(t, want, got, "rollback should return the first config verbatim")
	assert.Equal(t, first.ID, rolledBack.ID)
}

// TestFeatureDistributionRoundTrip verifies the surrogate-key model round-trips
// for all six declared fields.
func TestFeatureDistributionRoundTrip(t *testing.T) {
	db := setupTradingStackDB(t)
	now := nowUTC()

	fd := &FeatureDistribution{
		ComputedAt:  timep(now),
		FeatureName: strp("rsi_14"),
		Mean:        f64p(54.2),
		StdDev:      f64p(12.7),
		P25:         f64p(44.0),
		P75:         f64p(66.5),
	}
	require.NoError(t, db.Create(fd).Error)
	require.NotEqual(t, uuid.Nil, fd.ID)

	var got FeatureDistribution
	require.NoError(t, db.First(&got, "id = ?", fd.ID).Error)
	require.NotNil(t, got.ComputedAt)
	assert.True(t, now.Equal(*got.ComputedAt))
	assert.Equal(t, "rsi_14", *got.FeatureName)
	assert.InDelta(t, 54.2, *got.Mean, 1e-9)
	assert.InDelta(t, 12.7, *got.StdDev, 1e-9)
	assert.InDelta(t, 44.0, *got.P25, 1e-9)
	assert.InDelta(t, 66.5, *got.P75, 1e-9)
}
