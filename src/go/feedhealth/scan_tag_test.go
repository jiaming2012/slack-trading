package feedhealth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// TestTagScanResultHealthy verifies the tagging helper sets FeedHealth to
// "healthy" for a healthy status.
func TestTagScanResultHealthy(t *testing.T) {
	sr := &tradingstack.ScanResult{}
	TagScanResult(sr, Healthy)

	require.NotNil(t, sr.FeedHealth)
	assert.Equal(t, "healthy", *sr.FeedHealth)
}

// TestTagScanResultDegraded verifies the tagging helper sets FeedHealth to
// "degraded" for a degraded status.
func TestTagScanResultDegraded(t *testing.T) {
	sr := &tradingstack.ScanResult{}
	TagScanResult(sr, Degraded)

	require.NotNil(t, sr.FeedHealth)
	assert.Equal(t, "degraded", *sr.FeedHealth)
}

// TestTagScanResultStale verifies the tagging helper sets FeedHealth to
// "stale" for a stale status.
func TestTagScanResultStale(t *testing.T) {
	sr := &tradingstack.ScanResult{}
	TagScanResult(sr, Stale)

	require.NotNil(t, sr.FeedHealth)
	assert.Equal(t, "stale", *sr.FeedHealth)
}

// setupTradingStackDB spins up an ephemeral PostgreSQL container, opens a
// GORM connection, and runs tradingstack.MigrateTradingStack. The container
// is auto-terminated on test cleanup. Mirrors the tradingstack package's own
// test helper since setupTradingStackDB there is unexported.
func setupTradingStackDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "feedhealth_test"
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

// TestFeedHealthMigrationAdditiveOnly verifies the feed_health column
// migration does not alter any pre-existing scan_results column or
// constraint, and adds a nullable feed_health column.
func TestFeedHealthMigrationAdditiveOnly(t *testing.T) {
	db := setupTradingStackDB(t)

	var columns []struct {
		ColumnName string
		IsNullable string
	}
	require.NoError(t, db.Raw(
		`SELECT column_name AS column_name, is_nullable AS is_nullable
		 FROM information_schema.columns
		 WHERE table_name = 'scan_results'`,
	).Scan(&columns).Error)

	present := make(map[string]string, len(columns))
	for _, c := range columns {
		present[c.ColumnName] = c.IsNullable
	}

	wantPreExisting := []string{
		"id", "scanned_at", "ticker", "regime_tag", "regime_confidence",
		"price", "volume_ratio", "rsi_14", "atr_pct", "short_interest",
		"sector", "scanner_score", "scanner_version", "data_as_of",
	}
	for _, col := range wantPreExisting {
		_, ok := present[col]
		assert.True(t, ok, "pre-existing column %q must remain after migration", col)
	}

	nullable, ok := present["feed_health"]
	require.True(t, ok, "feed_health column should be added")
	assert.Equal(t, "YES", nullable, "feed_health column should be nullable")

	// The pre-existing CHECK constraint on data_as_of must still be intact.
	var constraintCount int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM pg_constraint WHERE conname = 'chk_scan_results_data_as_of'`,
	).Scan(&constraintCount).Error)
	assert.Equal(t, int64(1), constraintCount, "pre-existing data_as_of CHECK constraint must be unchanged")

	// Round-trip a tagged ScanResult through the extended schema.
	now := time.Now().UTC().Truncate(time.Microsecond)
	sr := &tradingstack.ScanResult{ScannedAt: now, Ticker: "AAPL"}
	TagScanResult(sr, Degraded)
	require.NoError(t, db.Create(sr).Error)

	var got tradingstack.ScanResult
	require.NoError(t, db.First(&got, "id = ?", sr.ID).Error)
	require.NotNil(t, got.FeedHealth)
	assert.Equal(t, "degraded", *got.FeedHealth)
}
