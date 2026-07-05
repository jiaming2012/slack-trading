package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newTestDB boots a disposable postgres:13 container and runs the telemetry
// migration — same harness pattern as the tradingstack suites.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "telemetry_test"
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

	require.NoError(t, Migrate(db))

	return db
}

func TestPersistence(t *testing.T) {
	db := newTestDB(t)

	t.Run("fresh database migrates and first snapshot succeeds", func(t *testing.T) {
		reg := NewRegistry()
		reg.Counter("grodt.orders.placed").Add(3, Label{Key: "mode", Value: "simulator"})
		reg.Gauge("grodt.heartbeat.open_orders").Set(2)

		require.NoError(t, WriteSnapshot(db, reg))

		var rows []MetricRow
		require.NoError(t, db.Find(&rows).Error)
		require.Len(t, rows, 2)
	})

	t.Run("counters persist cumulatively across snapshots", func(t *testing.T) {
		require.NoError(t, db.Where("1=1").Delete(&MetricRow{}).Error)

		reg := NewRegistry()
		c := reg.Counter("test.cumulative")
		c.Add(2)
		require.NoError(t, WriteSnapshot(db, reg))
		c.Add(3)
		require.NoError(t, WriteSnapshot(db, reg))

		var rows []MetricRow
		require.NoError(t, db.Where("name = ?", "test.cumulative").Order("id asc").Find(&rows).Error)
		require.Len(t, rows, 2)
		assert.Equal(t, float64(2), rows[0].Value)
		assert.Equal(t, float64(5), rows[1].Value, "second snapshot holds the non-decreasing cumulative value")
	})

	t.Run("snapshot labels round-trip as JSON", func(t *testing.T) {
		require.NoError(t, db.Where("1=1").Delete(&MetricRow{}).Error)

		reg := NewRegistry()
		reg.Counter("test.labeled").Add(1, Label{Key: "mode", Value: "live"}, Label{Key: "client_id", Value: "cc-v7"})
		require.NoError(t, WriteSnapshot(db, reg))

		var row MetricRow
		require.NoError(t, db.Where("name = ?", "test.labeled").First(&row).Error)

		var labels map[string]string
		require.NoError(t, json.Unmarshal([]byte(row.Labels), &labels))
		assert.Equal(t, map[string]string{"mode": "live", "client_id": "cc-v7"}, labels)
	})

	t.Run("empty registry writes nothing", func(t *testing.T) {
		require.NoError(t, db.Where("1=1").Delete(&MetricRow{}).Error)
		require.NoError(t, WriteSnapshot(db, NewRegistry()))

		var count int64
		require.NoError(t, db.Model(&MetricRow{}).Count(&count).Error)
		assert.Equal(t, int64(0), count)
	})

	t.Run("heartbeat upsert keeps one row per source and counts beats", func(t *testing.T) {
		first := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
		second := time.Now().UTC().Truncate(time.Microsecond)

		require.NoError(t, UpsertHeartbeat(db, "strategy", "covered-call", map[string]string{"state": "idle"}, first))
		require.NoError(t, UpsertHeartbeat(db, "strategy", "covered-call", map[string]string{"state": "active"}, second))
		require.NoError(t, UpsertHeartbeat(db, "datasource", "polygon-options", nil, second))

		var rows []HeartbeatRow
		require.NoError(t, db.Order("source_kind asc").Find(&rows).Error)
		require.Len(t, rows, 2)

		strategyRow := rows[1]
		assert.Equal(t, "strategy", strategyRow.SourceKind)
		assert.Equal(t, int64(2), strategyRow.BeatCount, "second beat increments, not duplicates")
		assert.WithinDuration(t, second, strategyRow.LastSeenAt, time.Second)
		assert.Contains(t, strategyRow.Meta, "active")
	})

	t.Run("prune removes old rows and keeps young ones", func(t *testing.T) {
		require.NoError(t, db.Where("1=1").Delete(&MetricRow{}).Error)

		old := MetricRow{RecordedAt: time.Now().UTC().Add(-31 * 24 * time.Hour), Name: "old", Labels: "{}", Kind: "counter", Value: 1}
		young := MetricRow{RecordedAt: time.Now().UTC().Add(-time.Hour), Name: "young", Labels: "{}", Kind: "counter", Value: 1}
		require.NoError(t, db.Create(&old).Error)
		require.NoError(t, db.Create(&young).Error)

		deleted, err := PruneOnce(db, time.Now().UTC().Add(-30*24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		var names []string
		require.NoError(t, db.Model(&MetricRow{}).Pluck("name", &names).Error)
		assert.Equal(t, []string{"young"}, names)
	})

	t.Run("alert rows persist lifecycle fields", func(t *testing.T) {
		fired := time.Now().UTC().Truncate(time.Microsecond)
		row := AlertRow{Rule: "stale_heartbeat", Subject: "strategy/covered-call", Message: "stale", FiredAt: fired}
		require.NoError(t, db.Create(&row).Error)
		require.NotZero(t, row.ID)

		acked := fired.Add(time.Minute)
		require.NoError(t, db.Model(&AlertRow{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
			"acked_at": acked, "acked_via": "cli",
		}).Error)

		var got AlertRow
		require.NoError(t, db.First(&got, row.ID).Error)
		require.NotNil(t, got.AckedAt)
		assert.Equal(t, "cli", got.AckedVia)
		assert.Nil(t, got.ResolvedAt)
	})
}
