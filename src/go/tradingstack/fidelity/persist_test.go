package fidelity

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

// newFidelityTestDB boots a disposable postgres:13 container and opens a GORM
// connection — same harness pattern as the tradingstack suites. It runs NO
// migrations; each test applies exactly the migrations it is about.
func newFidelityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	const (
		user   = "test"
		pass   = "test"
		dbName = "fidelity_test"
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

	return db
}

func publicTables(t *testing.T, db *gorm.DB) map[string]bool {
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

func fidelityRowCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&tradingstack.SimulatorFidelity{}).Count(&count).Error)
	return count
}

// nowMicro returns a UTC time truncated to timestamptz resolution so
// round-trips compare equal.
func nowMicro() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func TestMigrateFidelityMonitoring_MissingBaseTable(t *testing.T) {
	db := newFidelityTestDB(t)
	err := MigrateFidelityMonitoring(db)
	require.ErrorIs(t, err, ErrSimulatorFidelityTableMissing)
}

func TestFidelityPersistence(t *testing.T) {
	db := newFidelityTestDB(t)
	require.NoError(t, tradingstack.MigrateTradingStack(db))
	require.NoError(t, MigrateFidelityMonitoring(db))

	t.Run("migration creates only the unique index, idempotently", func(t *testing.T) {
		before := publicTables(t, db)

		require.NoError(t, MigrateFidelityMonitoring(db), "second run must be a no-op")

		var indexCount int64
		require.NoError(t, db.Raw(
			`SELECT count(*) FROM pg_indexes WHERE tablename = 'simulator_fidelity' AND indexname = ?`,
			fidelityPeriodUniqueIndex,
		).Scan(&indexCount).Error)
		assert.Equal(t, int64(1), indexCount, "unique index must exist exactly once")

		after := publicTables(t, db)
		assert.Equal(t, before, after, "migration must not create or drop any table")

		for _, tbl := range []string{
			"playgrounds", "order_records", "trade_records",
			"equity_plot_records", "live_accounts", "live_account_plots",
		} {
			assert.False(t, after[tbl], "playground table %q must NOT exist", tbl)
		}
	})

	period := Period{
		Start: time.Date(2026, time.June, 28, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC),
	}

	t.Run("persisted rows round-trip field-for-field", func(t *testing.T) {
		computedAt := nowMicro()
		results := []Result{
			{StrategyID: "s1", Period: period, DriftPnL: 12.5, DriftFill: -0.75, DriftScore: 0.33, WithinTolerance: false},
			{StrategyID: "s2", Period: period, DriftPnL: 0.0, DriftFill: 0.0, DriftScore: 0.0, WithinTolerance: true},
		}
		require.NoError(t, Persist(db, results, computedAt))
		require.Equal(t, int64(2), fidelityRowCount(t, db), "exactly one row per strategy")

		for _, want := range results {
			var got tradingstack.SimulatorFidelity
			require.NoError(t, db.Where("strategy_id = ?", want.StrategyID).First(&got).Error)
			require.NotNil(t, got.StrategyID)
			assert.Equal(t, want.StrategyID, *got.StrategyID)
			require.NotNil(t, got.PeriodStart)
			assert.True(t, want.Period.Start.Equal(*got.PeriodStart))
			require.NotNil(t, got.PeriodEnd)
			assert.True(t, want.Period.End.Equal(*got.PeriodEnd))
			require.NotNil(t, got.DriftPnl)
			assert.Equal(t, want.DriftPnL, *got.DriftPnl)
			require.NotNil(t, got.DriftFill)
			assert.Equal(t, want.DriftFill, *got.DriftFill)
			require.NotNil(t, got.DriftScore)
			assert.Equal(t, want.DriftScore, *got.DriftScore)
			assert.Equal(t, want.WithinTolerance, got.WithinTolerance)
			require.NotNil(t, got.ComputedAt)
			assert.True(t, computedAt.Equal(*got.ComputedAt))
		}
	})

	t.Run("re-persisting the same period updates instead of duplicating", func(t *testing.T) {
		firstAt := nowMicro().Add(-time.Hour)
		secondAt := nowMicro()

		require.NoError(t, Persist(db, []Result{
			{StrategyID: "s-rerun", Period: period, DriftPnL: 1.0, DriftFill: 0.1, DriftScore: 0.05, WithinTolerance: true},
		}, firstAt))
		require.NoError(t, Persist(db, []Result{
			{StrategyID: "s-rerun", Period: period, DriftPnL: 9.0, DriftFill: 0.9, DriftScore: 0.44, WithinTolerance: false},
		}, secondAt))

		var rows []tradingstack.SimulatorFidelity
		require.NoError(t, db.Where("strategy_id = ?", "s-rerun").Find(&rows).Error)
		require.Len(t, rows, 1, "same (strategy, period) must upsert, not duplicate")

		got := rows[0]
		require.NotNil(t, got.DriftScore)
		assert.Equal(t, 0.44, *got.DriftScore, "second persist's values win")
		require.NotNil(t, got.DriftPnl)
		assert.Equal(t, 9.0, *got.DriftPnl)
		require.NotNil(t, got.DriftFill)
		assert.Equal(t, 0.9, *got.DriftFill)
		assert.False(t, got.WithinTolerance)
		require.NotNil(t, got.ComputedAt)
		assert.True(t, secondAt.Equal(*got.ComputedAt), "computed_at is refreshed on re-persist")
	})

	t.Run("a monitor no_data run persists nothing", func(t *testing.T) {
		before := fidelityRowCount(t, db)

		m, err := NewMonitor(MonitorOptions{Source: NoLiveTradesSource{}, DB: db})
		require.NoError(t, err)
		report := m.RunOnce(context.Background(), nowMicro())

		assert.Equal(t, RunNoData, report.Outcome)
		assert.Equal(t, before, fidelityRowCount(t, db), "no_data must write no simulator_fidelity row")
	})

	t.Run("a monitor result-producing run persists one row per strategy and upserts on re-run", func(t *testing.T) {
		// Finite breaching fixture: zero-drift (Proceed) + exit-mismatch
		// (Pause). ExtremeDriftSet is deliberately excluded — its infinite
		// drift values exercise score clamping in memory but are not
		// representable as Postgres numerics.
		var set TradeSet
		for _, s := range []TradeSet{ZeroDriftSet(), ExitReasonMismatchSet()} {
			set.Live = append(set.Live, s.Live...)
			set.Sim = append(set.Sim, s.Sim...)
		}
		m, err := NewMonitor(MonitorOptions{Source: StaticTradeSource{Set: set}, DB: db})
		require.NoError(t, err)

		strategyIDs := []string{"zero-drift", "exit-mismatch"}
		at := nowMicro()
		report := m.RunOnce(context.Background(), at)
		require.Equal(t, RunBreach, report.Outcome)
		require.Len(t, report.Results, 2)

		var count int64
		require.NoError(t, db.Model(&tradingstack.SimulatorFidelity{}).
			Where("strategy_id IN ?", strategyIDs).Count(&count).Error)
		assert.Equal(t, int64(2), count)

		// Re-running the monitor for the same evaluation time re-persists the
		// same periods: still one row per strategy.
		report = m.RunOnce(context.Background(), at)
		require.Equal(t, RunBreach, report.Outcome)
		require.NoError(t, db.Model(&tradingstack.SimulatorFidelity{}).
			Where("strategy_id IN ?", strategyIDs).Count(&count).Error)
		assert.Equal(t, int64(2), count, "a re-run of the same period upserts, not duplicates")
	})

	t.Run("distinct periods accumulate history", func(t *testing.T) {
		later := Period{Start: period.Start.AddDate(0, 0, 7), End: period.End.AddDate(0, 0, 7)}

		require.NoError(t, Persist(db, []Result{
			{StrategyID: "s-history", Period: period, DriftScore: 0.10, WithinTolerance: true},
		}, nowMicro()))
		require.NoError(t, Persist(db, []Result{
			{StrategyID: "s-history", Period: later, DriftScore: 0.30, WithinTolerance: false},
		}, nowMicro()))

		var rows []tradingstack.SimulatorFidelity
		require.NoError(t, db.Where("strategy_id = ?", "s-history").Order("period_start asc").Find(&rows).Error)
		require.Len(t, rows, 2, "one row per distinct period")
		require.NotNil(t, rows[0].DriftScore)
		assert.Equal(t, 0.10, *rows[0].DriftScore)
		require.NotNil(t, rows[1].DriftScore)
		assert.Equal(t, 0.30, *rows[1].DriftScore)
	})
}
