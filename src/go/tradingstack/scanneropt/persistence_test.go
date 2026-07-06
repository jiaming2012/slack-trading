package scanneropt

import (
	"context"
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

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/overfitting"
)

// newTestDB spins up an ephemeral PostgreSQL container and runs the
// trading-stack-schema migration. The overfitting and scanner-optimizer
// migrations are left to each test so migration-ordering behavior stays
// testable. The container is auto-terminated on test cleanup.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "scanneropt_test"
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

// migrateAll runs the overfitting and scanner-optimizer migrations in their
// documented order.
func migrateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, overfitting.MigrateOverfittingCountermeasures(db))
	require.NoError(t, MigrateScannerOptimizer(db))
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

// TestMigrateScannerOptimizer_OrderingCreatesExactlyOneTableAndIsIdempotent
// covers three migration requirements in one container: the migration refuses
// to run before the overfitting migration (naming the missing dependency),
// creates exactly scanner_config_proposals once its FK target exists, and is
// idempotent.
func TestMigrateScannerOptimizer_OrderingCreatesExactlyOneTableAndIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	// Fresh DB without overfitting_verdicts: the FK target is missing.
	err := MigrateScannerOptimizer(db)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingVerdictTable)
	assert.Contains(t, err.Error(), "overfitting_verdicts")

	migrateAll(t, db)

	before := listPublicTables(t, db)
	assert.True(t, before["scanner_config_proposals"], "scanner_config_proposals must exist")

	tradingStackTables := []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
	}
	for _, tbl := range tradingStackTables {
		assert.True(t, before[tbl], "trading-stack table %q must be untouched (still present)", tbl)
	}
	assert.True(t, before["overfitting_verdicts"])
	assert.Len(t, before, len(tradingStackTables)+2,
		"exactly the six trading-stack tables, overfitting_verdicts, and scanner_config_proposals must exist")

	// Idempotency: a second run returns nil and leaves the table set unchanged.
	require.NoError(t, MigrateScannerOptimizer(db))
	after := listPublicTables(t, db)
	assert.Equal(t, before, after)
}

// TestRunCycle_DBPersistence covers the two run-outcome spec scenarios in one
// container: a passing run lands one pending_review proposal without touching
// scanner_configs, and a failing run is persisted as rejected_by_gate
// referencing its failing verdict.
func TestRunCycle_DBPersistence(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	verdicts := overfitting.NewGormVerdictStore(db)
	proposals := NewGormProposalStore(db)

	configsBefore := countRows(t, db, &tradingstack.ScannerConfig{})

	// --- Passing run ---
	passing, err := RunCycle(SyntheticWeightedRows(), SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	require.True(t, passing.Verdict.Passed)

	var storedPassing ScannerConfigProposal
	require.NoError(t, db.First(&storedPassing, "id = ?", passing.Proposal.ID).Error)
	assert.Equal(t, StatusPendingReview, storedPassing.Status)
	assert.Equal(t, passing.VerdictRow.ID, storedPassing.VerdictID)

	assert.Equal(t, configsBefore, countRows(t, db, &tradingstack.ScannerConfig{}),
		"scanner_configs must have exactly as many rows as before the run")

	// --- Failing run ---
	failing, err := RunCycle(SyntheticWeightedRows()[:100], SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	require.False(t, failing.Verdict.Passed)

	var storedFailing ScannerConfigProposal
	require.NoError(t, db.First(&storedFailing, "id = ?", failing.Proposal.ID).Error)
	assert.Equal(t, StatusRejectedByGate, storedFailing.Status)

	var verdictRow overfitting.OverfittingVerdict
	require.NoError(t, db.First(&verdictRow, "id = ?", storedFailing.VerdictID).Error)
	assert.False(t, verdictRow.Passed, "the referenced verdict row must record the failure")

	assert.Equal(t, configsBefore, countRows(t, db, &tradingstack.ScannerConfig{}))

	// Round-trip: the stored payload and evidence parse back.
	decodedEvidence, err := UnmarshalEvidence(storedPassing.EvidenceJSON)
	require.NoError(t, err)
	assert.Equal(t, passing.Evidence, decodedEvidence)
}

// TestGormPromote_SemanticsAndRefusals covers the three promote spec
// scenarios in one container: promoting a pending proposal creates the
// verbatim scanner_configs row and flips status; a gate-rejected proposal and
// an already-promoted proposal are refused with no writes.
func TestGormPromote_SemanticsAndRefusals(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	verdicts := overfitting.NewGormVerdictStore(db)
	proposals := NewGormProposalStore(db)

	passing, err := RunCycle(SyntheticWeightedRows(), SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	require.Equal(t, StatusPendingReview, passing.Proposal.Status)

	rejected, err := RunCycle(SyntheticWeightedRows()[:100], SyntheticBaseline(), SyntheticCycleConfig(), verdicts, proposals)
	require.NoError(t, err)
	require.Equal(t, StatusRejectedByGate, rejected.Proposal.Status)

	// --- Refusal: gate-rejected ---
	_, err = proposals.Promote(rejected.Proposal.ID, time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotPending)
	assert.Equal(t, int64(0), countRows(t, db, &tradingstack.ScannerConfig{}), "a refused promote writes nothing")
	stillRejected, err := proposals.FetchByID(rejected.Proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRejectedByGate, stillRejected.Status, "the refused proposal's status is unchanged")

	// --- Promote the pending proposal ---
	promotedAt := time.Date(2024, 7, 2, 8, 0, 0, 0, time.UTC)
	cfg, err := proposals.Promote(passing.Proposal.ID, promotedAt)
	require.NoError(t, err)

	var storedCfg tradingstack.ScannerConfig
	require.NoError(t, db.First(&storedCfg, "id = ?", cfg.ID).Error)
	assert.JSONEq(t, string(passing.Proposal.ProposedConfigJSON), string(storedCfg.ConfigJSON),
		"the promoted config_json must equal the proposal payload verbatim")
	require.NotNil(t, storedCfg.OptimizerRunID)
	assert.Equal(t, passing.Proposal.OptimizerRunID, *storedCfg.OptimizerRunID)
	assert.Nil(t, storedCfg.Regime)

	promoted, err := proposals.FetchByID(passing.Proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPromoted, promoted.Status)

	// --- Refusal: already promoted ---
	_, err = proposals.Promote(passing.Proposal.ID, time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotPending)
	assert.Equal(t, int64(1), countRows(t, db, &tradingstack.ScannerConfig{}),
		"a second promote must not create another config row")

	// --- Refusal: unknown id ---
	_, err = proposals.Promote(uuid.New(), time.Now().UTC())
	assert.ErrorIs(t, err, ErrProposalNotFound)
}

// TestProposalStatusVocabularyEnforced: an invalid status is rejected by the
// Go hook and, bypassing it, by the database CHECK constraint.
func TestProposalStatusVocabularyEnforced(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	verdictRow := overfitting.NewOverfittingVerdict(overfitting.Verdict{
		ProposalID:   uuid.New(),
		ProposalKind: overfitting.ProposalKindScanner,
	}, time.Now().UTC())
	require.NoError(t, overfitting.NewGormVerdictStore(db).Persist(verdictRow))

	row := ScannerConfigProposal{
		ID:                 uuid.New(),
		CreatedAt:          time.Now().UTC(),
		OptimizerRunID:     "run-x",
		ProposedConfigJSON: []byte(`{}`),
		EvidenceJSON:       []byte(`{}`),
		VerdictID:          verdictRow.ID,
		Status:             "dismissed",
	}

	err := db.Create(&row).Error
	require.Error(t, err, "the Go hook must reject a status outside the vocabulary")
	assert.ErrorIs(t, err, ErrInvalidProposalStatus)

	err = db.Exec(
		`INSERT INTO scanner_config_proposals
			(id, created_at, optimizer_run_id, proposed_config_json, evidence_json, verdict_id, status)
		 VALUES (?, now(), 'run-x', '{}', '{}', ?, 'dismissed')`,
		uuid.New(), verdictRow.ID,
	).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "chk_scanner_config_proposals_status"),
		"expected the CHECK constraint to reject the row, got: %v", err)

	// The FK is live too: a proposal referencing a nonexistent verdict fails.
	err = db.Exec(
		`INSERT INTO scanner_config_proposals
			(id, created_at, optimizer_run_id, proposed_config_json, evidence_json, verdict_id, status)
		 VALUES (?, now(), 'run-x', '{}', '{}', ?, 'pending_review')`,
		uuid.New(), uuid.New(),
	).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "fk_scanner_config_proposals_verdict"),
		"expected the FK constraint to reject the row, got: %v", err)
}
