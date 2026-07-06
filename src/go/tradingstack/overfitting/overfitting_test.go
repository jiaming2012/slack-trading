package overfitting

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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
)

// --- 5.1 Minimum-sample gate ---

func TestCheckMinSamples_AtThresholdPasses(t *testing.T) {
	e := SyntheticEvidencePass()
	e.SampleSize = 500
	e.OutOfSample.SampleSize = 50

	result := checkMinSamples(e, DefaultConfig())

	assert.True(t, result.Passed, "values exactly at the thresholds must pass")
	assert.Equal(t, 500.0, result.Observed["total_samples"])
	assert.Equal(t, 50.0, result.Observed["out_of_sample_samples"])
	assert.Equal(t, 500.0, result.Threshold["min_samples"])
	assert.Equal(t, 50.0, result.Threshold["min_oos_samples"])
}

func TestCheckMinSamples_TooFewTotalSamplesFails(t *testing.T) {
	e := SyntheticEvidencePass()
	e.SampleSize = 499
	e.OutOfSample.SampleSize = 200

	result := checkMinSamples(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "total labeled samples 499 below minimum 500")
	assert.NotContains(t, result.Detail, "out-of-sample samples")
}

func TestCheckMinSamples_TooFewOOSSamplesFails(t *testing.T) {
	e := SyntheticEvidencePass()
	e.SampleSize = 2000
	e.OutOfSample.SampleSize = 49

	result := checkMinSamples(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "out-of-sample samples 49 below minimum 50")
	assert.NotContains(t, result.Detail, "total labeled samples")
}

// --- 5.2 Walk-forward check ---

func TestCheckWalkForward_HealthyFoldsPass(t *testing.T) {
	// The pass fixture: 4 chronologically ordered, non-lookahead folds,
	// in-sample mean 0.02, out-of-sample mean 0.015 (retention 0.75 >= 0.5).
	result := checkWalkForward(SyntheticEvidencePass(), DefaultConfig())

	assert.True(t, result.Passed)
	assert.Equal(t, 4.0, result.Observed["fold_count"])
	assert.InDelta(t, 0.75, result.Observed["oos_retention_ratio"], 1e-9)
}

func TestCheckWalkForward_RetentionCollapseFails(t *testing.T) {
	e := SyntheticEvidencePass()
	e.InSample.Mean = 0.02
	e.OutOfSample.Mean = 0.004

	result := checkWalkForward(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "retention")
}

func TestCheckWalkForward_LookaheadFoldFails(t *testing.T) {
	e := SyntheticEvidencePass()
	// Push fold 2's test window start strictly before its train window end.
	e.Folds[1].TestStart = e.Folds[1].TrainEnd.Add(-24 * time.Hour)

	result := checkWalkForward(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "lookahead in fold 2")
}

func TestCheckWalkForward_TooFewFoldsFail(t *testing.T) {
	e := SyntheticEvidencePass()
	e.Folds = e.Folds[:2]

	result := checkWalkForward(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "fold count 2 below minimum 3")
}

func TestCheckWalkForward_NonPositiveInSampleMeanFailsOutright(t *testing.T) {
	for _, mean := range []float64{0, -0.01} {
		e := SyntheticEvidencePass()
		e.InSample.Mean = mean
		e.OutOfSample.Mean = 0.5 // excellent OOS numbers cannot rescue it

		result := checkWalkForward(e, DefaultConfig())

		assert.False(t, result.Passed, "in-sample mean %v must fail outright", mean)
		assert.Contains(t, result.Detail, "in-sample mean")
	}
}

func TestCheckWalkForward_NonIncreasingIndicesFail(t *testing.T) {
	e := SyntheticEvidencePass()
	e.Folds[2].Index = e.Folds[1].Index

	result := checkWalkForward(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "not strictly increasing")
}

// --- 5.3 Parameter-stability check ---

func TestCheckParamStability_WithinBoundsPasses(t *testing.T) {
	result := checkParamStability(SyntheticEvidencePass(), DefaultConfig())

	assert.True(t, result.Passed)
	assert.InDelta(t, 0.10, result.Observed["max_relative_step_observed"], 1e-9)
}

func TestCheckParamStability_OversizedStepNamesParameter(t *testing.T) {
	// Spec scenario: baseline 1.2, proposed 1.8 -- a 50% move.
	e := SyntheticEvidencePass()
	e.BaselineParams["entry_trigger"] = 1.2
	e.ProposedParams["entry_trigger"] = 1.8

	result := checkParamStability(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, `parameter "entry_trigger" step 0.5000 exceeds max relative step 0.2500`)
}

func TestCheckParamStability_FoldUnstableParameterNamesParameter(t *testing.T) {
	e := SyntheticEvidencePass()
	// All step bounds hold, but "stop_pct" swings wildly fold-to-fold:
	// values 0.01/0.09/0.01/0.09 -> mean 0.05, pop stddev 0.04, CV 0.8 > 0.5.
	wild := []float64{0.01, 0.09, 0.01, 0.09}
	for i := range e.Folds {
		e.Folds[i].Params["stop_pct"] = wild[i]
	}

	result := checkParamStability(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, `parameter "stop_pct" cross-fold coefficient of variation`)
}

func TestCheckParamStability_ProposalOnlyParameterNotedNotFailed(t *testing.T) {
	e := SyntheticEvidencePass()
	e.ProposedParams["brand_new_knob"] = 42.0

	result := checkParamStability(e, DefaultConfig())

	assert.True(t, result.Passed, "an unmatched parameter must not fail the check")
	assert.Contains(t, result.Detail, `parameter "brand_new_knob" present only in proposal`)
}

func TestCheckParamStability_ZeroBaselineParameterSkipped(t *testing.T) {
	e := SyntheticEvidencePass()
	e.BaselineParams["offset"] = 0.0
	e.ProposedParams["offset"] = 100.0 // any move from zero baseline is skipped, not failed

	result := checkParamStability(e, DefaultConfig())

	assert.True(t, result.Passed)
	assert.Contains(t, result.Detail, `parameter "offset" has zero baseline; skipped by step bound`)
}

// --- 5.4 Deflated performance check ---

// deflatedEvidence returns evidence whose out-of-sample t-statistic is
// exactly 2.5: 0.025 / (0.05 / sqrt(25)) = 2.5.
func deflatedEvidence(trials int) Evidence {
	e := SyntheticEvidencePass()
	e.TrialsCount = trials
	e.OutOfSample.Mean = 0.025
	e.OutOfSample.StdDev = 0.05
	e.OutOfSample.SampleSize = 25
	return e
}

func TestCheckDeflated_SmallSearchPasses(t *testing.T) {
	result := checkDeflated(deflatedEvidence(1), DefaultConfig())

	assert.True(t, result.Passed, "t 2.5 must clear the bar max(2.0, sqrt(2*ln 1)=0) = 2.0")
	assert.InDelta(t, 2.5, result.Observed["t_stat"], 1e-9)
	assert.InDelta(t, 2.0, result.Threshold["effective_t_stat_bar"], 1e-9)
}

func TestCheckDeflated_LargeSearchRaisesTheBar(t *testing.T) {
	result := checkDeflated(deflatedEvidence(100), DefaultConfig())

	assert.False(t, result.Passed, "the same t 2.5 must fail the 100-trial bar ~3.03")
	assert.InDelta(t, 2.5, result.Observed["t_stat"], 1e-9)
	assert.InDelta(t, 3.0349, result.Threshold["effective_t_stat_bar"], 1e-3)
}

func TestCheckDeflated_ZeroStdDevFailsClosed(t *testing.T) {
	e := deflatedEvidence(1)
	e.OutOfSample.StdDev = 0

	result := checkDeflated(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "no honest dispersion evidence")
}

func TestCheckDeflated_TinySampleFailsClosed(t *testing.T) {
	e := deflatedEvidence(1)
	e.OutOfSample.SampleSize = 1

	result := checkDeflated(e, DefaultConfig())

	assert.False(t, result.Passed)
	assert.Contains(t, result.Detail, "sample size 1 is below 2")
}

// --- 5.5 Gate orchestration ---

func TestRunGate_OneFailingCheckFailsVerdictButAllChecksReported(t *testing.T) {
	e := SyntheticEvidencePass()
	e.SampleSize = 100 // violate only the minimum-sample gate

	verdict, err := RunGate(e, DefaultConfig())
	require.NoError(t, err)

	assert.False(t, verdict.Passed)
	require.Len(t, verdict.Checks, 4, "all four checks must run without short-circuit")

	wantOrder := []string{CheckNameMinSamples, CheckNameWalkForward, CheckNameParamStability, CheckNameDeflated}
	passing := 0
	for i, c := range verdict.Checks {
		assert.Equal(t, wantOrder[i], c.Name, "checks must run in fixed order")
		if c.Passed {
			passing++
		}
	}
	assert.Equal(t, 3, passing, "exactly the minimum-sample check should fail")
	assert.False(t, verdict.Checks[0].Passed)
}

func TestRunGate_UnknownProposalKindErrors(t *testing.T) {
	e := SyntheticEvidencePass()
	e.ProposalKind = ProposalKind("ml-ranking")

	_, err := RunGate(e, DefaultConfig())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownProposalKind)
}

func TestRunGate_Deterministic(t *testing.T) {
	for _, evidence := range []Evidence{SyntheticEvidencePass(), SyntheticEvidenceFail()} {
		first, err := RunGate(evidence, DefaultConfig())
		require.NoError(t, err)
		second, err := RunGate(evidence, DefaultConfig())
		require.NoError(t, err)

		assert.True(t, reflect.DeepEqual(first, second), "verdicts must be identical values")

		firstJSON, err := json.Marshal(first)
		require.NoError(t, err)
		secondJSON, err := json.Marshal(second)
		require.NoError(t, err)
		assert.Equal(t, string(firstJSON), string(secondJSON), "verdicts must serialize byte-identically")
	}
}

func TestRunGate_PassFixturePassesAndFailFixtureFailsEveryCheck(t *testing.T) {
	pass, err := RunGate(SyntheticEvidencePass(), DefaultConfig())
	require.NoError(t, err)
	assert.True(t, pass.Passed)
	for _, c := range pass.Checks {
		assert.True(t, c.Passed, "pass fixture check %s must pass", c.Name)
	}

	fail, err := RunGate(SyntheticEvidenceFail(), DefaultConfig())
	require.NoError(t, err)
	assert.False(t, fail.Passed)
	for _, c := range fail.Checks {
		assert.False(t, c.Passed, "fail fixture check %s must fail", c.Name)
	}
}

// --- 5.7 In-memory fake store ---

func TestFakeVerdictStore_ReturnsLatestVerdictPerProposal(t *testing.T) {
	store := NewFakeVerdictStore()
	proposalID := uuid.New()
	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	verdict, err := RunGate(SyntheticEvidencePass(), DefaultConfig())
	require.NoError(t, err)
	verdict.ProposalID = proposalID

	earlier := NewOverfittingVerdict(verdict, base)
	later := NewOverfittingVerdict(verdict, base.Add(1*time.Hour))
	require.NoError(t, store.Persist(earlier))
	require.NoError(t, store.Persist(later))

	got, err := store.FetchLatestByProposal(proposalID)
	require.NoError(t, err)
	assert.Equal(t, later.ID, got.ID, "the verdict with the later computed_at must be returned")
}

func TestFakeVerdictStore_NotFound(t *testing.T) {
	store := NewFakeVerdictStore()

	_, err := store.FetchLatestByProposal(uuid.New())

	assert.ErrorIs(t, err, ErrVerdictNotFound)
}

func TestFakeVerdictStore_RejectsInvalidProposalKind(t *testing.T) {
	store := NewFakeVerdictStore()
	row := NewOverfittingVerdict(Verdict{ProposalID: uuid.New(), ProposalKind: "bogus"}, time.Now().UTC())

	err := store.Persist(row)

	assert.ErrorIs(t, err, ErrUnknownProposalKind)
}

// --- 5.6 Persistence (testcontainers) ---

// setupOverfittingDB spins up an ephemeral PostgreSQL container, runs the
// trading-stack-schema migration first (so the exactly-one-new-table
// assertion runs against a database that already has the six v4 tables), and
// then runs MigrateOverfittingCountermeasures. The container is
// auto-terminated on test cleanup.
func setupOverfittingDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "overfitting_test"
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
	require.NoError(t, MigrateOverfittingCountermeasures(db))

	return db
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

func TestMigrateOverfitting_CreatesExactlyOneTableAndIsIdempotent(t *testing.T) {
	db := setupOverfittingDB(t)

	before := listPublicTables(t, db)
	assert.True(t, before["overfitting_verdicts"], "overfitting_verdicts must exist")

	tradingStackTables := []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
	}
	for _, tbl := range tradingStackTables {
		assert.True(t, before[tbl], "trading-stack table %q must be untouched (still present)", tbl)
	}

	playgroundTables := []string{
		"playgrounds", "order_records", "trade_records",
		"equity_plot_records", "live_accounts", "live_account_plots",
	}
	for _, tbl := range playgroundTables {
		assert.False(t, before[tbl], "playground table %q must NOT be created", tbl)
	}

	assert.Len(t, before, len(tradingStackTables)+1,
		"exactly the six trading-stack tables plus overfitting_verdicts must exist")

	// Idempotency: a second run returns nil and leaves the table set unchanged.
	require.NoError(t, MigrateOverfittingCountermeasures(db))
	after := listPublicTables(t, db)
	assert.Equal(t, before, after, "a second migration run must not create, alter, or drop tables")
}

func TestOverfittingVerdict_RoundTripIncludingChecksJSON(t *testing.T) {
	db := setupOverfittingDB(t)
	store := NewGormVerdictStore(db)

	verdict, err := RunGate(SyntheticEvidenceFail(), DefaultConfig())
	require.NoError(t, err)
	require.False(t, verdict.Passed)
	require.Len(t, verdict.Checks, 4)

	computedAt := time.Date(2024, 7, 1, 9, 30, 0, 0, time.UTC)
	row := NewOverfittingVerdict(verdict, computedAt)
	require.NoError(t, store.Persist(row))

	var got OverfittingVerdict
	require.NoError(t, db.First(&got, "id = ?", row.ID).Error)

	assert.Equal(t, row.ProposalID, got.ProposalID)
	assert.Equal(t, string(ProposalKindScanner), got.ProposalKind)
	assert.False(t, got.Passed)
	assert.True(t, got.ComputedAt.Equal(computedAt))
	assert.Equal(t, LibraryVersion, got.LibraryVersion)
	require.Len(t, got.ChecksJSON, 4, "checks_json must deserialize to the same four check results")
	assert.Equal(t, CheckResults(verdict.Checks), got.ChecksJSON)
}

func TestOverfittingVerdict_InvalidProposalKindRejected(t *testing.T) {
	db := setupOverfittingDB(t)

	// Go validation: the BeforeSave hook rejects the row before it reaches
	// the database.
	row := NewOverfittingVerdict(Verdict{ProposalID: uuid.New(), ProposalKind: "bogus"}, time.Now().UTC())
	err := db.Create(&row).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownProposalKind)

	// Database CHECK constraint: a raw insert that bypasses the Go hook is
	// rejected by Postgres itself.
	err = db.Exec(
		`INSERT INTO overfitting_verdicts
			(id, proposal_id, proposal_kind, computed_at, passed, checks_json, library_version)
		 VALUES (?, ?, 'bogus', now(), false, '[]', ?)`,
		uuid.New(), uuid.New(), LibraryVersion,
	).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "chk_overfitting_verdicts_proposal_kind"),
		"expected the CHECK constraint to reject the row, got: %v", err)

	var count int64
	require.NoError(t, db.Model(&OverfittingVerdict{}).Where("proposal_kind = ?", "bogus").Count(&count).Error)
	assert.Zero(t, count, "no invalid row may be written")
}

func TestGormVerdictStore_FetchLatestByProposal(t *testing.T) {
	db := setupOverfittingDB(t)
	store := NewGormVerdictStore(db)

	proposalID := uuid.New()
	base := time.Date(2024, 7, 1, 9, 0, 0, 0, time.UTC)

	verdict, err := RunGate(SyntheticEvidencePass(), DefaultConfig())
	require.NoError(t, err)
	verdict.ProposalID = proposalID

	earlier := NewOverfittingVerdict(verdict, base)
	later := NewOverfittingVerdict(verdict, base.Add(2*time.Hour))
	require.NoError(t, store.Persist(earlier))
	require.NoError(t, store.Persist(later))

	got, err := store.FetchLatestByProposal(proposalID)
	require.NoError(t, err)
	assert.Equal(t, later.ID, got.ID)

	_, err = store.FetchLatestByProposal(uuid.New())
	assert.ErrorIs(t, err, ErrVerdictNotFound)
}
