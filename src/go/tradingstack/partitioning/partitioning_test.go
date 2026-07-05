package partitioning

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
	tradingstackdb "github.com/jiaming2012/slack-trading/src/go/tradingstack/db"
)

// setupPartitionedDB spins up an ephemeral Postgres container, runs
// MigrateTradingStack (creates the plain v4 tables) followed by the
// partitioning.sql migration (converts scan_results/sim_outcomes to
// partitioned parents). The container is auto-terminated on test cleanup.
func setupPartitionedDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Tmpfs:        map[string]string{"/var/lib/postgresql/data": "rw"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "partitioning_test",
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

	dsn := fmt.Sprintf("host=%s user=test password=test dbname=partitioning_test port=%s sslmode=disable", host, port.Port())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, tradingstack.MigrateTradingStack(db))
	require.NoError(t, tradingstackdb.ApplyPartitioningMigration(db))

	return db
}

func attachedPartitions(t *testing.T, db *gorm.DB, tableName string) []string {
	t.Helper()
	var names []string
	err := db.Raw(`
		SELECT c.relname FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = ?::regclass
		ORDER BY c.relname
	`, tableName).Scan(&names).Error
	require.NoError(t, err)
	return names
}

// TestEnsureMonthlyPartitionsCreatesHorizon proves 4.1: provisioning with a
// 3-month horizon creates exactly 4 monthly partitions (current + 3 ahead)
// for both scan_results and sim_outcomes, and rows route to the correct
// month's partition.
func TestEnsureMonthlyPartitionsCreatesHorizon(t *testing.T) {
	db := setupPartitionedDB(t)
	ctx := context.Background()

	for _, tbl := range []struct {
		table string
		col   string
	}{
		{"scan_results", "scanned_at"},
		{"sim_outcomes", "simulated_at"},
	} {
		created, err := EnsureMonthlyPartitions(ctx, db, tbl.table, tbl.col, 3)
		require.NoError(t, err)
		assert.Len(t, created, 4, "expected current month + 3 ahead for %s", tbl.table)

		attached := attachedPartitions(t, db, tbl.table)
		assert.GreaterOrEqual(t, len(attached), 4, "expected at least 4 partitions attached to %s", tbl.table)
	}

	// Insert one row per month into scan_results across 3 distinct months and
	// assert each lands in the expected partition via tableoid::regclass.
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0).AddDate(0, 0, 5)
		var partOID string
		err := db.Raw(`
			INSERT INTO scan_results (id, scanned_at, ticker)
			VALUES (gen_random_uuid(), ?, ?)
			RETURNING tableoid::regclass::text
		`, month, fmt.Sprintf("SYM%d", i)).Scan(&partOID).Error
		require.NoError(t, err)

		wantPart := partitionName("scan_results", time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC))
		assert.Equal(t, wantPart, partOID, "row for month offset %d should land in %s", i, wantPart)
	}

	// Same for sim_outcomes, keyed on simulated_at.
	for i := 0; i < 3; i++ {
		month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0).AddDate(0, 0, 5)
		var partOID string
		err := db.Raw(`
			INSERT INTO sim_outcomes (id, simulated_at, exit_reason)
			VALUES (gen_random_uuid(), ?, 'stop')
			RETURNING tableoid::regclass::text
		`, month).Scan(&partOID).Error
		require.NoError(t, err)

		wantPart := partitionName("sim_outcomes", time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC))
		assert.Equal(t, wantPart, partOID, "row for month offset %d should land in %s", i, wantPart)
	}
}

// TestEnsureMonthlyPartitionsIdempotent proves 4.1/2.2: re-running with no
// elapsed time creates zero new partitions and does not error.
func TestEnsureMonthlyPartitionsIdempotent(t *testing.T) {
	db := setupPartitionedDB(t)
	ctx := context.Background()

	_, err := EnsureMonthlyPartitions(ctx, db, "scan_results", "scanned_at", 3)
	require.NoError(t, err)
	before := attachedPartitions(t, db, "scan_results")

	_, err = EnsureMonthlyPartitions(ctx, db, "scan_results", "scanned_at", 3)
	require.NoError(t, err)
	after := attachedPartitions(t, db, "scan_results")

	assert.Equal(t, before, after, "second run must not create additional partitions")
}

// TestInsertIntoUnprovisionedMonthFailsLoudly proves 4.2 and the "No Catch-All
// Default Partition" requirement: an insert into a month with no provisioned
// partition fails with Postgres's native error, and no row is written to any
// partition.
func TestInsertIntoUnprovisionedMonthFailsLoudly(t *testing.T) {
	db := setupPartitionedDB(t)
	ctx := context.Background()

	_, err := EnsureMonthlyPartitions(ctx, db, "scan_results", "scanned_at", 1)
	require.NoError(t, err)

	farFuture := time.Now().UTC().AddDate(2, 0, 0)
	err = db.Exec(`INSERT INTO scan_results (id, scanned_at, ticker) VALUES (gen_random_uuid(), ?, 'AAPL')`, farFuture).Error
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no partition of relation"), "expected Postgres no-partition error, got: %v", err)

	var count int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM scan_results WHERE scanned_at = ?`, farFuture).Scan(&count).Error)
	assert.Equal(t, int64(0), count, "no row should be written for an unprovisioned month")
}

// TestEnsureMonthlyPartitionsRejectsBadInputs covers constructor-style
// validation.
func TestEnsureMonthlyPartitionsRejectsBadInputs(t *testing.T) {
	db := setupPartitionedDB(t)
	ctx := context.Background()

	_, err := EnsureMonthlyPartitions(ctx, nil, "scan_results", "scanned_at", 1)
	assert.ErrorIs(t, err, ErrNilDB)

	_, err = EnsureMonthlyPartitions(ctx, db, "", "scanned_at", 1)
	assert.ErrorIs(t, err, ErrEmptyTableName)

	_, err = EnsureMonthlyPartitions(ctx, db, "scan_results; DROP TABLE scan_results", "scanned_at", 1)
	assert.ErrorIs(t, err, ErrInvalidIdentifier)

	_, err = EnsureMonthlyPartitions(ctx, db, "scan_results", "scanned_at", -1)
	assert.ErrorIs(t, err, ErrNegativeHorizon)

	_, err = EnsureMonthlyPartitions(ctx, db, "scan_results", "wrong_column", 1)
	assert.ErrorIs(t, err, ErrPartitionKeyMismatch)
}
