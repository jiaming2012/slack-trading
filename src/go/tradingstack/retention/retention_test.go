package retention

import (
	"context"
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

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	tradingstackdb "github.com/jiaming2012/slack-trading/src/go/tradingstack/db"
	"github.com/jiaming2012/slack-trading/src/go/tradingstack/partitioning"
)

// setupRetentionDB spins up an ephemeral Postgres container, runs
// MigrateTradingStack + the partitioning migration, and provisions the
// current month's partitions for both v4 tables via EnsureMonthlyPartitions.
func setupRetentionDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Tmpfs:        map[string]string{"/var/lib/postgresql/data": "rw"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "retention_test",
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

	dsn := fmt.Sprintf("host=%s user=test password=test dbname=retention_test port=%s sslmode=disable", host, port.Port())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, tradingstack.MigrateTradingStack(db))
	require.NoError(t, tradingstackdb.ApplyPartitioningMigration(db))
	_, err = partitioning.EnsureMonthlyPartitions(ctx, db, "scan_results", "scanned_at", 1)
	require.NoError(t, err)
	_, err = partitioning.EnsureMonthlyPartitions(ctx, db, "sim_outcomes", "simulated_at", 1)
	require.NoError(t, err)

	return db
}

// manufactureOldPartition creates and attaches a partition covering a
// calendar month `monthsAgo` months before now, and inserts one row into it
// directly through the parent table.
func manufactureOldPartition(t *testing.T, db *gorm.DB, tableName, timeCol string, monthsAgo int) string {
	t.Helper()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -monthsAgo, 0)
	monthEnd := monthStart.AddDate(0, 1, 0)
	partName := fmt.Sprintf("%s_y%04d_m%02d", tableName, monthStart.Year(), int(monthStart.Month()))

	stmt := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		partName, tableName, monthStart.Format("2006-01-02"), monthEnd.Format("2006-01-02"),
	)
	require.NoError(t, db.Exec(stmt).Error)

	rowTime := monthStart.Add(24 * time.Hour)
	switch tableName {
	case "scan_results":
		require.NoError(t, db.Exec(
			`INSERT INTO scan_results (id, scanned_at, ticker) VALUES (gen_random_uuid(), ?, 'OLD')`, rowTime,
		).Error)
	case "sim_outcomes":
		require.NoError(t, db.Exec(
			`INSERT INTO sim_outcomes (id, simulated_at, exit_reason) VALUES (gen_random_uuid(), ?, 'stop')`, rowTime,
		).Error)
	default:
		t.Fatalf("manufactureOldPartition: unsupported table %s", tableName)
	}

	return partName
}

// countArchiver records how many times Archive is invoked and for which
// partition names, without doing anything else.
type countingArchiver struct {
	calls []string
}

func (a *countingArchiver) Archive(_ context.Context, partitionName string) error {
	a.calls = append(a.calls, partitionName)
	return nil
}

// TestFindEligiblePartitionsRespectsWindow proves the retention-threshold
// requirement: a partition older than the window is flagged, one within the
// window is not.
func TestFindEligiblePartitionsRespectsWindow(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()

	oldPart := manufactureOldPartition(t, db, "scan_results", "scanned_at", 13)
	recentPart := manufactureOldPartition(t, db, "scan_results", "scanned_at", 11)

	eligible, err := FindEligiblePartitions(ctx, db, "scan_results", 12*30*24*time.Hour)
	require.NoError(t, err)

	names := make([]string, 0, len(eligible))
	for _, p := range eligible {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, oldPart, "partition older than the retention window must be eligible")
	assert.NotContains(t, names, recentPart, "partition within the retention window must not be eligible")
}

// TestRetentionDryRunReportsWithoutDetaching proves 4.3 (dry-run half): the
// dry run reports the eligible partition without detaching it.
func TestRetentionDryRunReportsWithoutDetaching(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()

	oldPart := manufactureOldPartition(t, db, "scan_results", "scanned_at", 13)

	reports, err := RunDryRun(ctx, db, DefaultRetentionWindow)
	require.NoError(t, err)

	found := false
	for _, r := range reports {
		if r.TableName != "scan_results" {
			continue
		}
		for _, p := range r.Eligible {
			if p.Name == oldPart {
				found = true
			}
		}
	}
	assert.True(t, found, "dry run should report the old partition as eligible")

	// still attached afterward
	var stillAttached bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
			WHERE i.inhparent = 'scan_results'::regclass AND c.relname = ?
		)`, oldPart).Scan(&stillAttached).Error
	require.NoError(t, err)
	assert.True(t, stillAttached, "dry run must not detach the partition")
}

// TestRetentionLiveRunDetachesAndPreservesData proves 4.3 (live half): the
// live run detaches the eligible partition; the parent table no longer
// returns its rows, but the detached table still does, unchanged.
func TestRetentionLiveRunDetachesAndPreservesData(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()

	oldPart := manufactureOldPartition(t, db, "scan_results", "scanned_at", 13)

	archiver := &countingArchiver{}
	reports, err := RunLive(ctx, db, archiver, DefaultRetentionWindow)
	require.NoError(t, err)

	var detachedThisTable []string
	for _, r := range reports {
		if r.TableName == "scan_results" {
			detachedThisTable = r.Detached
		}
	}
	assert.Contains(t, detachedThisTable, oldPart)
	assert.Contains(t, archiver.calls, oldPart, "archiver should be invoked for the detached partition")

	// Parent table query for that old row now returns zero rows.
	var countViaParent int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM scan_results WHERE ticker = 'OLD'`).Scan(&countViaParent).Error)
	assert.Equal(t, int64(0), countViaParent, "detached partition's rows must no longer surface via the parent table")

	// Detached table, queried directly by name, still has its original row.
	var countDirect int64
	require.NoError(t, db.Raw(fmt.Sprintf(`SELECT count(*) FROM %s WHERE ticker = 'OLD'`, oldPart)).Scan(&countDirect).Error)
	assert.Equal(t, int64(1), countDirect, "detached partition's data must survive, queryable directly")
}

// TestRetentionLiveRunIsIdempotent proves 4.4: running the live job twice in
// a row detaches/archives each eligible partition at most once.
func TestRetentionLiveRunIsIdempotent(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()

	oldPart := manufactureOldPartition(t, db, "scan_results", "scanned_at", 13)

	archiver := &countingArchiver{}
	_, err := RunLive(ctx, db, archiver, DefaultRetentionWindow)
	require.NoError(t, err)
	require.Contains(t, archiver.calls, oldPart)
	firstRunCallCount := len(archiver.calls)

	reports, err := RunLive(ctx, db, archiver, DefaultRetentionWindow)
	require.NoError(t, err)

	for _, r := range reports {
		assert.Empty(t, r.Detached, "second run must find zero newly-eligible partitions for %s", r.TableName)
	}
	assert.Len(t, archiver.calls, firstRunCallCount, "archiver must not be invoked again for an already-detached partition")
}

// TestRetentionNeverTouchesSmallAnalyticalTables proves 3.5: the retention
// job's table list never includes feature_distributions or
// strategy_ev_weights.
func TestRetentionNeverTouchesSmallAnalyticalTables(t *testing.T) {
	for _, t2 := range PartitionedTables {
		assert.NotEqual(t, "feature_distributions", t2.TableName)
		assert.NotEqual(t, "strategy_ev_weights", t2.TableName)
	}

	db := setupRetentionDB(t)
	ctx := context.Background()

	fd := map[string]any{"id": uuid.New(), "computed_at": time.Now().UTC(), "feature_name": "rsi_14"}
	require.NoError(t, db.WithContext(ctx).Table("feature_distributions").Create(fd).Error)

	var before int64
	require.NoError(t, db.Table("feature_distributions").Count(&before).Error)

	archiver := &countingArchiver{}
	_, err := RunLive(ctx, db, archiver, DefaultRetentionWindow)
	require.NoError(t, err)

	var after int64
	require.NoError(t, db.Table("feature_distributions").Count(&after).Error)
	assert.Equal(t, before, after, "retention job must never touch feature_distributions")
}

// TestRunLiveRejectsNilArchiver covers constructor-style validation.
func TestRunLiveRejectsNilArchiver(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()

	_, err := RunLive(ctx, db, nil, DefaultRetentionWindow)
	assert.ErrorIs(t, err, ErrNilArchiver)
}
