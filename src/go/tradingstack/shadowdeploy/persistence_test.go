package shadowdeploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/scanneropt"
)

// newTestDB spins up an ephemeral PostgreSQL container and runs the
// trading-stack-schema migration. The overfitting, scanner-optimizer, and
// shadow-deployment migrations are left to each test so migration-ordering
// behavior stays testable. The container is auto-terminated on test cleanup.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "shadowdeploy_test"
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

	require.NoError(t, tradingstack.MigrateTradingStack(db))
	return db
}

// migrateAll runs the overfitting, scanner-optimizer, and shadow-deployment
// migrations in their documented order.
func migrateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, overfitting.MigrateOverfittingCountermeasures(db))
	require.NoError(t, scanneropt.MigrateScannerOptimizer(db))
	require.NoError(t, MigrateShadowDeployment(db))
}

func listPublicTables(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(
		`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`,
	).Scan(&tables).Error)

	present := make(map[string]bool, len(tables))
	for _, tbl := range tables {
		present[tbl] = true
	}
	return present
}

func countRows(t *testing.T, db *gorm.DB, model interface{}) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

// seedProposal persists one proposal (with its FK verdict row) carrying the
// given payload and status, returning its id.
func seedProposal(t *testing.T, db *gorm.DB, fixture SyntheticFixture, status string) uuid.UUID {
	t.Helper()

	verdictRow := overfitting.NewOverfittingVerdict(overfitting.Verdict{
		ProposalID:   uuid.New(),
		ProposalKind: overfitting.ProposalKindScanner,
	}, time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC))
	require.NoError(t, overfitting.NewGormVerdictStore(db).Persist(verdictRow))

	payloadJSON, err := fixture.ShadowPayload.Marshal()
	require.NoError(t, err)

	proposal := scanneropt.ScannerConfigProposal{
		ID:                 uuid.New(),
		CreatedAt:          time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
		OptimizerRunID:     "shadow-test-run",
		ProposedConfigJSON: payloadJSON,
		EvidenceJSON:       []byte(`{}`),
		VerdictID:          verdictRow.ID,
		Status:             status,
	}
	require.NoError(t, scanneropt.NewGormProposalStore(db).Persist(proposal))
	return proposal.ID
}

// seedObservations inserts the fixture's observations as scan_results rows
// and its outcome map as sim_outcomes rows.
func seedObservations(t *testing.T, db *gorm.DB, fixture SyntheticFixture) {
	t.Helper()

	for _, o := range fixture.Observations {
		row := tradingstack.ScanResult{
			BaseModel:   tradingstack.BaseModel{ID: o.ScanResultID},
			ScannedAt:   o.ScannedAt,
			Ticker:      o.Ticker,
			RegimeTag:   o.RegimeTag,
			VolumeRatio: o.VolumeRatio,
			Rsi14:       o.RSI14,
			AtrPct:      o.ATRPct,
		}
		require.NoError(t, db.Create(&row).Error)
	}
	for _, rows := range fixture.OutcomesByScanResult {
		for _, row := range rows {
			require.NoError(t, db.Create(&row).Error)
		}
	}
}

// TestMigrateShadowDeployment_OrderingCreatesExactlyTwoTablesAndIsIdempotent
// covers the migration requirements in one container: it refuses to run
// before the scanner-optimizer migration (naming the missing dependency),
// creates exactly shadow_runs and shadow_divergences once the FK target
// exists, and a second run changes nothing.
func TestMigrateShadowDeployment_OrderingCreatesExactlyTwoTablesAndIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	// Fresh DB without scanner_config_proposals: the FK target is missing.
	err := MigrateShadowDeployment(db)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingProposalTable)
	assert.Contains(t, err.Error(), "scanner_config_proposals")

	migrateAll(t, db)

	before := listPublicTables(t, db)
	assert.True(t, before["shadow_runs"])
	assert.True(t, before["shadow_divergences"])

	priorTables := []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
		"overfitting_verdicts", "scanner_config_proposals",
	}
	for _, tbl := range priorTables {
		assert.True(t, before[tbl], "pre-existing table %q must be untouched (still present)", tbl)
	}
	assert.Len(t, before, len(priorTables)+2,
		"exactly the prior tables plus shadow_runs and shadow_divergences must exist")

	// Idempotency: a second run returns nil and leaves the table set unchanged.
	require.NoError(t, MigrateShadowDeployment(db))
	after := listPublicTables(t, db)
	assert.Equal(t, before, after)
}

// TestShadowRun_RoundTripAndKindEnforcement covers the persistence spec
// scenarios in one container: a run with two divergences round-trips by id,
// and an invalid divergence kind is rejected by the Go hook and, bypassing
// it, by the database CHECK constraint.
func TestShadowRun_RoundTripAndKindEnforcement(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	fixture := Synthetic()
	proposalID := seedProposal(t, db, fixture, scanneropt.StatusPendingReview)

	store := NewGormShadowStore(db)

	summary, err := json.Marshal(CompareOutcomes(nil, nil, nil))
	require.NoError(t, err)

	run := ShadowRun{
		ID:                 uuid.New(),
		CreatedAt:          time.Date(2026, 3, 2, 18, 0, 0, 0, time.UTC),
		ProposalID:         proposalID,
		WindowStart:        time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		WindowEnd:          time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC),
		TotalObservations:  10,
		SelectedActive:     4,
		SelectedShadow:     4,
		SelectedBoth:       3,
		DivergencePct:      40,
		OutcomeSummaryJSON: summary,
		Synthetic:          true,
	}
	divergences := []ShadowDivergence{
		{Ticker: "AMZN", Kind: KindShadowOnly, ActiveScore: floatPtr(0.47), ShadowScore: floatPtr(0.60), ScanResultID: uuid.New()},
		{Ticker: "AMD", Kind: KindActiveOnly, ActiveScore: floatPtr(0.70), ShadowScore: floatPtr(0.30), ScanResultID: uuid.New()},
	}
	require.NoError(t, store.PersistRun(run, divergences))

	readBack, readDivergences, err := store.FetchRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, readBack.ID)
	assert.Nil(t, readBack.ActiveConfigID, "the null active config id (default baseline) round-trips")
	assert.Equal(t, proposalID, readBack.ProposalID)
	assert.Equal(t, 10, readBack.TotalObservations)
	assert.Equal(t, 4, readBack.SelectedActive)
	assert.Equal(t, 4, readBack.SelectedShadow)
	assert.Equal(t, 3, readBack.SelectedBoth)
	assert.InDelta(t, 40.0, readBack.DivergencePct, 1e-9)
	assert.True(t, readBack.Synthetic)
	assert.JSONEq(t, string(summary), string(readBack.OutcomeSummaryJSON))

	require.Len(t, readDivergences, 2, "exactly 2 shadow_divergences rows reference the run")
	assert.Equal(t, "AMD", readDivergences[0].Ticker, "divergences read back tickers ascending")
	assert.Equal(t, "AMZN", readDivergences[1].Ticker)
	for _, d := range readDivergences {
		assert.Equal(t, run.ID, d.ShadowRunID)
	}

	// FK: a run for a nonexistent proposal is refused.
	orphan := run
	orphan.ID = uuid.New()
	orphan.ProposalID = uuid.New()
	err = store.PersistRun(orphan, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fk_shadow_runs_proposal")

	// Invalid kind: the Go hook rejects it before the wire...
	err = store.PersistRun(ShadowRun{ProposalID: proposalID, CreatedAt: time.Now().UTC()},
		[]ShadowDivergence{{Ticker: "AAPL", Kind: "sideways"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDivergenceKind)

	// ...and bypassing GORM, the database CHECK constraint does.
	err = db.Exec(
		`INSERT INTO shadow_divergences (id, shadow_run_id, ticker, kind, scan_result_id)
		 VALUES (?, ?, 'AAPL', 'sideways', ?)`,
		uuid.New(), run.ID, uuid.New(),
	).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "chk_shadow_divergences_kind"),
		"expected the CHECK constraint to reject the row, got: %v", err)

	// The transactional persist left no partial evidence behind.
	assert.Equal(t, int64(1), countRows(t, db, &ShadowRun{}))
	assert.Equal(t, int64(2), countRows(t, db, &ShadowDivergence{}))
}

// TestRunShadow_DBScenarios covers the run-orchestration spec scenarios in
// one container: default-baseline fallback with a null active config id,
// same-inputs parallel evaluation, the persisted 40% fixture divergence,
// gate-rejected refusal, promoted re-shadowing, the empty-window sentinel,
// and the configs-and-proposals-untouched guarantee.
func TestRunShadow_DBScenarios(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	fixture := Synthetic()
	seedObservations(t, db, fixture)

	pendingID := seedProposal(t, db, fixture, scanneropt.StatusPendingReview)
	rejectedID := seedProposal(t, db, fixture, scanneropt.StatusRejectedByGate)
	promotedID := seedProposal(t, db, fixture, scanneropt.StatusPromoted)

	var proposalsBefore []scanneropt.ScannerConfigProposal
	require.NoError(t, db.Order("id ASC").Find(&proposalsBefore).Error)

	store := NewGormShadowStore(db)
	windowStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 9, 8, 0, 0, 0, time.UTC)

	// --- Scenario: empty scanner_configs falls back to the default baseline ---
	result, err := RunShadow(models.ModeSimulation, db, store, pendingID, windowStart, windowEnd, now)
	require.NoError(t, err)
	assert.Nil(t, result.Run.ActiveConfigID, "an empty scanner_configs table records a null active config id")
	assert.Equal(t, "builtin-default-v1", result.ActivePayload.Version)
	assert.Equal(t, 0, result.Report.SelectedActive, "the default payload has no regime models, so the active side selects nothing")
	assert.Equal(t, 4, result.Report.SelectedShadow)
	assert.InDelta(t, 100.0, result.Report.DivergencePct, 1e-9)

	// Both decision vectors cover exactly the same tickers in the same order.
	require.Equal(t, len(result.ActiveDecisions), len(result.ShadowDecisions))
	for i := range result.ActiveDecisions {
		assert.Equal(t, result.ActiveDecisions[i].Ticker, result.ShadowDecisions[i].Ticker)
		assert.Equal(t, result.ActiveDecisions[i].ScanResultID, result.ShadowDecisions[i].ScanResultID)
	}

	// --- Seed the active config; the fixture divergence becomes 40% ---
	activeJSON, err := fixture.ActivePayload.Marshal()
	require.NoError(t, err)
	createdAt := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	activeCfg := tradingstack.ScannerConfig{CreatedAt: &createdAt, ConfigJSON: activeJSON}
	require.NoError(t, db.Create(&activeCfg).Error)

	var configsBefore []tradingstack.ScannerConfig
	require.NoError(t, db.Order("id ASC").Find(&configsBefore).Error)

	result, err = RunShadow(models.ModeSimulation, db, store, pendingID, windowStart, windowEnd, now)
	require.NoError(t, err)
	require.NotNil(t, result.Run.ActiveConfigID)
	assert.Equal(t, activeCfg.ID, *result.Run.ActiveConfigID)
	assert.InDelta(t, 40.0, result.Report.DivergencePct, 1e-9)
	assert.Equal(t, 10, result.Run.TotalObservations)
	assert.Equal(t, 3, result.Run.SelectedBoth)

	// The persisted evidence round-trips: totals, divergences, outcome JSON.
	readBack, divergences, err := store.FetchRun(result.Run.ID)
	require.NoError(t, err)
	assert.InDelta(t, 40.0, readBack.DivergencePct, 1e-9)
	assert.False(t, readBack.Synthetic)
	require.Len(t, divergences, 2)
	assert.Equal(t, "AMD", divergences[0].Ticker)
	assert.Equal(t, KindActiveOnly, divergences[0].Kind)
	assert.Equal(t, "AMZN", divergences[1].Ticker)
	assert.Equal(t, KindShadowOnly, divergences[1].Kind)

	var summary OutcomeComparison
	require.NoError(t, json.Unmarshal(readBack.OutcomeSummaryJSON, &summary))
	assert.InDelta(t, 100.0, summary.Active.CoveragePct, 1e-9)
	assert.InDelta(t, 75.0, summary.Shadow.CoveragePct, 1e-9)
	assert.InDelta(t, 2.0/3.0, summary.Shadow.WinRate, 1e-9)
	assert.NotEmpty(t, summary.Limitation)

	// --- Scenario: a gate-rejected proposal is refused, nothing persisted ---
	runsBefore := countRows(t, db, &ShadowRun{})
	_, err = RunShadow(models.ModeSimulation, db, store, rejectedID, windowStart, windowEnd, now)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProposalRejectedByGate)
	assert.Equal(t, runsBefore, countRows(t, db, &ShadowRun{}), "a refused run writes no shadow_runs row")

	// --- Scenario: a promoted proposal may be re-shadowed ---
	result, err = RunShadow(models.ModeSimulation, db, store, promotedID, windowStart, windowEnd, now)
	require.NoError(t, err)
	assert.Equal(t, runsBefore+1, countRows(t, db, &ShadowRun{}))
	assert.Equal(t, promotedID, result.Run.ProposalID)

	// --- Scenario: an empty window persists nothing ---
	runsBefore = countRows(t, db, &ShadowRun{})
	_, err = RunShadow(models.ModeSimulation, db, store, pendingID,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), now)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoObservations)
	assert.Equal(t, runsBefore, countRows(t, db, &ShadowRun{}))

	// --- Completed runs left configs and proposals untouched ---
	var proposalsAfter []scanneropt.ScannerConfigProposal
	require.NoError(t, db.Order("id ASC").Find(&proposalsAfter).Error)
	assert.Equal(t, proposalsBefore, proposalsAfter,
		"scanner_config_proposals (statuses included) must be byte-identical to the pre-run contents")

	var configsAfter []tradingstack.ScannerConfig
	require.NoError(t, db.Order("id ASC").Find(&configsAfter).Error)
	assert.Equal(t, configsBefore, configsAfter, "scanner_configs must be untouched")

	// And the source data was only read, never written.
	assert.Equal(t, int64(len(fixture.Observations)), countRows(t, db, &tradingstack.ScanResult{}))
	assert.Equal(t, int64(5), countRows(t, db, &tradingstack.SimOutcome{}))
}
