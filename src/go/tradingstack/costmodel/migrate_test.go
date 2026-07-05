package costmodel

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

// setupCostModelDB spins up an ephemeral PostgreSQL container, opens a GORM
// connection, and runs tradingstack.MigrateTradingStack so
// strategy_ev_weights exists before this package's own migration runs. The
// container is auto-terminated on test cleanup.
func setupCostModelDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "costmodel_test"
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

func columnExists(t *testing.T, db *gorm.DB, table, column string) bool {
	t.Helper()
	var count int64
	err := db.Raw(
		`SELECT count(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&count).Error
	require.NoError(t, err)
	return count == 1
}

// TestMigrateNetEvCostModel_AddsColumnsWithoutDisturbingExistingData pins the
// spec scenario: the migration adds gross_ev, net_ev, and capacity_shares
// without altering any pre-existing row's other fields.
func TestMigrateNetEvCostModel_AddsColumnsWithoutDisturbingExistingData(t *testing.T) {
	db := setupCostModelDB(t)

	row := &tradingstack.StrategyEvWeight{
		StrategyID: strPtr("covered_call_v7"),
		Regime:     strPtr("bull"),
		Ev30d:      floatPtr(0.031),
		Ev90d:      floatPtr(0.028),
		EvSlope:    floatPtr(-0.0004),
		EvWeight:   floatPtr(0.65),
	}
	require.NoError(t, db.Create(row).Error)

	require.NoError(t, MigrateNetEvCostModel(db))

	assert.True(t, columnExists(t, db, "strategy_ev_weights", "gross_ev"))
	assert.True(t, columnExists(t, db, "strategy_ev_weights", "net_ev"))
	assert.True(t, columnExists(t, db, "strategy_ev_weights", "capacity_shares"))

	var got tradingstack.StrategyEvWeight
	require.NoError(t, db.First(&got, "id = ?", row.ID).Error)
	assert.Equal(t, "covered_call_v7", *got.StrategyID)
	assert.Equal(t, "bull", *got.Regime)
	assert.InDelta(t, 0.031, *got.Ev30d, 1e-9)
	assert.InDelta(t, 0.028, *got.Ev90d, 1e-9)
	assert.InDelta(t, -0.0004, *got.EvSlope, 1e-9)
	assert.InDelta(t, 0.65, *got.EvWeight, 1e-9)
}

// TestMigrateNetEvCostModel_IsIdempotent pins the spec scenario: running the
// migration twice in succession is a no-op on the second call, leaving
// exactly one of each new column.
func TestMigrateNetEvCostModel_IsIdempotent(t *testing.T) {
	db := setupCostModelDB(t)

	require.NoError(t, MigrateNetEvCostModel(db))
	require.NoError(t, MigrateNetEvCostModel(db))

	var count int64
	err := db.Raw(
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'strategy_ev_weights' AND column_name IN ('gross_ev', 'net_ev', 'capacity_shares')`,
	).Scan(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestMigrateNetEvCostModel_MissingTableReturnsError asserts the migration
// refuses to run (and does not create the table itself) when
// strategy_ev_weights does not yet exist, i.e. trading-stack-schema's
// migration has not been run first.
func TestMigrateNetEvCostModel_MissingTableReturnsError(t *testing.T) {
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "costmodel_no_table_test"
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

	err = MigrateNetEvCostModel(db)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStrategyEvWeightsTableMissing)
}

func strPtr(s string) *string     { return &s }
func floatPtr(f float64) *float64 { return &f }
