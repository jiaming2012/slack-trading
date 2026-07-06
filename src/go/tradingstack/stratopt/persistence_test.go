package stratopt

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
// trading-stack-schema migration. The overfitting and strategy-optimizer
// migrations are left to each test so migration-ordering behavior stays
// testable. The container is auto-terminated on test cleanup.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "stratopt_test"
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

// migrateAll runs the overfitting and strategy-optimizer migrations in their
// documented order.
func migrateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, overfitting.MigrateOverfittingCountermeasures(db))
	require.NoError(t, MigrateStrategyOptimizer(db))
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

// persistTestVerdict writes one verdict row and returns it, so proposal rows
// under test can hold a valid verdict_id FK.
func persistTestVerdict(t *testing.T, db *gorm.DB, passed bool) overfitting.OverfittingVerdict {
	t.Helper()
	row := overfitting.NewOverfittingVerdict(overfitting.Verdict{
		ProposalID:   uuid.New(),
		ProposalKind: overfitting.ProposalKindStrategy,
		Passed:       passed,
	}, time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, overfitting.NewGormVerdictStore(db).Persist(row))
	return row
}

// testProposal builds a persistable proposal row referencing verdictID.
func testProposal(verdictID uuid.UUID, status string) StrategyProposal {
	return StrategyProposal{
		ID:            uuid.New(),
		CreatedAt:     time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		RunID:         uuid.New(),
		StrategyID:    "strat-alpha",
		Regime:        "trend",
		Parameter:     ParamMaxHoldDays,
		AdjustmentPct: -0.20,
		Rule:          RuleTimeoutDrag,
		Rationale:     "timeout losses dominate -- shorten max_hold_days by -20% (relative)",
		EvidenceJSON:  []byte(`{"weighted_ev":0.0033,"trigger_share":0.7,"decided_samples":88,"gating_summary":{"input_rows":100,"timestamp_violations":2,"dropped_by_regime":5,"dropped_by_fidelity":5,"clean_rows":88,"gated_outcomes":88}}`),
		VerdictID:     verdictID,
		Status:        status,
	}
}

// TestMigrateStrategyOptimizer_OrderingCreatesExactlyOneTableAndIsIdempotent
// covers three migration requirements in one container: the migration
// refuses to run before the overfitting migration (naming the missing
// dependency), creates exactly strategy_proposals once its FK target exists,
// and is idempotent and additive-only.
func TestMigrateStrategyOptimizer_OrderingCreatesExactlyOneTableAndIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	// Fresh DB without overfitting_verdicts: the FK target is missing.
	err := MigrateStrategyOptimizer(db)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingVerdictTable)
	assert.Contains(t, err.Error(), "overfitting_verdicts")

	migrateAll(t, db)

	before := listPublicTables(t, db)
	assert.True(t, before["strategy_proposals"], "strategy_proposals must exist")

	tradingStackTables := []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
	}
	for _, tbl := range tradingStackTables {
		assert.True(t, before[tbl], "trading-stack table %q must be untouched (still present)", tbl)
	}
	assert.True(t, before["overfitting_verdicts"])
	assert.Len(t, before, len(tradingStackTables)+2,
		"exactly the six trading-stack tables, overfitting_verdicts, and strategy_proposals must exist")

	// Idempotency: a second run returns nil and leaves the table set unchanged.
	require.NoError(t, MigrateStrategyOptimizer(db))
	after := listPublicTables(t, db)
	assert.Equal(t, before, after)
}

// TestStrategyProposal_RoundTripFieldForField: a proposal with evidence JSON,
// a signed negative adjustment, and a verdict reference re-reads exactly as
// written.
func TestStrategyProposal_RoundTripFieldForField(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	verdict := persistTestVerdict(t, db, true)
	written := testProposal(verdict.ID, StatusPendingReview)
	require.NoError(t, PersistRun(db, written.RunID, []StrategyProposal{written}))

	read, err := GetProposal(db, written.ID)
	require.NoError(t, err)

	assert.Equal(t, written.ID, read.ID)
	assert.True(t, written.CreatedAt.Equal(read.CreatedAt))
	assert.Equal(t, written.RunID, read.RunID)
	assert.Equal(t, written.StrategyID, read.StrategyID)
	assert.Equal(t, written.Regime, read.Regime)
	assert.Equal(t, written.Parameter, read.Parameter)
	assert.InDelta(t, -0.20, read.AdjustmentPct, 1e-12, "the signed negative adjustment must round-trip")
	assert.Equal(t, written.Rule, read.Rule)
	assert.Equal(t, written.Rationale, read.Rationale)
	assert.Equal(t, verdict.ID, read.VerdictID)
	assert.Equal(t, StatusPendingReview, read.Status)
	assert.Nil(t, read.DecidedAt)
	assert.Nil(t, read.DecidedVia)

	evidence, err := UnmarshalProposalEvidence(read.EvidenceJSON)
	require.NoError(t, err)
	assert.InDelta(t, 0.0033, evidence.WeightedEV, 1e-12)
	assert.InDelta(t, 0.7, evidence.TriggerShare, 1e-12)
	assert.Equal(t, 88, evidence.DecidedSamples)
	assert.Equal(t, GatingSummary{
		InputRows:           100,
		TimestampViolations: 2,
		DroppedByRegime:     5,
		DroppedByFidelity:   5,
		CleanRows:           88,
		GatedOutcomes:       88,
	}, evidence.GatingSummary)
}

// TestStrategyProposal_InvalidStatusAndOrphanVerdictRejected: the Go hook and
// the database CHECK reject a status outside the vocabulary, and the FK
// rejects a verdict_id referencing no persisted verdict -- in every case no
// row is written.
func TestStrategyProposal_InvalidStatusAndOrphanVerdictRejected(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	verdict := persistTestVerdict(t, db, true)

	// Go hook.
	bad := testProposal(verdict.ID, "promoted")
	err := db.Create(&bad).Error
	require.Error(t, err, "the Go hook must reject a status outside the vocabulary")
	assert.ErrorIs(t, err, ErrInvalidProposalStatus)

	// Database CHECK, bypassing the hook.
	err = db.Exec(
		`INSERT INTO strategy_proposals
			(id, created_at, run_id, strategy_id, regime, parameter, adjustment_pct, rule, rationale, evidence_json, verdict_id, status)
		 VALUES (?, now(), ?, 's', 'r', 'stop_pct', 0.2, 'stop_churn', 'x', '{}', ?, 'promoted')`,
		uuid.New(), uuid.New(), verdict.ID,
	).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "chk_strategy_proposals_status"),
		"expected the CHECK constraint to reject the row, got: %v", err)

	// FK: a proposal referencing a nonexistent verdict fails.
	orphan := testProposal(uuid.New(), StatusPendingReview)
	err = db.Create(&orphan).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "fk_strategy_proposals_verdict"),
		"expected the FK constraint to reject the row, got: %v", err)

	var count int64
	require.NoError(t, db.Model(&StrategyProposal{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no rejected row may be written")
}

// TestDecideProposal_SemanticsAndRefusals covers the decide spec scenarios in
// one container: accepted and dismissed happy paths, double-decide refusal,
// decide-on-rejected_by_gate refusal, unknown id, and invalid decision -- all
// refusals leaving the rows unchanged.
func TestDecideProposal_SemanticsAndRefusals(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	passVerdict := persistTestVerdict(t, db, true)
	failVerdict := persistTestVerdict(t, db, false)

	pendingA := testProposal(passVerdict.ID, StatusPendingReview)
	pendingB := testProposal(passVerdict.ID, StatusPendingReview)
	rejected := testProposal(failVerdict.ID, StatusRejectedByGate)
	require.NoError(t, PersistRun(db, uuid.New(), []StrategyProposal{pendingA, pendingB, rejected}))

	decidedAt := time.Date(2024, 7, 2, 8, 0, 0, 0, time.UTC)

	// Happy path: accepted.
	decided, err := DecideProposal(db, pendingA.ID, DecisionAccepted, "cli", decidedAt)
	require.NoError(t, err)
	assert.Equal(t, StatusAccepted, decided.Status)
	require.NotNil(t, decided.DecidedAt)
	assert.True(t, decidedAt.Equal(*decided.DecidedAt))
	require.NotNil(t, decided.DecidedVia)
	assert.Equal(t, "cli", *decided.DecidedVia)

	// Double decide: refused, row unchanged.
	_, err = DecideProposal(db, pendingA.ID, DecisionDismissed, "cli", decidedAt.Add(time.Hour))
	assert.ErrorIs(t, err, ErrProposalNotDecidable)
	still, err := GetProposal(db, pendingA.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusAccepted, still.Status)
	assert.True(t, decidedAt.Equal(*still.DecidedAt), "a refused decide must change nothing")

	// Happy path: dismissed.
	decided, err = DecideProposal(db, pendingB.ID, DecisionDismissed, "cli", decidedAt)
	require.NoError(t, err)
	assert.Equal(t, StatusDismissed, decided.Status)

	// rejected_by_gate: never decidable.
	_, err = DecideProposal(db, rejected.ID, DecisionAccepted, "cli", decidedAt)
	assert.ErrorIs(t, err, ErrProposalNotDecidable)
	stillRejected, err := GetProposal(db, rejected.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRejectedByGate, stillRejected.Status)
	assert.Nil(t, stillRejected.DecidedAt)

	// Unknown id.
	_, err = DecideProposal(db, uuid.New(), DecisionAccepted, "cli", decidedAt)
	assert.ErrorIs(t, err, ErrProposalNotFound)

	// Invalid decision, before any row is touched.
	_, err = DecideProposal(db, pendingA.ID, "promoted", "cli", decidedAt)
	assert.ErrorIs(t, err, ErrInvalidDecision)
}

// TestListProposals_DefaultQueueAndExplicitFilter: the default listing is the
// pending_review queue; rejected_by_gate rows appear only under the explicit
// status filter; terminal rows appear under theirs.
func TestListProposals_DefaultQueueAndExplicitFilter(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	passVerdict := persistTestVerdict(t, db, true)
	failVerdict := persistTestVerdict(t, db, false)

	pendingA := testProposal(passVerdict.ID, StatusPendingReview)
	pendingB := testProposal(passVerdict.ID, StatusPendingReview)
	rejected := testProposal(failVerdict.ID, StatusRejectedByGate)
	dismissed := testProposal(passVerdict.ID, StatusPendingReview)
	require.NoError(t, PersistRun(db, uuid.New(), []StrategyProposal{pendingA, pendingB, rejected, dismissed}))
	_, err := DecideProposal(db, dismissed.ID, DecisionDismissed, "cli", time.Now().UTC())
	require.NoError(t, err)

	// Default (empty filter) -> pending_review queue only.
	queue, err := ListProposals(db, "")
	require.NoError(t, err)
	require.Len(t, queue, 2)
	ids := map[uuid.UUID]bool{}
	for _, p := range queue {
		ids[p.ID] = true
		assert.Equal(t, StatusPendingReview, p.Status,
			"gate-rejected and decided rows must be absent from the default listing")
	}
	assert.True(t, ids[pendingA.ID])
	assert.True(t, ids[pendingB.ID])

	// Explicit rejected_by_gate filter surfaces the audit rows.
	audit, err := ListProposals(db, StatusRejectedByGate)
	require.NoError(t, err)
	require.Len(t, audit, 1)
	assert.Equal(t, rejected.ID, audit[0].ID)

	// Terminal filter.
	done, err := ListProposals(db, StatusDismissed)
	require.NoError(t, err)
	require.Len(t, done, 1)
	assert.Equal(t, dismissed.ID, done[0].ID)

	// An invalid filter is refused.
	_, err = ListProposals(db, "promoted")
	assert.ErrorIs(t, err, ErrInvalidProposalStatus)
}

// TestGenerateRun_DBPersistenceWritesOnlyProposalAndVerdictRows: a full
// synthetic generate-and-persist run against a migrated database writes
// exactly its strategy_proposals rows and their overfitting_verdicts rows --
// and nothing else.
func TestGenerateRun_DBPersistenceWritesOnlyProposalAndVerdictRows(t *testing.T) {
	db := newTestDB(t)
	migrateAll(t, db)

	countAll := func() map[string]int64 {
		tables := []string{
			"scan_results", "sim_outcomes", "simulator_fidelity",
			"strategy_ev_weights", "scanner_configs", "feature_distributions",
			"overfitting_verdicts", "strategy_proposals",
		}
		counts := make(map[string]int64, len(tables))
		for _, tbl := range tables {
			var n int64
			require.NoError(t, db.Table(tbl).Count(&n).Error)
			counts[tbl] = n
		}
		return counts
	}

	before := countAll()

	input, outcomes := SyntheticDataset()
	result, err := GenerateRun(input, outcomes, SyntheticRunConfig(), overfitting.NewGormVerdictStore(db))
	require.NoError(t, err)
	require.Len(t, result.Proposals, 2)
	require.NoError(t, PersistRun(db, result.RunID, result.ProposalRows()))

	after := countAll()
	assert.Equal(t, before["strategy_proposals"]+2, after["strategy_proposals"])
	assert.Equal(t, before["overfitting_verdicts"]+2, after["overfitting_verdicts"])
	for _, tbl := range []string{
		"scan_results", "sim_outcomes", "simulator_fidelity",
		"strategy_ev_weights", "scanner_configs", "feature_distributions",
	} {
		assert.Equal(t, before[tbl], after[tbl], "table %q must be untouched by a generation run", tbl)
	}

	// The persisted rows carry their statuses and verdict references.
	stored, err := ListProposals(db, StatusPendingReview)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "strat-alpha", stored[0].StrategyID)

	var verdictRow overfitting.OverfittingVerdict
	require.NoError(t, db.First(&verdictRow, "id = ?", stored[0].VerdictID).Error)
	assert.True(t, verdictRow.Passed)
	assert.Equal(t, string(overfitting.ProposalKindStrategy), verdictRow.ProposalKind)

	auditRows, err := ListProposals(db, StatusRejectedByGate)
	require.NoError(t, err)
	require.Len(t, auditRows, 1)
	var failedVerdictRow overfitting.OverfittingVerdict
	require.NoError(t, db.First(&failedVerdictRow, "id = ?", auditRows[0].VerdictID).Error)
	assert.False(t, failedVerdictRow.Passed)
}
